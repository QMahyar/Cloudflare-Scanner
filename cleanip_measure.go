package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// dialReachable TCP-dials endpoint, retrying ONLY on timeout (up to maxAttempts).
// A single dropped SYN under the high-concurrency burst would otherwise discard
// an IP whose real RTT is well under the deadline — the cause of "tight timeout
// finds nothing, loose timeout finds the same IPs fast". A refused/unreachable
// port won't change on retry, so those return immediately and don't pay the cost.
//
// The dial is context-aware: cancelling ctx aborts an in-flight dial promptly so
// a stopped scan doesn't keep workers blocked in DialTimeout for up to maxAttempts
// × timeout after the caller has already snapshotted and returned.
func dialReachable(ctx context.Context, endpoint string, timeout time.Duration, maxAttempts int) (time.Duration, bool) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var d net.Dialer
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Honor cancellation between attempts (and before the first), not only
		// mid-dial, so a stop between retries exits immediately.
		if err := ctx.Err(); err != nil {
			return 0, false
		}
		start := time.Now()
		dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
		conn, err := d.DialContext(dialCtx, "tcp", endpoint)
		dialCancel()
		if err == nil {
			conn.Close()
			return time.Since(start), true
		}
		// Cancellation surfaces as a ctx error, not a timeout — bail without retry.
		if ctx.Err() != nil {
			return 0, false
		}
		var ne net.Error
		if !errors.As(err, &ne) || !ne.Timeout() {
			return 0, false
		}
	}
	return 0, false
}

func runCleanPhase1TCP(ctx context.Context, endpoints []string, timeout time.Duration, cancel chan struct{}, job *CleanIPJob, concurrency int, stopAfter int) []CleanIPResult {
	var mu sync.Mutex
	if concurrency <= 0 {
		concurrency = 500
	}
	if len(endpoints) < concurrency {
		concurrency = len(endpoints)
	}
	results := make([]CleanIPResult, 0, len(endpoints))

	// localStop lets phase 1 finish early once stopAfter responders are found,
	// WITHOUT closing the job's global Cancel (which would also abort phase 2).
	localStop := make(chan struct{})
	var stopOnce sync.Once
	stopNow := func() { stopOnce.Do(func() { close(localStop) }) }

	// phaseCtx is cancelled on either job cancel or local early-stop, so one
	// context drives all workers without per-goroutine cancel watchers.
	phaseCtx, phaseCancel := context.WithCancel(ctx)
	defer phaseCancel()
	go func() {
		select {
		case <-localStop:
			phaseCancel()
		case <-cancel:
			phaseCancel()
		case <-phaseCtx.Done():
		}
	}()

	// Fixed worker pool: a feeder pushes endpoints through a channel and
	// `concurrency` workers drain it, capping live goroutines at `concurrency`
	// instead of one parked goroutine per endpoint (a 100k-IP scan otherwise
	// parks 100k goroutines on the semaphore at once). wg.Wait() below always
	// drains the in-flight set before snapshotting, so no appender touches
	// `results` after we sort/return it.
	work := make(chan string)
	go func() {
		defer close(work)
		for _, ep := range endpoints {
			select {
			case <-phaseCtx.Done():
				return
			case work <- ep:
			}
		}
	}()

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for endpoint := range work {
				latency, ok := dialReachable(phaseCtx, endpoint, timeout, 2)
				if !ok {
					// Count a cancelled probe as not-attempted: it was never
					// actually tested, so reporting it as "scanned" would
					// understate the hit rate after a stop. Failed-but-tested
					// dials still count.
					if phaseCtx.Err() != nil {
						return
					}
					if job != nil {
						job.mu.Lock()
						job.Phase1Progress++
						job.mu.Unlock()
					}
					continue
				}

				// Drop a result that landed after the job was cancelled or the
				// local stop-after target was reached — the scan is winding down
				// and the published snapshot must not drift under late appenders.
				if phaseCtx.Err() != nil {
					return
				}

				// Colo/loc are enriched separately for a bounded set of the
				// fastest responders — keeping the trace round-trip out of this
				// dial loop so dense ranges aren't throttled to ~2s per responder.
				result := CleanIPResult{
					Endpoint: endpoint,
					Latency:  latency,
					Success:  true,
					Attempts: 1,
					Passes:   1,
					Best:     latency,
				}

				mu.Lock()
				results = append(results, result)
				n := len(results)
				mu.Unlock()

				if job != nil {
					job.mu.Lock()
					job.Phase1Results = append(job.Phase1Results, result)
					job.Phase1Progress++
					job.mu.Unlock()
				}

				if stopAfter > 0 && n >= stopAfter {
					stopNow()
				}
			}
		}()
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

	return results
}

func probeCloudflareTrace(ctx context.Context, endpoint, sni string, timeout time.Duration) (colo, loc string) {
	_, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", ""
	}
	// /cdn-cgi/trace is answered by the Cloudflare edge that terminates the
	// connection, and exists on every CF-proxied hostname. The well-known
	// speed.cloudflare.com SNI is exactly what DPI on filtered ISPs resets on
	// sight, which is why a direct edge probe returns nothing there and the colo
	// column stays empty. Reusing the user's own config SNI — which their working
	// config proves is unblocked — lets the probe complete and report that IP's
	// real colo. Falls back to speed.cloudflare.com when no domain SNI is given
	// (e.g. one-phase scans with no config).
	host := strings.TrimSpace(sni)
	if host == "" || net.ParseIP(host) != nil {
		host = "speed.cloudflare.com"
	}
	scheme := "http"
	if port == "443" || port == "8443" || port == "2053" || port == "2083" || port == "2087" || port == "2096" {
		scheme = "https"
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, endpoint)
		},
		TLSClientConfig:   &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12},
		DisableKeepAlives: true,
	}
	// One-shot transport per probe — release its connection promptly instead of
	// leaving idle conns to linger until GC (up to ~150 probes per scan).
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+host+"/cdn-cgi/trace", nil)
	if err != nil {
		return "", ""
	}
	req.Host = host
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "colo=") {
			colo = strings.TrimSpace(strings.TrimPrefix(line, "colo="))
		}
		if strings.HasPrefix(line, "loc=") {
			loc = strings.TrimSpace(strings.TrimPrefix(line, "loc="))
		}
	}
	return colo, loc
}

