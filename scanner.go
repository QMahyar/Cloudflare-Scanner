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
	"sync/atomic"
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
}

type Scanner struct {
	Config      *WarpConfig
	Noise       NoiseConfig
	XrayPath    string
	Concurrency int
	Timeout     time.Duration
	TCPOnly     bool
	portCounter atomic.Int32
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
// actually run. The native WireGuard handshake is just a UDP socket plus some
// cheap crypto, so it scales to hundreds of concurrent probes; the xray
// noise-fallback path spawns a process per probe and must stay modest.
func DefaultConcurrency(cfg *WarpConfig, noise NoiseConfig) int {
	if noise.Type != "" {
		return 12 // xray process per probe — keep it small
	}
	return 256 // native handshake or plain TCP dial — lightweight
}

func (s *Scanner) nextPort() int {
	return int(s.portCounter.Add(1))
}

func (s *Scanner) testEndpointAttempts(ctx context.Context, endpoint string, attempts int) ScanResult {
	if attempts <= 0 {
		attempts = 1
	}
	var latencies []time.Duration
	var lastErr string
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
			i = attempts
		default:
		}
	}
	passes := len(latencies)
	if passes == 0 {
		return ScanResult{Endpoint: endpoint, Error: lastErr, Attempts: attempts, Passes: passes}
	}
	return ScanResult{
		Endpoint: endpoint,
		Success:  true,
		Latency:  medianDuration(latencies),
		Attempts: attempts,
		Passes:   passes,
		Best:     bestDuration(latencies),
		Jitter:   jitterDuration(latencies),
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

	// Fast path: a native WireGuard handshake validates the endpoint over UDP —
	// the protocol WARP actually speaks — using the uploaded config's registered
	// credentials, with no xray process and no SOCKS hop. Only when noise /
	// AmneziaWG obfuscation is requested do we fall back to xray, which is what
	// applies that obfuscation on the wire.
	if s.Noise.Type == "" {
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
		rtt, err := prober.Probe(endpoint, s.Timeout)
		if err != nil {
			return ScanResult{Endpoint: endpoint, Error: err.Error()}
		}
		return ScanResult{Endpoint: endpoint, Success: true, Latency: rtt}
	}

	socksPort := s.nextPort() + 10799

	xm := &XrayManager{
		XrayPath: s.XrayPath,
		Config:   s.Config,
		Noise:    s.Noise,
	}

	configPath, err := xm.GenerateConfig(endpoint, socksPort)
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("config: %v", err)}
	}
	defer os.RemoveAll(filepath.Dir(configPath))

	cmd, err := xm.StartXray(configPath)
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("start: %v", err)}
	}
	defer xm.StopXray(cmd)

	if !xm.WaitForPortCtx(ctx, socksPort, 4*time.Second) {
		return ScanResult{Endpoint: endpoint, Error: "xray startup timeout"}
	}

	start := time.Now()

	dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
	defer dialCancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", fmt.Sprintf("127.0.0.1:%d", socksPort))
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("socks connect: %v", err)}
	}
	defer conn.Close()

	// Set a single deadline covering both the SOCKS5 handshake and HTTP round-trip.
	conn.SetDeadline(time.Now().Add(6 * time.Second))

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
	case 0x01:
		io.ReadFull(conn, make([]byte, 4))
	case 0x03:
		n := make([]byte, 1)
		io.ReadFull(conn, n)
		io.ReadFull(conn, make([]byte, n[0]))
	case 0x04:
		io.ReadFull(conn, make([]byte, 16))
	}
	io.ReadFull(conn, make([]byte, 2))

	return nil
}
