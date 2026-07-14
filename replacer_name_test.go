package main

import (
	"strings"
	"testing"
)

func TestApplyNameTemplate(t *testing.T) {
	cfg := &ProxyConfig{Protocol: "vless", Remark: "MyNode"}
	cases := []struct {
		tmpl string
		want string
	}{
		{"", "MyNode @ 1.2.3.4:443"},
		{"{remark}-{ip}", "MyNode-1.2.3.4"},
		{"{proto} {ep} #{n}", "vless 1.2.3.4:443 #7"},
		{"{ip}:{port}", "1.2.3.4:443"},
	}
	for _, c := range cases {
		got := applyNameTemplate(c.tmpl, cfg, "1.2.3.4", 443, 7)
		if got != c.want {
			t.Errorf("applyNameTemplate(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}

	// Empty remark + empty template falls back to the bare endpoint.
	bare := &ProxyConfig{Protocol: "trojan"}
	if got := applyNameTemplate("", bare, "5.6.7.8", 8443, 1); got != "5.6.7.8:8443" {
		t.Errorf("empty-remark fallback = %q, want %q", got, "5.6.7.8:8443")
	}
}

// TestGenerateReplacedConfigsPinsSNI verifies the replacer pins the original
// CDN hostname into SNI before repointing Address at a scan IP. Without it, a
// CDN-fronted config (SNI implicitly = the hostname) would emit sni:<scan-IP>,
// which Cloudflare cannot route — a dead config.
func TestGenerateReplacedConfigsPinsSNI(t *testing.T) {
	// A vless config whose SNI is empty and Address is a CDN hostname.
	cfg := &ProxyConfig{
		Protocol: "vless",
		UUID:     "abc-123",
		Address:  "cdn.example.com",
		Port:     443,
		Security: "tls",
		Network:  "ws",
	}
	urls := GenerateReplacedConfigsNamed([]*ProxyConfig{cfg}, []string{"104.16.1.1:443"}, "")
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	// The generated share URL must carry sni=cdn.example.com, not the scan IP.
	if !strings.Contains(urls[0], "sni=cdn.example.com") {
		t.Errorf("expected original hostname pinned as SNI, got: %s", urls[0])
	}
	if strings.Contains(urls[0], "sni=104.16.1.1") {
		t.Errorf("SNI was wrongly set to the scan IP: %s", urls[0])
	}
	// The address must still be repointed at the scan IP.
	if !strings.Contains(urls[0], "104.16.1.1") {
		t.Errorf("expected address repointed to scan IP, got: %s", urls[0])
	}
}

// TestGenerateReplacedConfigsKeepsExplicitSNI verifies an explicit SNI is not
// clobbered by the pinning logic.
func TestGenerateReplacedConfigsKeepsExplicitSNI(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol: "vless",
		UUID:     "abc-123",
		Address:  "cdn.example.com",
		Port:     443,
		Security: "tls",
		SNI:      "explicit.sni.com",
	}
	urls := GenerateReplacedConfigsNamed([]*ProxyConfig{cfg}, []string{"104.16.1.1:443"}, "")
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	if !strings.Contains(urls[0], "sni=explicit.sni.com") {
		t.Errorf("expected explicit SNI preserved, got: %s", urls[0])
	}
}

// TestConfigFingerprint locks the dedup fingerprint's field coverage: identical
// configs share a fingerprint; a difference in any one fingerprint field yields a
// different fingerprint. Guards against silent config loss when a field is added
// to ProxyConfig but forgotten in ConfigFingerprint.
func TestConfigFingerprint(t *testing.T) {
	base := &ProxyConfig{
		Protocol: "vless", UUID: "u1", Encryption: "none", Security: "reality",
		SNI: "example.com", Fingerprint: "chrome", Network: "grpc", Host: "h",
		Path: "/p", PacketEncoding: "xudp", Flow: "xtls-rprx-vision", PublicKey: "pk",
		ShortId: "sid", SpiderX: "/", AllowInsecure: false, ALPN: "h2",
		HeaderType: "none", Mode: "gun", ServiceName: "svc", MaxEarlyData: 2048,
		EarlyDataHeaderName: "Sec-WebSocket-Protocol",
	}
	same := *base
	if ConfigFingerprint(base) != ConfigFingerprint(&same) {
		t.Fatal("identical configs must share a fingerprint")
	}
	// Each of these single-field mutations must change the fingerprint.
	mutations := map[string]func(*ProxyConfig){
		"UUID":                func(c *ProxyConfig) { c.UUID = "u2" },
		"Flow":                func(c *ProxyConfig) { c.Flow = "none" },
		"ShortId":             func(c *ProxyConfig) { c.ShortId = "sid2" },
		"ServiceName":         func(c *ProxyConfig) { c.ServiceName = "svc2" },
		"Path":                func(c *ProxyConfig) { c.Path = "/q" },
		"PublicKey":           func(c *ProxyConfig) { c.PublicKey = "pk2" },
		"AllowInsecure":       func(c *ProxyConfig) { c.AllowInsecure = true },
		"MaxEarlyData":        func(c *ProxyConfig) { c.MaxEarlyData = 4096 },
		"EarlyDataHeaderName": func(c *ProxyConfig) { c.EarlyDataHeaderName = "X" },
	}
	for field, mut := range mutations {
		diff := *base
		mut(&diff)
		if ConfigFingerprint(base) == ConfigFingerprint(&diff) {
			t.Errorf("configs differing in %s must not share a fingerprint", field)
		}
	}
}

// TestDeduplicateConfigs verifies true duplicates collapse (first-occurrence order
// preserved) while configs distinct only in a rarely-set field survive. The
// survive-by-Flow case is the regression guard: it fails if Flow is dropped from
// the fingerprint.
func TestDeduplicateConfigs(t *testing.T) {
	if got := DeduplicateConfigs(nil); len(got) != 0 {
		t.Fatalf("empty input should give empty output, got %v", got)
	}
	a := &ProxyConfig{Protocol: "vless", UUID: "u", Flow: "flow-a"}
	b := &ProxyConfig{Protocol: "vless", UUID: "u", Flow: "flow-b"} // differs only in Flow
	dupOfA := &ProxyConfig{Protocol: "vless", UUID: "u", Flow: "flow-a"}
	got := DeduplicateConfigs([]*ProxyConfig{a, b, dupOfA})
	if len(got) != 2 {
		t.Fatalf("want 2 distinct (a,b); got %d — Flow may be missing from the fingerprint", len(got))
	}
	if got[0] != a || got[1] != b {
		t.Fatalf("dedup must preserve first-occurrence order")
	}
}