// ipOnly returns the host portion of an "ip:port" / "[ipv6]:port" endpoint.
func ipOnly(endpoint string) string {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return ""
	}
	return host
}

// buildColoMap probes /cdn-cgi/trace for up to maxIPs distinct responders
// (deduped by IP, fastest first since results arrive latency-sorted) and returns
// an IP -> {colo, loc} map. It only reads results, never mutates them, so it is
// safe to run lock-free against a published slice. Bounded + concurrent so it
// stays off the Phase-1 dial hot path regardless of how many IPs responded.
func buildColoMap(ctx context.Context, results []CleanIPResult, sni string, maxIPs, concurrency int) map[string][2]string {
	if maxIPs <= 0 {
		maxIPs = 150
	}
	if concurrency <= 0 {
		concurrency = 48
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

	coloMap := make(map[string][2]string, len(targets))
	if len(targets) == 0 {
		return coloMap
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, tgt := range targets {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			colo, loc := probeCloudflareTrace(ctx, t.endpoint, sni, 3*time.Second)
			if colo == "" && loc == "" {
				return
			}
			mu.Lock()
			coloMap[t.ip] = [2]string{colo, loc}
			mu.Unlock()
		}(tgt)
	}
	wg.Wait()
	return coloMap
}

// applyColo writes the colo/loc from a coloMap onto every matching result.
// Callers must hold job.mu when results is the published job slice.
func applyColo(results []CleanIPResult, coloMap map[string][2]string) {
	if len(coloMap) == 0 {
		return
	}
	for i := range results {
		if cl, ok := coloMap[ipOnly(results[i].Endpoint)]; ok {
			results[i].Colo = cl[0]
			results[i].Loc = cl[1]
		}
	}
}

// qualitySample is the per-IP quality measurement produced by measureQuality.
type qualitySample struct {
	best   time.Duration
	median time.Duration
	jitter time.Duration
	loss   float64 // 0–100
}

// measureQuality probes up to maxIPs distinct responders (fastest first, since
// results arrive latency-sorted) with k INDEPENDENT single-shot TCP dials each,
// yielding per-IP loss / jitter / median. Phase 1 does one dial per endpoint
// (retried only on timeout) to maximize the discovery hit-rate; that can't
// measure loss, because a dropped SYN is retried away. This pass deliberately
// does discrete dials so drops count, and computes the jitter Phase 1 leaves at
// zero — turning "it responded once" into a real quality signal for the handful
// of IPs the user will actually pick. Bounded + concurrent and off the dial hot
// path, exactly like buildColoMap, so dense ranges aren't throttled.
func measureQuality(ctx context.Context, results []CleanIPResult, k, maxIPs, concurrency int, timeout time.Duration) map[string]qualitySample {
	if k <= 0 {
		k = 4
	}
	if maxIPs <= 0 {
		maxIPs = 64
	}
	if concurrency <= 0 {
		concurrency = 48
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

	out := make(map[string]qualitySample, len(targets))
	if len(targets) == 0 {
		return out
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, tgt := range targets {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			var lats []time.Duration
			for i := 0; i < k; i++ {
				if ctx.Err() != nil {
					return
				}
				// maxAttempts=1 → one independent dial, no internal retry, so a
				// dropped/refused connection is recorded as loss rather than masked.
				if rtt, ok := dialReachable(ctx, t.endpoint, timeout, 1); ok {
					lats = append(lats, rtt)
				}
			}
			if ctx.Err() != nil {
				return
			}
			sample := qualitySample{loss: lossPercent(len(lats), k)}
			if len(lats) > 0 {
				sample.best = bestDuration(lats)
				sample.median = medianDuration(lats)
				sample.jitter = jitterDuration(lats)
			}
			mu.Lock()
			out[t.ip] = sample
			mu.Unlock()
		}(tgt)
	}
	wg.Wait()
	return out
}

// applyQuality writes loss/jitter/best from a quality map onto matching results
// and (re)computes each result's Score from its own displayed latency plus the
// measured jitter and loss. Phase-1 results score off their TCP latency; Phase-2
// results keep their tunnel latency and inherit the edge IP's measured loss/jitter,
// so the score reflects both the proxy path and the underlying IP's stability.
// Callers must hold job.mu when results is the published job slice.
func applyQuality(results []CleanIPResult, qmap map[string]qualitySample) {
	if len(qmap) == 0 {
		return
	}
	for i := range results {
		s, ok := qmap[ipOnly(results[i].Endpoint)]
		if !ok {
			continue
		}
		results[i].Loss = s.loss
		if results[i].Jitter == 0 {
			results[i].Jitter = s.jitter
		}
		if s.best > 0 && (results[i].Best == 0 || s.best < results[i].Best) {
			results[i].Best = s.best
		}
		results[i].Score = qualityScore(results[i].Latency, results[i].Jitter, results[i].Loss)
	}
}
