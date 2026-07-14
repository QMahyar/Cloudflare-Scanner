package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// summarizeFailure collapses a raw Phase-2 error into a short, actionable
// category for the UI's failure-reason breakdown. It turns a wall of identical
// low-level strings into "12× xray startup timeout" so the user can tell a
// broken xray from a routing/Host problem from a too-tight timeout.
func summarizeFailure(err string) string {
	e := strings.ToLower(err)
	switch {
	case strings.Contains(e, "startup timeout"):
		return "xray didn't come up in time (slow start or crash)"
	case strings.Contains(e, "start xray"), strings.Contains(e, "xray config"):
		return "xray failed to launch (check the xray binary / config)"
	case strings.Contains(e, "no uuid"), strings.Contains(e, "empty uuid"), strings.Contains(e, "empty address"), strings.Contains(e, "invalid port"):
		return "incomplete config (UUID/address/port missing)"
	case strings.Contains(e, "socks connect"):
		return "couldn't reach xray's local SOCKS port"
	case strings.Contains(e, "socks5"):
		return "tunnel handshake failed (proxy refused the connection)"
	case strings.Contains(e, "forcibly closed"), strings.Contains(e, "connection reset"), strings.Contains(e, "reset by peer"), strings.Contains(e, "unexpected eof"):
		return "connection reset mid-handshake (likely ISP/DPI filtering or a dead origin)"
	case strings.Contains(e, "connection refused"):
		return "origin refused the connection (dead endpoint or wrong port)"
	case strings.Contains(e, "http write"), strings.Contains(e, "http read"):
		return "no usable response through the tunnel (timeout / reset)"
	case strings.HasPrefix(e, "http "):
		return err + " (Cloudflare reached but didn't route — check SNI/Host)"
	case strings.Contains(e, "cancelled"):
		return "cancelled"
	default:
		return err
	}
}

// extractXrayErrorFrom mines already-read xray log bytes for the concrete reason
// a Phase-2 tunnel failed. The error we observe locally is always a generic
// "http read timeout" once the SOCKS CONNECT is (optimistically) accepted; the
// real cause — a TLS reset, a refused dial, a routing rejection — only lives in
// xray's log. Folding it into the result turns "no usable response" into
// something actionable (e.g. distinguishing ISP/DPI filtering from a dead origin).
// Returns "" when nothing useful is found.
//
// One xray process serves a whole batch (BuildXrayJSONBatch), so its log
// interleaves every endpoint's errors and the caller reads it ONCE for the whole
// batch (not per failed endpoint). Picking the bare last error line would
// mis-attribute one endpoint's failure to another — two endpoints reporting an
// error that names a third IP. When ipHint is set we therefore only accept a log
// line that mentions THIS endpoint's IP, and return "" (letting the honest base
// error stand) when none does.
func extractXrayErrorFrom(data []byte, ipHint string) string {
	if len(data) == 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		low := strings.ToLower(l)
		if !strings.Contains(low, "[error]") && !strings.Contains(low, "[warning]") {
			continue
		}
		if !strings.Contains(low, "failed") && !strings.Contains(low, "forcibly closed") &&
			!strings.Contains(low, "refused") && !strings.Contains(low, "reset") &&
			!strings.Contains(low, "eof") && !strings.Contains(low, "rejected") {
			continue
		}
		// Shared batch log: only trust a line that names this endpoint's IP, or we'd
		// attribute a neighbor's failure to it. xray logs the dial target as
		// "ip:port" (v4) or "[ip]:port" (v6), so require the IP followed by its
		// delimiter — a bare Contains would let "1.2.3.4" match "1.2.3.40" (and
		// "2606:4700::1" match "2606:4700::10").
		if ipHint != "" && !strings.Contains(l, ipHint+":") && !strings.Contains(l, ipHint+"]") {
			continue
		}
		// Keep only the deepest cause (xray chains them with "> ").
		if idx := strings.LastIndex(l, "> "); idx >= 0 {
			l = l[idx+2:]
		}
		l = strings.TrimSpace(l)
		if len(l) > 160 {
			l = l[:160] + "…"
		}
		return l
	}
	return ""
}

