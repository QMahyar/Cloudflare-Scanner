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

type ScanResult struct {
	Endpoint string
	Latency  time.Duration
	Success  bool
	Error    string
	Attempts int
	Passes   int
	Best     time.Duration
	Jitter   time.Duration
	Loss     float64 // packet-loss % across attempts (0–100)
	Score    int     // 0–100 quality rank (latency+jitter+loss)
}

type Scanner struct {
	Config      *WarpConfig
	Noise       NoiseConfig
	XrayPath    string
	Concurrency int
	Timeout     time.Duration
	TCPOnly     bool
	prober      *warpProber
}

func NewScanner(cfg *WarpConfig, noise NoiseConfig, xrayPath string) *Scanner {
	s := &Scanner{
		Config:      cfg,
		Noise:       noise,
		XrayPath:    xrayPath,
		Concurrency: DefaultConcurrency(cfg, noise),
		Timeout:     6 * time.Second,
	}
	// Build the handshake prober once when the native UDP path is in play
	// (a config is present and no obfuscation forces the xray fallback). The
	// expensive static-static DH and key parsing then happen a single time
	// instead of once per endpoint.
	if cfg != nil && noise.Type == "" {
		if p, err := newWarpProber(cfg); err == nil {
			s.prober = p
		}
	}
	return s
}

// DefaultConcurrency picks a starting worker count for the scan path that will
// actually run. Native handshake / TCP probes are lightweight; noise scans use
// xray batches, so keep their process count modest.
func DefaultConcurrency(cfg *WarpConfig, noise NoiseConfig) int {
	if noise.Type != "" {
		return 12
	}
	return 256
}

func (s *Scanner) testEndpointAttempts(ctx context.Context, endpoint string, attempts int) ScanResult {
	if attempts <= 0 {
		attempts = 1
	}
	var latencies []time.Duration
	var lastErr string
loop:
	for i := 0; i < attempts; i++ {
		result := s.testEndpointOnce(ctx, endpoint)
		if result.Success {
			latencies = append(latencies, result.Latency)
		} else {
			lastErr = result.Error
		}
		select {
		case <-ctx.Done():
			lastErr = "cancelled"
			break loop
		default:
		}
	}
	passes := len(latencies)
	if passes == 0 {
		return ScanResult{Endpoint: endpoint, Error: lastErr, Attempts: attempts, Passes: passes}
	}
	median := medianDuration(latencies)
	jitter := jitterDuration(latencies)
	loss := lossPercent(passes, attempts)
	return ScanResult{
		Endpoint: endpoint,
		Success:  true,
		Latency:  median,
		Attempts: attempts,
		Passes:   passes,
		Best:     bestDuration(latencies),
		Jitter:   jitter,
		Loss:     loss,
		Score:    qualityScore(median, jitter, loss),
	}
}

func (s *Scanner) testEndpointOnce(ctx context.Context, endpoint string) ScanResult {
	select {
	case <-ctx.Done():
		return ScanResult{Endpoint: endpoint, Error: "cancelled"}
	default:
	}

	if s.TCPOnly {
		start := time.Now()
		dialCtx, dialCancel := context.WithTimeout(ctx, s.Timeout)
		defer dialCancel()
		var d net.Dialer
		conn, err := d.DialContext(dialCtx, "tcp", endpoint)
		if err != nil {
			return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("tcp dial: %v", err)}
		}
		conn.Close()
		return ScanResult{Endpoint: endpoint, Success: true, Latency: time.Since(start)}
	}

	// Native WireGuard handshake validates the endpoint over UDP with no xray
	// process. Noise / AmneziaWG scans are routed to scanBatchNoise by runScan.
	prober := s.prober
	if prober == nil {
		// Defensive fallback: build a one-shot prober if NewScanner could
		// not (should not happen for a valid config).
		p, err := newWarpProber(s.Config)
		if err != nil {
			return ScanResult{Endpoint: endpoint, Error: err.Error()}
		}
		prober = p
	}
	rtt, err := prober.Probe(ctx, endpoint, s.Timeout)
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: err.Error()}
	}
	return ScanResult{Endpoint: endpoint, Success: true, Latency: rtt}
}

// probe204 runs the SOCKS5 + GET /generate_204 check against an already-up local
// SOCKS port and returns the result. The caller owns the xray process.
func (s *Scanner) probe204(ctx context.Context, endpoint string, socksPort int) ScanResult {
	start := time.Now()

	dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
	defer dialCancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", fmt.Sprintf("127.0.0.1:%d", socksPort))
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("socks connect: %v", err)}
	}
	defer conn.Close()

	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	// Set a single deadline covering both the SOCKS5 handshake and HTTP round-trip.
	conn.SetDeadline(time.Now().Add(timeout))

	if err := socks5Handshake(conn, "www.gstatic.com", 80); err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("socks5: %v", err)}
	}

	req := "GET /generate_204 HTTP/1.1\r\nHost: www.gstatic.com\r\nConnection: close\r\nUser-Agent: Mozilla/5.0\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("http write: %v", err)}
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("http read: %v", err)}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 204 {
		return ScanResult{Endpoint: endpoint, Success: true, Latency: time.Since(start)}
	}

	return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// scanBatchNoise validates a batch of WARP endpoints through a SINGLE xray process
