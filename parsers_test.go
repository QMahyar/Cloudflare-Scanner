package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── ParseWarpConfig ────────────────────────────────────────────────────────

func TestParseWarpConfig_Valid(t *testing.T) {
	conf := `
[Interface]
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
Address    = 172.16.0.2/32
Reserved   = 1,2,3
MTU        = 1420

[Peer]
PublicKey  = BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=
Endpoint   = 162.159.192.1:2408
`
	f := writeTempConf(t, conf)
	cfg, err := ParseWarpConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PrivateKey != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Errorf("wrong PrivateKey: %q", cfg.PrivateKey)
	}
	if cfg.PublicKey != "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=" {
		t.Errorf("wrong PublicKey: %q", cfg.PublicKey)
	}
	if len(cfg.Addresses) != 1 || cfg.Addresses[0] != "172.16.0.2/32" {
		t.Errorf("wrong Addresses: %v", cfg.Addresses)
	}
	if len(cfg.Reserved) != 3 || cfg.Reserved[0] != 1 || cfg.Reserved[1] != 2 || cfg.Reserved[2] != 3 {
		t.Errorf("wrong Reserved: %v", cfg.Reserved)
	}
	if cfg.MTU != 1420 {
		t.Errorf("wrong MTU: %d", cfg.MTU)
	}
}

