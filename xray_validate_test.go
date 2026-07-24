package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// resolveXrayBin returns the path to an xray binary for config-shape validation,
// or skips the test. The validation is opt-in so CI (which bundles no xray and
// only runs `go test` on linux/amd64) stays green: set XRAY_VALIDATE=1 and,
// optionally, XRAY_BIN=/path/to/xray. When XRAY_BIN is unset it falls back to an
// `xray`/`xray.exe` on PATH or next to the test binary's working directory.
func resolveXrayBin(t *testing.T) string {
	t.Helper()
	if os.Getenv("XRAY_VALIDATE") != "1" {
		t.Skip("set XRAY_VALIDATE=1 (and optionally XRAY_BIN) to validate configs against a local xray-core")
	}
	if bin := os.Getenv("XRAY_BIN"); bin != "" {
		return bin
	}
	name := "xray"
	if runtime.GOOS == "windows" {
		name = "xray.exe"
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if _, err := os.Stat(name); err == nil {
		abs, _ := filepath.Abs(name)
		return abs
	}
	t.Fatalf("XRAY_VALIDATE=1 but no xray binary found (set XRAY_BIN, or put %s on PATH / in the working dir)", name)
	return ""
}

// xrayTest runs `xray run -test -c <configPath>`, which loads and validates the
// config without binding any listeners or touching the network. A non-zero exit
// means the config shape is rejected by this xray version.
func xrayTest(t *testing.T, bin, configPath string) {
	t.Helper()
	out, err := exec.Command(bin, "run", "-test", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Errorf("xray rejected config %s: %v\n%s", filepath.Base(configPath), err, string(out))
	}
}

// TestXrayConfigValid validates every generated outbound/transport shape against
// a local xray-core (opt-in). This is the migration safety net: any shape the
// bundled xray rejects fails here before it can break a real scan. Covers the
// WARP WireGuard-noise builder and the clean-IP Phase-2 proxy builder across all
// supported protocols and transports.
func TestXrayConfigValid(t *testing.T) {
	bin := resolveXrayBin(t)

	// WARP WireGuard + AmneziaWG noise (GenerateConfigBatch).
	t.Run("wireguard-noise", func(t *testing.T) {
		xm := &XrayManager{
			XrayPath: bin,
			Config: &WarpConfig{
				PrivateKey: "pstiyhXYh1zksH3Wc7Fbaz7cyeMssM/VdYNxKI4IvfA=",
				Addresses:  []string{"172.16.0.2/32"},
				PublicKey:  "SSS4YQvI4g1Nebt0Stm6BnfZR7p5IiVl6icQg7JQ9nM=",
				Reserved:   []int{1, 2, 3},
				MTU:        1280,
			},
			Noise: NoiseConfig{Type: "rand", Packet: "50-100", Delay: "10-20", Count: 5},
		}
		configPath, _, err := xm.GenerateConfigBatch([]string{"188.114.97.1:2408"}, 10816)
		if err != nil {
			t.Fatalf("GenerateConfigBatch: %v", err)
		}
		defer os.RemoveAll(filepath.Dir(configPath))
		xrayTest(t, bin, configPath)
	})

	// Clean-IP Phase-2 proxy shapes (BuildXrayJSONBatch).
	base := 20800
	cases := []struct {
		name string
		cfg  ProxyConfig
	}{
		{"vless-tls-ws", ProxyConfig{Protocol: "vless", UUID: "uuid-1234", Address: "example.com", Port: 443, Encryption: "none", Security: "tls", SNI: "example.com", Network: "ws", Path: "/ws", Host: "example.com"}},
		{"vless-reality", ProxyConfig{Protocol: "vless", UUID: "uuid-1234", Address: "example.com", Port: 443, Encryption: "none", Flow: "xtls-rprx-vision", Security: "reality", SNI: "example.com", PublicKey: "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0", ShortId: "6ba85179e30d4fc2"}},
		{"trojan-tcp-tls", ProxyConfig{Protocol: "trojan", UUID: "trojan-pass", Address: "example.com", Port: 443, Security: "tls", SNI: "example.com", Network: "tcp"}},
		{"vmess-ws-tls", ProxyConfig{Protocol: "vmess", UUID: "vmess-uuid-abcd", Address: "example.com", Port: 443, Encryption: "", Security: "tls", SNI: "example.com", Network: "ws", Path: "/vmess", Host: "example.com"}},
		{"vless-grpc-tls", ProxyConfig{Protocol: "vless", UUID: "uuid-1234", Address: "example.com", Port: 443, Encryption: "none", Security: "tls", SNI: "example.com", Network: "grpc", ServiceName: "gsvc", Mode: "multi"}},
		{"vless-mkcp", ProxyConfig{Protocol: "vless", UUID: "uuid-1234", Address: "example.com", Port: 2408, Encryption: "none", Network: "kcp"}},
		{"vless-httpupgrade-tls", ProxyConfig{Protocol: "vless", UUID: "uuid-1234", Address: "example.com", Port: 443, Encryption: "none", Security: "tls", SNI: "example.com", Network: "httpupgrade", Path: "/hu", Host: "example.com"}},
		{"vless-tcp-http", ProxyConfig{Protocol: "vless", UUID: "uuid-1234", Address: "example.com", Port: 80, Encryption: "none", Network: "tcp", HeaderType: "http"}},
	}
	for i, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configPath, _, err := tc.cfg.BuildXrayJSONBatch([]string{"1.2.3.4:443"}, base+i*8)
			if err != nil {
				t.Fatalf("BuildXrayJSONBatch: %v", err)
			}
			defer os.RemoveAll(filepath.Dir(configPath))
			xrayTest(t, bin, configPath)
		})
	}
}

// TestProbeConfigOmitsAllowInsecure is a regression guard for the xray v26.2.6
// removal of the TLS allowInsecure option: the Phase-2 probe builder must never
// emit that key, even when the source config has AllowInsecure=true. Runs always
// (no local xray required).
func TestProbeConfigOmitsAllowInsecure(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol:      "vless",
		UUID:          "uuid-1234",
		Address:       "example.com",
		Port:          443,
		Encryption:    "none",
		Security:      "tls",
		SNI:           "example.com",
		Network:       "tcp",
		AllowInsecure: true, // set on the source config; must NOT reach the probe config
	}
	configPath, _, err := cfg.BuildXrayJSONBatch([]string{"1.2.3.4:443"}, 20960)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(configPath))
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "allowInsecure") {
		t.Errorf("probe config leaked allowInsecure (removed from xray v26.2.6):\n%s", string(data))
	}
}