// (the noise/AmneziaWG fallback): one SOCKS inbound + WireGuard outbound + routing
// rule per endpoint (see GenerateConfigBatch). It trades one process spawn + port
// wait per endpoint for one per batch, while keeping each endpoint's 204 check
// independent and concurrent. Results are aligned to the endpoints slice.
func (s *Scanner) scanBatchNoise(ctx context.Context, endpoints []string, basePort int) []ScanResult {
	xm := &XrayManager{XrayPath: s.XrayPath, Config: s.Config, Noise: s.Noise}
	configPath, _, err := xm.GenerateConfigBatch(endpoints, basePort)
	if err != nil {
		results := make([]ScanResult, len(endpoints))
		for i, ep := range endpoints {
			results[i] = ScanResult{Endpoint: ep, Error: fmt.Sprintf("config: %v", err)}
		}
		return results
	}
	defer os.RemoveAll(filepath.Dir(configPath))

	// ponytail: shared batch lifecycle via BatchProbe - same as cleanip.go
	pResults := BatchProbe(ctx, s.XrayPath, configPath, endpoints, basePort, s.Timeout,
		func(ctx context.Context, endpoint string, socksPort int) probeResult {
			r := s.probe204(ctx, endpoint, socksPort)
			return probeResult{Endpoint: r.Endpoint, Latency: r.Latency, Success: r.Success, Error: r.Error}
		},
	)

	results := make([]ScanResult, len(pResults))
	for i, pr := range pResults {
		results[i] = ScanResult{Endpoint: pr.Endpoint, Latency: pr.Latency, Success: pr.Success, Error: pr.Error}
	}
	return results
}

// summarizeWarpFailure collapses a raw WARP-probe error into a short, actionable
// category for the UI's failure-reason breakdown. An all-failed scan is otherwise
// a silent dead end where "ISP blocks WARP UDP" looks identical to "wrong config"
// or "endpoints simply unreachable". The native handshake path's dominant failure
// — no handshake response — almost always means the UDP datagrams aren't getting
// through (ISP/DPI dropping WireGuard), which the noise/xray fallback can sometimes
// bypass; we say so explicitly.
func summarizeWarpFailure(err string) string {
	e := strings.ToLower(err)
	switch {
	case strings.Contains(e, "no handshake response"):
		return "No WireGuard handshake — UDP likely blocked/throttled by your ISP (try enabling UDP Noise)"
	case strings.Contains(e, "startup timeout"):
		return "xray didn't come up in time (slow start or crash)"
	case strings.Contains(e, "start xray"), strings.Contains(e, "config:"):
		return "xray failed to launch (check the xray binary / config)"
	case strings.Contains(e, "socks connect"), strings.Contains(e, "socks5"):
		return "couldn't reach xray's local SOCKS port"
	case strings.Contains(e, "i/o timeout"), strings.Contains(e, "deadline exceeded"):
		return "probe timed out — endpoint unreachable or UDP filtered"
	case strings.Contains(e, "forcibly closed"), strings.Contains(e, "connection reset"), strings.Contains(e, "refused"):
		return "connection reset/refused (DPI filtering or dead endpoint)"
	case strings.Contains(e, "invalid private key"), strings.Contains(e, "peer public key"):
		return "config credentials rejected (bad/expired .conf keys)"
	case strings.Contains(e, "cancelled"):
		return "cancelled"
	case strings.HasPrefix(e, "tcp dial"):
		return "TCP port closed (reachability-only check, not a working WARP endpoint)"
	case e == "":
		return "unknown"
	default:
		return err
	}
}

func socks5Handshake(conn net.Conn, host string, port int) error {
	// Auth negotiation
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("auth neg: %w", err)
	}

	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("auth resp: %w", err)
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		return fmt.Errorf("auth failed: %x %x", buf[0], buf[1])
	}

	// Connect request
	addr := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	addr = append(addr, []byte(host)...)
	addr = append(addr, byte(port>>8), byte(port))

	if _, err := conn.Write(addr); err != nil {
		return fmt.Errorf("connect req: %w", err)
	}

	// Read response header
	resp := make([]byte, 4)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("connect resp: %w", err)
	}
	if resp[1] != 0x00 {
		return fmt.Errorf("connect failed: code %d", resp[1])
	}

	// Skip remaining address
	switch resp[3] {
	case 0x01: // IPv4
		if _, err := io.ReadFull(conn, make([]byte, 4)); err != nil {
			return fmt.Errorf("skip addr: %w", err)
		}
	case 0x03: // domain
		n := make([]byte, 1)
		if _, err := io.ReadFull(conn, n); err != nil {
			return fmt.Errorf("skip domain len: %w", err)
		}
		if _, err := io.ReadFull(conn, make([]byte, n[0])); err != nil {
			return fmt.Errorf("skip domain: %w", err)
		}
	case 0x04: // IPv6
		if _, err := io.ReadFull(conn, make([]byte, 16)); err != nil {
			return fmt.Errorf("skip addr: %w", err)
		}
	}
	if _, err := io.ReadFull(conn, make([]byte, 2)); err != nil {
		return fmt.Errorf("skip port: %w", err)
	}

	return nil
}
