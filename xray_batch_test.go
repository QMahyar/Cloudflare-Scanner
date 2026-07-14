package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readBatchConfig parses a written batch config.json and cleans up its dir.
func readBatchConfig(t *testing.T, configPath string) XrayConfig {
	t.Helper()
	defer os.RemoveAll(filepath.Dir(configPath))
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg XrayConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return cfg
}

// assertBatchShape checks the structural invariants both batch builders share:
// one SOCKS inbound + one outbound + one routing rule per endpoint, sequential
// ports from basePort, in/out tag wiring, and trailing direct/block outbounds.
// This is the safety contract for the shared-builder refactor: it must pass
// identically before and after.
func assertBatchShape(t *testing.T, cfg XrayConfig, endpoints []string, ports []int, basePort int) {
	t.Helper()
	n := len(endpoints)
	if len(cfg.Inbounds) != n {
		t.Fatalf("want %d inbounds, got %d", n, len(cfg.Inbounds))
	}
	if len(cfg.Outbounds) != n+2 {
		t.Fatalf("want %d outbounds (n+direct+block), got %d", n+2, len(cfg.Outbounds))
	}
	if cfg.Routing == nil || len(cfg.Routing.Rules) != n {
		t.Fatalf("want %d routing rules", n)
	}
	if len(ports) != n {
		t.Fatalf("want %d ports, got %d", n, len(ports))
	}
	for i := 0; i < n; i++ {
		if ports[i] != basePort+i {
			t.Errorf("ports[%d] = %d, want %d", i, ports[i], basePort+i)
		}
		in := cfg.Inbounds[i]
		if in.Port != basePort+i || in.Listen != "127.0.0.1" || in.Protocol != "socks" {
			t.Errorf("inbound %d: got port=%d listen=%q proto=%q", i, in.Port, in.Listen, in.Protocol)
		}
		var socks map[string]interface{}
		if err := json.Unmarshal(in.Settings, &socks); err != nil || socks["auth"] != "noauth" || socks["udp"] != true {
			t.Errorf("inbound %d: unexpected socks settings %s", i, string(in.Settings))
		}
		rule := cfg.Routing.Rules[i]
		if len(rule.InboundTag) != 1 || rule.InboundTag[0] != in.Tag || rule.OutboundTag != cfg.Outbounds[i].Tag {
			t.Errorf("rule %d: %v -> %s (want inbound %s -> outbound %s)",
				i, rule.InboundTag, rule.OutboundTag, in.Tag, cfg.Outbounds[i].Tag)
		}
	}
	if cfg.Outbounds[n].Tag != "direct" || cfg.Outbounds[n].Protocol != "freedom" {
		t.Errorf("outbound[n] should be direct/freedom, got %s/%s", cfg.Outbounds[n].Tag, cfg.Outbounds[n].Protocol)
	}
	if cfg.Outbounds[n+1].Tag != "block" || cfg.Outbounds[n+1].Protocol != "blackhole" {
		t.Errorf("outbound[n+1] should be block/blackhole, got %s/%s", cfg.Outbounds[n+1].Tag, cfg.Outbounds[n+1].Protocol)
	}
	if cfg.Log == nil || cfg.Log.Loglevel != "warning" {
		t.Error("log block missing or wrong loglevel")
	}
	if cfg.Routing.DomainStrategy != "AsIs" {
		t.Errorf("domainStrategy %q, want AsIs", cfg.Routing.DomainStrategy)
	}
}

// TestBatchConfigWireGuard covers the WARP noise-fallback batch builder
// (GenerateConfigBatch): shared shape + a wireguard outbound per endpoint whose
// peer Endpoint is the batch entry.
func TestBatchConfigWireGuard(t *testing.T) {
	xm := &XrayManager{
		XrayPath: "unused",
		Config: &WarpConfig{
			PrivateKey: "KPfZ9oNYi6etptKQKEMqynTQRrxqrDem/It8rwjUe2Q=",
			Addresses:  []string{"172.16.0.2/32"},
			PublicKey:  "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
			Reserved:   []int{1, 2, 3},
			MTU:        1280,
		},
		Noise: NoiseConfig{Type: "rand", Packet: "50-100", Delay: "10-20", Count: 5},
	}
	endpoints := []string{"188.114.97.1:2408", "162.159.192.1:500", "188.114.98.9:1701"}
	basePort := 10816

	configPath, ports, err := xm.GenerateConfigBatch(endpoints, basePort)
	if err != nil {
		t.Fatalf("GenerateConfigBatch: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(configPath), "_xray_work/wgbatch_") {
		t.Errorf("config dir should be under _xray_work/wgbatch_, got %s", configPath)
	}
	cfg := readBatchConfig(t, configPath)
	assertBatchShape(t, cfg, endpoints, ports, basePort)

	// Each outbound must be a wireguard outbound pointed at its endpoint.
	for i, ep := range endpoints {
		ob := cfg.Outbounds[i]
		if ob.Protocol != "wireguard" {
			t.Errorf("outbound %d protocol %q, want wireguard", i, ob.Protocol)
		}
		var wg WireGuardSettings
		if err := json.Unmarshal(ob.Settings, &wg); err != nil {
			t.Fatalf("outbound %d settings: %v", i, err)
		}
		if len(wg.Peers) != 1 || wg.Peers[0].Endpoint != ep {
			t.Errorf("outbound %d peer endpoint %v, want %s", i, wg.Peers, ep)
		}
		if wg.SecretKey != xm.Config.PrivateKey {
			t.Errorf("outbound %d secretKey mismatch", i)
		}
		if len(wg.Noises) != 1 || wg.Noises[0].Type != "rand" {
			t.Errorf("outbound %d noise not applied: %v", i, wg.Noises)
		}
	}
}

// TestBatchConfigProxy covers the clean-IP Phase-2 batch builder
// (BuildXrayJSONBatch): shared shape + a per-endpoint outbound built from the
// proxy config repointed at each IP.
func TestBatchConfigProxy(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol: "vless",
		UUID:     "uuid-1234",
		Address:  "example.com",
		Port:     443,
		Security: "tls",
		SNI:      "example.com",
		Network:  "ws",
		Path:     "/ws",
		Host:     "example.com",
	}
	endpoints := []string{"1.2.3.4:443", "5.6.7.8:2053"}
	basePort := 20832

	configPath, ports, err := cfg.BuildXrayJSONBatch(endpoints, basePort)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(configPath), "_xray_clean/batch_") {
		t.Errorf("config dir should be under _xray_clean/batch_, got %s", configPath)
	}
	xc := readBatchConfig(t, configPath)
	assertBatchShape(t, xc, endpoints, ports, basePort)

	for i := range endpoints {
		ob := xc.Outbounds[i]
		if ob.Protocol != "vless" {
			t.Errorf("outbound %d protocol %q, want vless", i, ob.Protocol)
		}
		if len(ob.Settings) == 0 {
			t.Errorf("outbound %d missing settings", i)
		}
	}

	// Empty UUID must be rejected (guard preserved by the refactor).
	bad := *cfg
	bad.UUID = ""
	if _, _, err := bad.BuildXrayJSONBatch(endpoints, basePort); err == nil {
		t.Error("expected error for empty UUID")
	}
}
