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
			PrivateKey: "pstiyhXYh1zksH3Wc7Fbaz7cyeMssM/VdYNxKI4IvfA=",
			Addresses:  []string{"172.16.0.2/32"},
			PublicKey:  "SSS4YQvI4g1Nebt0Stm6BnfZR7p5IiVl6icQg7JQ9nM=",
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

// TestBatchConfigProxyTrojan asserts Trojan outbound settings use the nested
// xray-core shape: settings.servers[0].{address,port,password}.
func TestBatchConfigProxyTrojan(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol: "trojan",
		UUID:     "trojan-password-xyz",
		Address:  "example.com",
		Port:     443,
		Security: "tls",
		SNI:      "example.com",
		Network:  "tcp",
	}
	endpoints := []string{"1.2.3.4:443"}
	basePort := 20901

	configPath, ports, err := cfg.BuildXrayJSONBatch(endpoints, basePort)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch: %v", err)
	}
	xc := readBatchConfig(t, configPath)
	assertBatchShape(t, xc, endpoints, ports, basePort)

	ob := xc.Outbounds[0]
	if ob.Protocol != "trojan" {
		t.Fatalf("protocol %q, want trojan", ob.Protocol)
	}
	var settings TrojanOutboundSettings
	if err := json.Unmarshal(ob.Settings, &settings); err != nil {
		t.Fatalf("unmarshal trojan settings: %v\nraw: %s", err, string(ob.Settings))
	}
	if len(settings.Servers) != 1 {
		t.Fatalf("want 1 server, got %d (raw %s)", len(settings.Servers), string(ob.Settings))
	}
	s := settings.Servers[0]
	// BuildXrayJSONBatch repoints the outbound at the endpoint IP/port.
	if s.Address != "1.2.3.4" {
		t.Errorf("address %q, want 1.2.3.4", s.Address)
	}
	if s.Port != 443 {
		t.Errorf("port %d, want 443", s.Port)
	}
	if s.Password != "trojan-password-xyz" {
		t.Errorf("password %q, want trojan-password-xyz", s.Password)
	}
}

// TestBatchConfigProxyVMess asserts VMess outbound settings use the nested
// xray-core shape: settings.vnext[0].users[0].{id,security,alterId}, with empty
// encryption defaulting to "auto".
func TestBatchConfigProxyVMess(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol:   "vmess",
		UUID:       "vmess-uuid-abcd",
		Address:    "example.com",
		Port:       443,
		Encryption: "", // should default to "auto"
		Security:   "tls",
		SNI:        "example.com",
		Network:    "ws",
		Path:       "/vmess",
		Host:       "example.com",
	}
	endpoints := []string{"5.6.7.8:2053"}
	basePort := 20911

	configPath, ports, err := cfg.BuildXrayJSONBatch(endpoints, basePort)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch: %v", err)
	}
	xc := readBatchConfig(t, configPath)
	assertBatchShape(t, xc, endpoints, ports, basePort)

	ob := xc.Outbounds[0]
	if ob.Protocol != "vmess" {
		t.Fatalf("protocol %q, want vmess", ob.Protocol)
	}
	var settings VMessOutboundSettings
	if err := json.Unmarshal(ob.Settings, &settings); err != nil {
		t.Fatalf("unmarshal vmess settings: %v\nraw: %s", err, string(ob.Settings))
	}
	if len(settings.VNext) != 1 {
		t.Fatalf("want 1 vnext, got %d (raw %s)", len(settings.VNext), string(ob.Settings))
	}
	srv := settings.VNext[0]
	if srv.Address != "5.6.7.8" {
		t.Errorf("address %q, want 5.6.7.8", srv.Address)
	}
	if srv.Port != 2053 {
		t.Errorf("port %d, want 2053", srv.Port)
	}
	if len(srv.Users) != 1 {
		t.Fatalf("want 1 user, got %d", len(srv.Users))
	}
	u := srv.Users[0]
	if u.ID != "vmess-uuid-abcd" {
		t.Errorf("id %q, want vmess-uuid-abcd", u.ID)
	}
	if u.Security != "auto" {
		t.Errorf("security %q, want auto", u.Security)
	}
	if u.AlterId != 0 {
		t.Errorf("alterId %d, want 0", u.AlterId)
	}
}
