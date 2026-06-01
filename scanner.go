package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ScanResult struct {
	Endpoint string
	Latency  time.Duration
	Success  bool
	Error    string
}

type Scanner struct {
	Config      *WarpConfig
	Noise       NoiseConfig
	XrayPath    string
	WorkDir     string
	Concurrency int
	Timeout     time.Duration
	portCounter atomic.Int32
}

func NewScanner(cfg *WarpConfig, noise NoiseConfig, xrayPath, workDir string) *Scanner {
	return &Scanner{
		Config:      cfg,
		Noise:       noise,
		XrayPath:    xrayPath,
		WorkDir:     workDir,
		Concurrency: 12,
		Timeout:     6 * time.Second,
	}
}

func (s *Scanner) nextPort() int {
	return int(s.portCounter.Add(1))
}

func (s *Scanner) testEndpoint(endpoint string) ScanResult {
	socksPort := s.nextPort() + 10799

	xm := &XrayManager{
		XrayPath: s.XrayPath,
		WorkDir:  s.WorkDir,
		Config:   s.Config,
		Noise:    s.Noise,
	}

	configPath, err := xm.GenerateConfig(endpoint, socksPort)
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("config: %v", err)}
	}

	cmd, err := xm.StartXray(configPath)
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("start: %v", err)}
	}

	defer xm.StopXray(cmd)

	if !xm.WaitForPort(socksPort, 4*time.Second) {
		return ScanResult{Endpoint: endpoint, Error: "xray startup timeout"}
	}

	start := time.Now()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), 3*time.Second)
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("socks connect: %v", err)}
	}

	// SOCKS5 handshake
	if err := socks5Handshake(conn, "www.gstatic.com", 80); err != nil {
		conn.Close()
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("socks5: %v", err)}
	}

	// HTTP GET
	req := "GET /generate_204 HTTP/1.1\r\nHost: www.gstatic.com\r\nConnection: close\r\nUser-Agent: Mozilla/5.0\r\n\r\n"
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("http write: %v", err)}
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	conn.Close()
	if err != nil {
		return ScanResult{Endpoint: endpoint, Error: fmt.Sprintf("http read: %v", err)}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 204 {
		latency := time.Since(start)
		return ScanResult{Endpoint: endpoint, Success: true, Latency: latency}
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

func (s *Scanner) Run(endpoints []string) []ScanResult {
	var (
		mu      sync.Mutex
		results []ScanResult
		wg      sync.WaitGroup
		sem     = make(chan struct{}, s.Concurrency)
	)

	for _, ep := range endpoints {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := s.testEndpoint(endpoint)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if result.Success {
				fmt.Printf("  \x1b[32m✓\x1b[0m %-30s %v\n", endpoint, result.Latency.Round(time.Millisecond))
			} else {
				fmt.Printf("  \x1b[31m✗\x1b[0m %-30s %s\n", endpoint, result.Error)
			}
		}(ep)
	}

	wg.Wait()
	return results
}
