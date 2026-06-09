package main

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"
)

// readEndpoint pulls the [Peer] Endpoint out of a WARP .conf (ParseWarpConfig
// intentionally ignores it). Used only by the integration test below.
func readEndpoint(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(strings.ToLower(line), "endpoint") {
			if i := strings.Index(line, "="); i > 0 {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ""
}

// TestWarpHandshakeProbe_Live verifies the native WireGuard handshake against a
// real WARP endpoint. It is skipped unless WARP_TEST_CONF points at a valid
// .conf, so CI (which has no credentials) stays green.
//
//	WARP_TEST_CONF="D:\Apps\Vpn\WARPGermany.conf" go test -run TestWarpHandshakeProbe_Live -v ./.
func TestWarpHandshakeProbe_Live(t *testing.T) {
	confPath := os.Getenv("WARP_TEST_CONF")
	if confPath == "" {
		t.Skip("set WARP_TEST_CONF to a WARP .conf to run the live handshake test")
	}

	cfg, err := ParseWarpConfig(confPath)
	if err != nil {
		t.Fatalf("parse conf: %v", err)
	}
	endpoint := readEndpoint(confPath)
	if endpoint == "" {
		endpoint = "162.159.192.1:2408" // any WARP edge works with valid account creds
	}
	t.Logf("probing %s (reserved=%v)", endpoint, cfg.Reserved)

	rtt, err := WarpHandshakeProbe(cfg, endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	t.Logf("handshake OK — RTT %v", rtt.Round(time.Millisecond))
}
