package main

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// h3HTTPSPorts are the Cloudflare ports that terminate TLS (and therefore can
// speak QUIC/HTTP-3). Probing h3 on a plain-HTTP port just burns a full timeout,
// so the probe is gated to these.
var h3HTTPSPorts = map[string]bool{
	"443": true, "8443": true, "2053": true, "2083": true, "2087": true, "2096": true,
}

// h3Probe reports whether an edge IP answers HTTP/3 (QUIC). It fetches
// /cdn-cgi/trace over h3 while dialing the *target IP* directly on UDP and
// presenting a CF-proxied SNI — the user's config host when supplied, else
// speed.cloudflare.com (DPI resets the well-known SNI on filtered ISPs, the same
// reason buildColoMap reuses the config SNI). A true result is a clean h3
// round-trip (2xx/3xx); false means "no QUIC reachable here", not a global verdict.
//
// Cost is a full QUIC handshake, so callers must keep this on the bounded
// enrichment path (top-N only), never the dial hot loop.
func h3Probe(ctx context.Context, endpoint, sni string, timeout time.Duration) bool {
	_, port, err := net.SplitHostPort(endpoint)
	if err != nil || !h3HTTPSPorts[port] {
		return false
	}
	host := strings.TrimSpace(sni)
	if host == "" || net.ParseIP(host) != nil {
		host = "speed.cloudflare.com"
	}
	return h3RoundTrip(ctx, endpoint, host, "/cdn-cgi/trace", timeout, false)
}

// h3RoundTrip performs the actual HTTP/3 GET against a target UDP endpoint while
// presenting `host` as the SNI/authority, returning true on a 2xx/3xx response.
// It's split out from h3Probe (which adds the port gate + SNI fallback) so the
// QUIC wiring — the custom Dial, the UDP-socket lifecycle, the round-trip — can
// be tested against a loopback h3 server without needing real UDP/443 egress.
// insecure skips cert verification (loopback tests with self-signed certs only).
func h3RoundTrip(ctx context.Context, endpoint, host, path string, timeout time.Duration, insecure bool) bool {
	udpAddr, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var udpConn *net.UDPConn
	tr := &http3.Transport{
		TLSClientConfig: &tls.Config{
			ServerName:         host,
			MinVersion:         tls.VersionTLS13,
			NextProtos:         []string{"h3"},
			InsecureSkipVerify: insecure,
		},
		QUICConfig: &quic.Config{
			HandshakeIdleTimeout: timeout,
			MaxIdleTimeout:       timeout,
		},
		// Force the QUIC dial at the scanned edge IP, ignoring the SNI host's own
		// DNS. We own the UDP socket; quic-go won't close a PacketConn we pass in,
		// so we close it ourselves after the round-trip (see the defers below).
		Dial: func(ctx context.Context, _ string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
			uc, err := net.ListenUDP("udp", nil)
			if err != nil {
				return nil, err
			}
			udpConn = uc
			return quic.DialEarly(ctx, uc, udpAddr, tlsCfg, cfg)
		},
	}
	// LIFO: tr.Close() (tears down the QUIC connection) runs first, then we
	// release the UDP socket it was using.
	defer func() {
		if udpConn != nil {
			udpConn.Close()
		}
	}()
	defer tr.Close()

	client := &http.Client{Transport: tr, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+path, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// buildH3Map probes h3 reachability for up to maxIPs distinct responders
// (fastest first, deduped by IP) and returns an IP -> reachable map. Bounded +
// concurrent and read-only over results, mirroring buildColoMap / measureQuality.
func buildH3Map(ctx context.Context, results []CleanIPResult, sni string, maxIPs, concurrency int, timeout time.Duration) map[string]bool {
	if maxIPs <= 0 {
		maxIPs = 48
	}
	if concurrency <= 0 {
		concurrency = 24
	}

	type target struct{ ip, endpoint string }
	var targets []target
	seen := make(map[string]bool)
	for _, r := range results {
		if !r.Success {
			continue
		}
		ip := ipOnly(r.Endpoint)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		targets = append(targets, target{ip: ip, endpoint: r.Endpoint})
		if len(targets) >= maxIPs {
			break
		}
	}

	out := make(map[string]bool, len(targets))
	if len(targets) == 0 {
		return out
	}

	var mu sync.Mutex

	// Probe-first bail-out: many networks this tool targets (the WARP-blocking
	// ISPs) drop UDP/443 entirely, so every h3 probe would time out. Probe a few
	// up front; if none answer, QUIC is unreachable here — skip the rest instead
	// of burning maxIPs×timeout to produce an all-false column.
	head := 3
	if head > len(targets) {
		head = len(targets)
	}
	var hwg sync.WaitGroup
	anyOK := false
	for _, t := range targets[:head] {
		hwg.Add(1)
		go func(t target) {
			defer hwg.Done()
			if h3Probe(ctx, t.endpoint, sni, timeout) {
				mu.Lock()
				out[t.ip] = true
				anyOK = true
				mu.Unlock()
			}
		}(t)
	}
	hwg.Wait()
	if !anyOK || ctx.Err() != nil {
		return out
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, tgt := range targets[head:] {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			if h3Probe(ctx, t.endpoint, sni, timeout) {
				mu.Lock()
				out[t.ip] = true
				mu.Unlock()
			}
		}(tgt)
	}
	wg.Wait()
	return out
}

// applyH3 marks every result whose IP answered h3. Callers must hold job.mu when
// results is the published job slice.
func applyH3(results []CleanIPResult, h3Map map[string]bool) {
	if len(h3Map) == 0 {
		return
	}
	for i := range results {
		if h3Map[ipOnly(results[i].Endpoint)] {
			results[i].H3 = true
		}
	}
}
