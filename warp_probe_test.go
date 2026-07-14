package main

import (
	"bufio"
	"context"
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

	prober, err := newWarpProber(cfg)
	if err != nil {
		t.Fatalf("build prober: %v", err)
	}
	rtt, err := prober.Probe(context.Background(), endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}
	t.Logf("handshake OK — RTT %v", rtt.Round(time.Millisecond))
}

// TestWarpProbeHonorsContext verifies a cancelled context aborts an in-flight
// probe far sooner than its timeout — so a user Stop is responsive even on a long
// (up to 60s) timeout. Uses a black-hole UDP address so no real handshake occurs.
func TestWarpProbeHonorsContext(t *testing.T) {
	cfg, err := ParseWarpConfig(writeSampleConf(t))
	if err != nil {
		t.Fatalf("parse conf: %v", err)
	}
	prober, err := newWarpProber(cfg)
	if err != nil {
		t.Fatalf("build prober: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	start := time.Now()
	// 192.0.2.1 is TEST-NET-1 (RFC 5737) — guaranteed unreachable, so the probe
	// would otherwise run the full 30s.
	if _, perr := prober.Probe(ctx, "192.0.2.1:65535", 30*time.Second); perr == nil {
		t.Fatal("expected error on cancelled probe")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("probe ignored cancellation: took %v (want < 3s)", elapsed)
	}
}

// writeSampleConf writes a minimal valid WARP .conf to a temp file and returns its
// path. The keys are syntactically valid (base64) but need not be a real account —
// TestWarpProbeHonorsContext never completes a handshake.
func writeSampleConf(t *testing.T) string {
	t.Helper()
	// Syntactically valid Curve25519 keys (base64, 32 bytes) — test material only,
	// no real WARP account; the probe never completes a handshake.
	const conf = `[Interface]
PrivateKey = KPfZ9oNYi6etptKQKEMqynTQRrxqrDem/It8rwjUe2Q=
Address = 172.16.0.2/32
[Peer]
PublicKey = bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
Endpoint = 192.0.2.1:65535
`
	p := t.TempDir() + "/warp.conf"
	if err := os.WriteFile(p, []byte(conf), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}