// socks204Probe runs the SOCKS5 + GET /generate_204 check against an already-up
// local SOCKS port and returns the result. It is the shared core of the pooled
// validateBatchWithXray path, keeping the success criterion (exact 204) in one
// place. The caller owns the xray process; it also enriches http write/read
// failures with the xray-log cause once per batch (see validateBatchWithXray),
// so this probe just returns the bare transport error.
func socks204Probe(ctx context.Context, endpoint string, socksPort int, timeout time.Duration) CleanIPResult {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	start := time.Now()

	var d net.Dialer
	dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
	defer dialCancel()
	conn, err := d.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("socks connect: %v", err)}
	}
	defer conn.Close()

	// Single deadline covering both SOCKS5 handshake and HTTP round-trip.
	conn.SetDeadline(time.Now().Add(timeout))

	if err := socks5Handshake(conn, "www.gstatic.com", 80); err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("socks5: %v", err)}
	}

	req := "GET /generate_204 HTTP/1.1\r\nHost: www.gstatic.com\r\nConnection: close\r\nUser-Agent: Mozilla/5.0\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("http write: %v", err)}
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("http read: %v", err)}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 204 {
		return CleanIPResult{Endpoint: endpoint, Success: true, Latency: time.Since(start)}
	}
	return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// validateBatchWithXray validates a whole batch of endpoints through a SINGLE
// xray process: one SOCKS inbound + outbound + routing rule per endpoint (see
// BuildXrayJSONBatch). This trades N process spawns (each with its own exec +
// up-to-4s port-wait) for one, the dominant Phase-2 cost. Each endpoint is still
// probed independently (its own port, its own 204 check), and the probes run
// concurrently against the shared process. Results are aligned to the input
// endpoints slice. Honors ctx cancellation between and during probes.
func validateBatchWithXray(ctx context.Context, cfg *ProxyConfig, endpoints []string, xrayPath string, basePort int, timeout time.Duration) []CleanIPResult {
	configPath, _, err := cfg.BuildXrayJSONBatch(endpoints, basePort)
	if err != nil {
		results := make([]CleanIPResult, len(endpoints))
		for i, ep := range endpoints {
			results[i] = CleanIPResult{Endpoint: ep, Error: fmt.Sprintf("xray config: %v", err)}
		}
		return results
	}
	defer os.RemoveAll(filepath.Dir(configPath))

	logPath := filepath.Join(filepath.Dir(configPath), "xray.log")

	// ponytail: shared batch lifecycle via BatchProbe - same as scanner.go
	pResults := BatchProbe(ctx, xrayPath, configPath, endpoints, basePort, timeout,
		func(ctx context.Context, endpoint string, socksPort int) probeResult {
			r := socks204Probe(ctx, endpoint, socksPort, timeout)
			return probeResult{Endpoint: r.Endpoint, Latency: r.Latency, Success: r.Success, Error: r.Error}
		},
	)

	results := make([]CleanIPResult, len(pResults))
	for i, pr := range pResults {
		results[i] = CleanIPResult{Endpoint: pr.Endpoint, Latency: pr.Latency, Success: pr.Success, Error: pr.Error, Attempts: 1}
		if pr.Success {
			results[i].Passes = 1
			results[i].Best = pr.Latency
		}
	}

	// Enrich tunnel failures with the deepest cause from xray log (clean-IP specific).
	if logData, rerr := os.ReadFile(logPath); rerr == nil && len(logData) > 0 {
		for i := range results {
			if results[i].Success {
				continue
			}
			e := results[i].Error
			if !strings.HasPrefix(e, "http write:") && !strings.HasPrefix(e, "http read:") {
				continue
			}
			if cause := extractXrayErrorFrom(logData, ipOnly(results[i].Endpoint)); cause != "" {
				results[i].Error = e + " | xray: " + cause
			}
		}
	}
	return results
}