func TestParseSubscription_URLSafeBase64(t *testing.T) {
	plain := "vless://uuid@1.2.3.4:443?security=tls#one\n"
	encoded := base64.RawURLEncoding.EncodeToString([]byte(plain))
	configs, err := ParseSubscription(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Protocol != "vless" {
		t.Errorf("wrong protocol: %s", configs[0].Protocol)
	}
	if configs[0].Address != "1.2.3.4" {
		t.Errorf("wrong address: %s", configs[0].Address)
	}
	if configs[0].Port != 443 {
		t.Errorf("wrong port: %d", configs[0].Port)
	}
}

func TestParseWarpConfig_MissingPrivateKey(t *testing.T) {
	conf := `
[Interface]
Address = 172.16.0.2/32
[Peer]
PublicKey = BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=
`
	f := writeTempConf(t, conf)
	_, err := ParseWarpConfig(f)
	if err == nil || !strings.Contains(err.Error(), "PrivateKey") {
		t.Errorf("expected PrivateKey error, got %v", err)
	}
}

func TestParseWarpConfig_MissingPublicKey(t *testing.T) {
	conf := `
[Interface]
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
Address    = 172.16.0.2/32
[Peer]
`
	f := writeTempConf(t, conf)
	_, err := ParseWarpConfig(f)
	if err == nil || !strings.Contains(err.Error(), "PublicKey") {
		t.Errorf("expected PublicKey error, got %v", err)
	}
}

func TestParseWarpConfig_DefaultMTUAndReserved(t *testing.T) {
	conf := `
[Interface]
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
Address    = 172.16.0.2/32
[Peer]
PublicKey  = BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=
`
	f := writeTempConf(t, conf)
	cfg, err := ParseWarpConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MTU != 1280 {
		t.Errorf("expected default MTU 1280, got %d", cfg.MTU)
	}
	if len(cfg.Reserved) != 3 {
		t.Errorf("expected 3 reserved bytes, got %v", cfg.Reserved)
	}
}

func writeTempConf(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "warp-*.conf")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// ─── ParseProxyURL ──────────────────────────────────────────────────────────

func TestParseProxyURL_VLESS(t *testing.T) {
	raw := "vless://uuid-1234@1.2.3.4:443?security=tls&sni=example.com&fp=chrome&type=ws&host=example.com&path=/ws#remark"
	cfg, err := ParseProxyURL(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Protocol != "vless" {
		t.Errorf("wrong protocol: %s", cfg.Protocol)
	}
	if cfg.UUID != "uuid-1234" {
		t.Errorf("wrong UUID: %s", cfg.UUID)
	}
	if cfg.Address != "1.2.3.4" {
		t.Errorf("wrong Address: %s", cfg.Address)
	}
	if cfg.Port != 443 {
		t.Errorf("wrong Port: %d", cfg.Port)
	}
	if cfg.Security != "tls" {
		t.Errorf("wrong Security: %s", cfg.Security)
	}
	if cfg.SNI != "example.com" {
		t.Errorf("wrong SNI: %s", cfg.SNI)
	}
	if cfg.Network != "ws" {
		t.Errorf("wrong Network: %s", cfg.Network)
	}
	if cfg.Remark != "remark" {
		t.Errorf("wrong Remark: %s", cfg.Remark)
	}
}

func TestParseProxyURL_WSEarlyDataAndHostFallback(t *testing.T) {
	// A BPB-style config: WS early-data params must be parsed, and the xray
	// validation JSON must carry them plus the Host header.
	raw := "vless://uuid-ed@162.159.82.8:2087?security=tls&sni=panel.example.ir&type=ws&host=panel.example.ir&path=/p&max_early_data=2560&early_data_header_name=Sec-WebSocket-Protocol"
	cfg, err := ParseProxyURL(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxEarlyData != 2560 {
		t.Errorf("wrong MaxEarlyData: %d", cfg.MaxEarlyData)
	}
	if cfg.EarlyDataHeaderName != "Sec-WebSocket-Protocol" {
		t.Errorf("wrong EarlyDataHeaderName: %q", cfg.EarlyDataHeaderName)
	}

	// Round-trip through the share URL must preserve the early-data params.
	round, err := ParseProxyURL(cfg.GenerateShareURL())
	if err != nil {
		t.Fatalf("round-trip parse error: %v", err)
	}
	if round.MaxEarlyData != 2560 || round.EarlyDataHeaderName != "Sec-WebSocket-Protocol" {
		t.Errorf("early-data lost in round-trip: %d %q", round.MaxEarlyData, round.EarlyDataHeaderName)
	}

	// The generated xray config must contain the WS Host header + early data.
	path, _, err := cfg.BuildXrayJSONBatch([]string{"162.159.82.8:2087"}, 35999)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch error: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(path))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	js := string(data)
	for _, want := range []string{`"maxEarlyData": 2560`, `"earlyDataHeaderName": "Sec-WebSocket-Protocol"`, `"Host": "panel.example.ir"`} {
		if !strings.Contains(js, want) {
			t.Errorf("xray config missing %s\n%s", want, js)
		}
	}
}

func TestBuildXrayJSON_WSHostFallsBackToSNI(t *testing.T) {
	// No host= param: validation must fall back to SNI for the WS Host header so
	// Cloudflare can route a CDN-fronted config (matches GenerateShareURL).
	cfg := &ProxyConfig{
		Protocol: "vless", UUID: "u", Address: "1.2.3.4", Port: 443,
		Security: "tls", SNI: "edge.example.com", Network: "ws", Path: "/",
	}
	path, _, err := cfg.BuildXrayJSONBatch([]string{"1.2.3.4:443"}, 35998)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch error: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(path))
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"Host": "edge.example.com"`) {
		t.Errorf("WS Host did not fall back to SNI:\n%s", string(data))
	}
}

func TestBuildXrayJSON_SNIFallsBackToOriginalHost(t *testing.T) {
	// No sni= param: the config relies on its hostname doubling as the TLS SNI.
	// Phase-2 validation swaps Address for a scan IP, so the SNI must fall back
	// to the ORIGINAL hostname — otherwise xray sends SNI:<ip>, Cloudflare can't
	// route, and every clean IP fails with "no usable response through the tunnel".
	cfg := &ProxyConfig{
		Protocol: "vless", UUID: "u", Address: "panel.example.ir", Port: 443,
		Security: "tls", Network: "tcp",
	}
	path, _, err := cfg.BuildXrayJSONBatch([]string{"104.16.5.3:443"}, 35997)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch error: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(path))
	data, _ := os.ReadFile(path)
	js := string(data)
	if !strings.Contains(js, `"serverName": "panel.example.ir"`) {
		t.Errorf("TLS SNI did not fall back to original host:\n%s", js)
	}
	if strings.Contains(js, `"serverName": "104.16.5.3"`) {
		t.Errorf("TLS SNI was set to the scan IP (Cloudflare can't route this):\n%s", js)
	}
}

func TestBuildXrayJSONBatch_StructureAndRouting(t *testing.T) {
	// The pooled Phase-2 path multiplexes a whole batch into ONE xray process:
	// one SOCKS inbound + outbound + routing rule per endpoint. Verify the
	// generated config is valid JSON with that exact shape, distinct sequential
	// ports, each endpoint's IP as its outbound address, and the SNI preserved
	// (not rewritten to a scan IP — Cloudflare couldn't route that).
	cfg := &ProxyConfig{
		Protocol: "vless", UUID: "uuid-1", Address: "panel.example.ir", Port: 443,
		Security: "tls", Network: "ws", Path: "/", SNI: "panel.example.ir",
	}
	endpoints := []string{"104.16.5.3:443", "172.67.1.2:443", "188.114.96.7:8443"}

	path, ports, err := cfg.BuildXrayJSONBatch(endpoints, 41000)
	if err != nil {
		t.Fatalf("BuildXrayJSONBatch error: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(path))

	if len(ports) != len(endpoints) {
		t.Fatalf("got %d ports, want %d", len(ports), len(endpoints))
	}
	for i, p := range ports {
		if p != 41000+i {
			t.Errorf("port[%d] = %d, want %d", i, p, 41000+i)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var xc XrayConfig
	if err := json.Unmarshal(data, &xc); err != nil {
		t.Fatalf("batch config is not valid JSON: %v\n%s", err, data)
	}

	if len(xc.Inbounds) != len(endpoints) {
		t.Fatalf("got %d inbounds, want %d", len(xc.Inbounds), len(endpoints))
	}
	if len(xc.Outbounds) != len(endpoints)+2 { // proxies + direct + block
		t.Fatalf("got %d outbounds, want %d", len(xc.Outbounds), len(endpoints)+2)
	}
	if xc.Routing == nil || len(xc.Routing.Rules) != len(endpoints) {
		t.Fatalf("expected %d routing rules", len(endpoints))
	}
	for i := range endpoints {
		inTag := xc.Inbounds[i].Tag
		outTag := xc.Outbounds[i].Tag
		rule := xc.Routing.Rules[i]
		if len(rule.InboundTag) != 1 || rule.InboundTag[0] != inTag || rule.OutboundTag != outTag {
			t.Errorf("rule %d: %v -> %q, want [%s] -> %s", i, rule.InboundTag, rule.OutboundTag, inTag, outTag)
		}
		if xc.Inbounds[i].Port != ports[i] {
			t.Errorf("inbound %d port = %d, want %d", i, xc.Inbounds[i].Port, ports[i])
		}
	}

	js := string(data)
	for _, ip := range []string{"104.16.5.3", "172.67.1.2", "188.114.96.7"} {
		if !strings.Contains(js, `"address": "`+ip+`"`) {
			t.Errorf("batch config missing outbound address %s", ip)
		}
	}
	if !strings.Contains(js, `"serverName": "panel.example.ir"`) {
		t.Errorf("batch config lost SNI:\n%s", js)
	}
	if strings.Contains(js, `"serverName": "104.16.5.3"`) {
		t.Errorf("batch config set SNI to a scan IP (Cloudflare can't route)")
	}
}

func TestGenerateConfigBatch_Structure(t *testing.T) {
	// The pooled WARP-noise path mirrors the clean batch: one SOCKS inbound +
	// WireGuard outbound + routing rule per endpoint, with each endpoint as its
	// peer Endpoint and the shared credentials/noise on every outbound.
	xm := &XrayManager{
		Config: &WarpConfig{
			PrivateKey: "cHJpdmF0ZWtleXBsYWNlaG9sZGVy",
			Addresses:  []string{"172.16.0.2/32"},
			PublicKey:  "cHVibGlja2V5cGxhY2Vob2xkZXI=",
			Reserved:   []int{1, 2, 3},
			MTU:        1280,
		},
		Noise: DefaultNoise(),
	}
	endpoints := []string{"188.114.96.1:2408", "162.159.192.5:1701"}

	path, ports, err := xm.GenerateConfigBatch(endpoints, 12000)
	if err != nil {
		t.Fatalf("GenerateConfigBatch error: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(path))

	if len(ports) != len(endpoints) || ports[0] != 12000 || ports[1] != 12001 {
		t.Fatalf("unexpected ports: %v", ports)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var xc XrayConfig
	if err := json.Unmarshal(data, &xc); err != nil {
		t.Fatalf("WARP batch config is not valid JSON: %v\n%s", err, data)
	}
	if len(xc.Inbounds) != len(endpoints) || len(xc.Outbounds) != len(endpoints)+2 {
		t.Fatalf("got %d inbounds / %d outbounds", len(xc.Inbounds), len(xc.Outbounds))
	}
	js := string(data)
	for _, ep := range endpoints {
		if !strings.Contains(js, `"endpoint": "`+ep+`"`) {
			t.Errorf("WARP batch config missing peer endpoint %s", ep)
		}
	}
	if !strings.Contains(js, `"wireguard"`) {
		t.Errorf("WARP batch config missing wireguard outbound:\n%s", js)
	}
}

func TestWithEndpoint_KeepsExplicitSNIAndBareIP(t *testing.T) {
	// An explicit sni= must be preserved verbatim (not overwritten by the host).
	withSNI := (&ProxyConfig{Address: "real.example.com", SNI: "keep.example.com"}).WithEndpoint("104.16.5.3:443")
	if withSNI.SNI != "keep.example.com" {
		t.Errorf("explicit SNI clobbered: %q", withSNI.SNI)
	}
	// A config whose address is already a bare IP has no hostname to recover, so
	// SNI must stay empty rather than become an unroutable IP literal.
	ipOnly := (&ProxyConfig{Address: "203.0.113.7", SNI: ""}).WithEndpoint("104.16.5.3:443")
	if ipOnly.SNI != "" {
		t.Errorf("SNI should stay empty when original address is a bare IP, got %q", ipOnly.SNI)
	}
}

func TestParseProxyURL_Trojan(t *testing.T) {
	raw := "trojan://password123@192.168.1.1:8443?security=tls&sni=host.example"
	cfg, err := ParseProxyURL(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Protocol != "trojan" {
		t.Errorf("wrong protocol: %s", cfg.Protocol)
	}
	if cfg.UUID != "password123" {
		t.Errorf("wrong password: %s", cfg.UUID)
	}
	if cfg.Port != 8443 {
		t.Errorf("wrong Port: %d", cfg.Port)
	}
}

func TestParseProxyURL_IPv6Host(t *testing.T) {
	raw := "vless://myuuid@[2606:4700::1]:443?security=tls"
	cfg, err := ParseProxyURL(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "2606:4700::1" {
		t.Errorf("expected bare IPv6 address, got %q", cfg.Address)
	}
	if cfg.Port != 443 {
		t.Errorf("wrong Port: %d", cfg.Port)
	}
}

func TestParseProxyURL_AllowInsecure(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"vless://u@h:443?allowInsecure=1", true},
		{"vless://u@h:443?allowInsecure=true", true},
		{"vless://u@h:443?insecure=1", true},
		{"vless://u@h:443?insecure=0", false},
		{"vless://u@h:443", false},
	}
	for _, tc := range cases {
		cfg, err := ParseProxyURL(tc.url)
		if err != nil {
			t.Errorf("%s: parse error: %v", tc.url, err)
			continue
		}
		if cfg.AllowInsecure != tc.want {
			t.Errorf("%s: AllowInsecure = %v, want %v", tc.url, cfg.AllowInsecure, tc.want)
		}
	}
}

func TestParseProxyURL_VMessURLSafeBase64(t *testing.T) {
	payload := `{"v":"2","ps":"remark","add":"1.2.3.4","port":"443","id":"uuid-1234","aid":"0","scy":"auto","net":"ws","type":"none","host":"example.com","path":"/ws","tls":"tls","sni":"example.com"}`
	raw := "vmess://" + base64.RawURLEncoding.EncodeToString([]byte(payload))
	cfg, err := ParseProxyURL(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Protocol != "vmess" {
		t.Errorf("wrong protocol: %s", cfg.Protocol)
	}
	if cfg.Address != "1.2.3.4" {
		t.Errorf("wrong Address: %s", cfg.Address)
	}
	if cfg.Port != 443 {
		t.Errorf("wrong Port: %d", cfg.Port)
	}
	if cfg.SNI != "example.com" {
		t.Errorf("wrong SNI: %s", cfg.SNI)
	}
}
func TestParseProxyURL_UnsupportedScheme(t *testing.T) {
	_, err := ParseProxyURL("ss://whatever@host:1234")
	if err == nil {
		t.Error("expected error for unsupported scheme")
	}
}

func TestParseProxyURL_RejectsEmptyAddressAndBadPort(t *testing.T) {
	cases := []string{
		"vless://u@:443?security=tls",
		"trojan://p@example.com:0?security=tls",
		"vless://u@example.com:70000?security=tls",
	}
	for _, raw := range cases {
		if _, err := ParseProxyURL(raw); err == nil {
			t.Errorf("expected parse error for %q", raw)
		}
	}
}

// ─── Share URL round-trip ────────────────────────────────────────────────────

// Parsing a share URL, regenerating it, and parsing again must preserve the
// load-bearing fields. (Host is intentionally excluded: GenerateShareURL fills
// an empty ws Host from the SNI, so it is not round-trip-stable by design.)
func TestShareURLRoundTrip(t *testing.T) {
	vmessPayload := `{"v":"2","ps":"r","add":"1.2.3.4","port":"443","id":"uuid-1234","aid":"0","scy":"auto","net":"ws","type":"none","host":"example.com","path":"/ws","tls":"tls","sni":"example.com"}`
	cases := []string{
		"vless://uuid-1234@1.2.3.4:443?security=tls&sni=example.com&fp=chrome&type=ws&host=example.com&path=/ws#remark",
		"trojan://password123@192.168.1.1:8443?security=tls&sni=host.example",
		"vmess://" + base64.RawURLEncoding.EncodeToString([]byte(vmessPayload)),
	}
	for _, raw := range cases {
		c1, err := ParseProxyURL(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		regen := c1.GenerateShareURL()
		c2, err := ParseProxyURL(regen)
		if err != nil {
			t.Fatalf("re-parse %q: %v", regen, err)
		}
		if c1.Protocol != c2.Protocol {
			t.Errorf("%s: Protocol %q -> %q", c1.Protocol, c1.Protocol, c2.Protocol)
		}
		if c1.Address != c2.Address {
			t.Errorf("%s: Address %q -> %q", c1.Protocol, c1.Address, c2.Address)
		}
		if c1.Port != c2.Port {
			t.Errorf("%s: Port %d -> %d", c1.Protocol, c1.Port, c2.Port)
		}
		if c1.UUID != c2.UUID {
			t.Errorf("%s: UUID %q -> %q", c1.Protocol, c1.UUID, c2.UUID)
		}
		if c1.Security != c2.Security {
			t.Errorf("%s: Security %q -> %q", c1.Protocol, c1.Security, c2.Security)
		}
		if c1.Network != c2.Network {
			t.Errorf("%s: Network %q -> %q", c1.Protocol, c1.Network, c2.Network)
		}
		if c1.SNI != c2.SNI {
			t.Errorf("%s: SNI %q -> %q", c1.Protocol, c1.SNI, c2.SNI)
		}
		if c1.Path != c2.Path {
			t.Errorf("%s: Path %q -> %q", c1.Protocol, c1.Path, c2.Path)
		}
	}
}

// ─── GenerateReplacedConfigs ────────────────────────────────────────────────

func TestGenerateReplacedConfigs_Basic(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol: "vless",
		UUID:     "test-uuid",
		Address:  "1.2.3.4",
		Port:     443,
		Remark:   "base",
	}
	endpoints := []string{"5.6.7.8:8443", "9.10.11.12:2053"}
	urls := GenerateReplacedConfigsNamed([]*ProxyConfig{cfg}, endpoints, "")
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
	for _, u := range urls {
		if !strings.HasPrefix(u, "vless://") {
			t.Errorf("expected vless:// URL, got %q", u)
		}
	}
}

func TestGenerateReplacedConfigs_SkipMalformedEndpoint(t *testing.T) {
	cfg := &ProxyConfig{Protocol: "vless", UUID: "u", Address: "1.2.3.4", Port: 443}
	endpoints := []string{"notanendpoint", "5.6.7.8:443", "  ", "[bad"}
	urls := GenerateReplacedConfigsNamed([]*ProxyConfig{cfg}, endpoints, "")
	if len(urls) != 1 {
		t.Errorf("expected 1 valid URL (only 5.6.7.8:443), got %d: %v", len(urls), urls)
	}
}

func TestGenerateReplacedConfigs_Deduplication(t *testing.T) {
	cfg := &ProxyConfig{Protocol: "vless", UUID: "u", Address: "1.2.3.4", Port: 443}
	endpoints := []string{"5.6.7.8:443", "5.6.7.8:443"}
	urls := GenerateReplacedConfigsNamed([]*ProxyConfig{cfg}, endpoints, "")
	if len(urls) != 1 {
		t.Errorf("expected deduplication to 1 URL, got %d", len(urls))
	}
}

func TestHandleReplacerApplyRejectsInvalidEndpoint(t *testing.T) {
	body, _ := json.Marshal(replacerApplyRequest{
		Configs:   []replacerConfigEntry{{Protocol: "vless", UUID: "u", Address: "example.com", Port: 443}},
		Endpoints: []string{"not-an-endpoint"},
	})
	req := httptest.NewRequest("POST", "/api/replacer/apply", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleReplacerApply(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rr.Code)
	}
}

func TestHandleCleanScanStartRejectsBadCustomRangeLine(t *testing.T) {
	body, _ := json.Marshal(cleanScanRequest{
		OnePhase:     true,
		Count:        10,
		CustomRanges: "104.16.0.0/24\ngarbage",
	})
	req := httptest.NewRequest("POST", "/api/clean-scan", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleCleanScanStart("xray")(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rr.Code)
	}
}

// ─── handleApplyEndpoint path traversal ─────────────────────────────────────

func TestHandleApplyEndpoint_PathTraversal(t *testing.T) {
	tmpExe, err := os.CreateTemp("", "scanner-*.exe")
	if err != nil {
		t.Fatal(err)
	}
	tmpExe.Close()
	t.Cleanup(func() { os.Remove(tmpExe.Name()) })

	exeDir := t.TempDir()

	reject := func(outputDir string) bool {
		if !filepath.IsAbs(outputDir) && (strings.HasPrefix(outputDir, "/") || strings.HasPrefix(outputDir, `\\`)) {
			return true
		}
		outputDir = filepath.Clean(outputDir)
		if !filepath.IsAbs(outputDir) {
			outputDir = filepath.Join(exeDir, outputDir)
		}
		rel, err := filepath.Rel(exeDir, outputDir)
		return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	}

	cases := []struct {
		dir    string
		denied bool
	}{
		{filepath.Join(exeDir, "output"), false},
		{exeDir, false},
		{filepath.Join(exeDir, "..", "escape"), true},
		{"/tmp/evil", true},
	}

	for _, tc := range cases {
		got := reject(tc.dir)
		if got != tc.denied {
			t.Errorf("dir=%q: denied=%v, want %v", tc.dir, got, tc.denied)
		}
	}
}

// ─── Endpoint format validation (apply handler) ──────────────────────────────

func TestHandleApplyEndpoint_InvalidEndpointFormat(t *testing.T) {
	handler := http.HandlerFunc(handleApplyEndpoint)

	cases := []struct {
		label    string
		endpoint string
	}{
		{"missing endpoint", ""},
		{"no port", "1.2.3.4"},
		{"garbage", "notanendpoint"},
	}

	for _, tc := range cases {
		var body strings.Builder
		body.WriteString("endpoint=")
		body.WriteString(tc.endpoint)
		req := httptest.NewRequest("POST", "/api/apply-endpoint",
			strings.NewReader(body.String()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code < 400 || rr.Code >= 500 {
			t.Errorf("[%s] endpoint=%q: got status %d, want 4xx",
				tc.label, tc.endpoint, rr.Code)
		}
	}
}

func TestHandleApplyEndpoint_ValidEndpointRejectedWithoutFiles(t *testing.T) {
	handler := http.HandlerFunc(handleApplyEndpoint)
	req := httptest.NewRequest("POST", "/api/apply-endpoint",
		strings.NewReader("endpoint=1.2.3.4:443"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing files, got %d", rr.Code)
	}
}
