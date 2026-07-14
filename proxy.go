package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type ProxyConfig struct {
	Protocol       string
	UUID           string
	Address        string
	Port           int
	Encryption     string
	Security       string
	SNI            string
	Fingerprint    string
	Network        string
	Host           string
	Path           string
	PacketEncoding string
	Remark         string
	RawURL         string
	Flow           string
	PublicKey      string
	ShortId        string
	SpiderX        string
	AllowInsecure  bool
	ALPN           string
	HeaderType     string
	Mode           string
	ServiceName    string
	// WebSocket 0-RTT early data (e.g. BPB / edge-tunnel panels). Dropping these
	// during validation builds a WS stream the origin Worker may reject.
	MaxEarlyData        int
	EarlyDataHeaderName string
}

func parseInsecureFlag(v string) bool {
	return v == "1" || strings.EqualFold(v, "true")
}
func decodeBase64Loose(raw string) ([]byte, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		default:
			return r
		}
	}, clean)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(clean)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// looseString unmarshals a JSON value that some clients quote and others emit
// bare into a string. The vmess base64-JSON payload is wildly inconsistent
// across panels: `port`, `aid`, and `v` are frequently numbers, and
// `allowInsecure` is sometimes a bool, even though the de-facto v2rayN format
// quotes everything. Declaring those fields as plain `string` makes
// json.Unmarshal fail on the whole object, so one numeric field silently
// rejects an otherwise-valid config (and, for an all-vmess subscription, every
// config). This accepts either form so `"port":443` parses like `"port":"443"`.
type looseString string

func (s *looseString) UnmarshalJSON(b []byte) error {
	t := strings.TrimSpace(string(b))
	if t == "" || t == "null" {
		return nil
	}
	if t[0] == '"' {
		var str string
		if err := json.Unmarshal([]byte(t), &str); err != nil {
			return err
		}
		*s = looseString(str)
		return nil
	}
	// Bare number/bool token (e.g. 443, true) — keep its textual form verbatim.
	*s = looseString(t)
	return nil
}

func ParseVMessURL(rawURL string) (*ProxyConfig, error) {
	if !strings.HasPrefix(rawURL, "vmess://") {
		return nil, fmt.Errorf("not a vmess URL")
	}

	b64 := strings.TrimPrefix(rawURL, "vmess://")
	decoded, err := decodeBase64Loose(b64)
	if err != nil {
		return nil, fmt.Errorf("vmess base64 decode: %w", err)
	}

	// Numeric-or-string fields use looseString; the rest are reliably quoted.
	var vmess struct {
		V             looseString `json:"v"`
		Remark        string      `json:"ps"`
		Address       string      `json:"add"`
		Port          looseString `json:"port"`
		ID            string      `json:"id"`
		Aid           looseString `json:"aid"`
		Security      string      `json:"scy"`
		Network       string      `json:"net"`
		Type          string      `json:"type"`
		Host          string      `json:"host"`
		Path          string      `json:"path"`
		TLS           string      `json:"tls"`
		SNI           string      `json:"sni"`
		ALPN          string      `json:"alpn"`
		Fingerprint   string      `json:"fp"`
		AllowInsecure looseString `json:"allowInsecure"`
		Flow          string      `json:"flow"`
	}

	if err := json.Unmarshal(decoded, &vmess); err != nil {
		return nil, fmt.Errorf("vmess json decode: %w", err)
	}

	if vmess.Address == "" {
		return nil, fmt.Errorf("vmess: empty address")
	}

	port, _ := strconv.Atoi(strings.TrimSpace(string(vmess.Port)))
	if port <= 0 || port > 65535 {
		port = 443
	}

	security := "none"
	if vmess.TLS == "tls" {
		security = "tls"
	}

	sni := vmess.SNI
	if sni == "" {
		sni = vmess.Host
	}
	if sni == "" {
		sni = vmess.Address
	}

	netType := vmess.Network
	if netType == "" {
		netType = "tcp"
	}

	path := vmess.Path
	if path == "" {
		path = "/"
	}

	cfg := &ProxyConfig{
		Protocol:    "vmess",
		UUID:        vmess.ID,
		Address:     vmess.Address,
		Port:        port,
		Encryption:  vmess.Security,
		Security:    security,
		SNI:         sni,
		Fingerprint: vmess.Fingerprint,
		Network:     netType,
		Host:        vmess.Host,
		Path:        path,
		Flow:        vmess.Flow,
		Remark:      vmess.Remark,
		HeaderType:  vmess.Type,
		ALPN:        vmess.ALPN,
		RawURL:      rawURL,
	}

	if parseInsecureFlag(string(vmess.AllowInsecure)) {
		cfg.AllowInsecure = true
	}

	return cfg, nil
}

func ParseProxyURL(rawURL string) (*ProxyConfig, error) {
	if strings.HasPrefix(rawURL, "vmess://") {
		return ParseVMessURL(rawURL)
	}

	if !strings.Contains(rawURL, "://") {
		return nil, fmt.Errorf("invalid URL: missing scheme")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	proto := parsed.Scheme
	if proto != "vless" && proto != "trojan" {
		return nil, fmt.Errorf("unsupported protocol: %s (only vless/trojan/vmess)", proto)
	}

	cfg := &ProxyConfig{
		Protocol:   proto,
		Encryption: "none",
		Security:   "none",
		Network:    "tcp",
		// Path is intentionally left empty (not defaulted to "/"): an empty Path
		// means "the URL carried no path", which lets GenerateShareURL omit it,
		// while a bare "/" supplied in the URL is a real value that must survive
		// the round trip. The two use sites (ws/httpupgrade) default empty->"/".
		RawURL: rawURL,
	}

	if parsed.User != nil {
		cfg.UUID = parsed.User.Username()
	}

	cfg.Address = parsed.Hostname()
	if cfg.Address == "" {
		return nil, fmt.Errorf("empty address")
	}
	if portStr := parsed.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid port: %s", portStr)
		}
		cfg.Port = port
	}

	if cfg.Port == 0 {
		cfg.Port = 443
	}

	q := parsed.Query()
	if v := q.Get("encryption"); v != "" {
		cfg.Encryption = v
	}
	if v := q.Get("security"); v != "" {
		cfg.Security = v
	}
	if v := q.Get("sni"); v != "" {
		cfg.SNI = v
	}
	if v := q.Get("fp"); v != "" {
		cfg.Fingerprint = v
	}
	if v := q.Get("type"); v != "" {
		cfg.Network = v
	}
	if v := q.Get("host"); v != "" {
		cfg.Host = v
	}
	if v := q.Get("path"); v != "" {
		cfg.Path = v
	}
	if v := q.Get("packetEncoding"); v != "" {
		cfg.PacketEncoding = v
	}
	if v := q.Get("flow"); v != "" {
		cfg.Flow = v
	}
	if v := q.Get("pbk"); v != "" {
		cfg.PublicKey = v
	}
	if v := q.Get("sid"); v != "" {
		cfg.ShortId = v
	}
	if v := q.Get("spx"); v != "" {
		cfg.SpiderX = v
	}
	if v := q.Get("allowInsecure"); parseInsecureFlag(v) {
		cfg.AllowInsecure = true
	}
	if v := q.Get("allow_insecure"); parseInsecureFlag(v) {
		cfg.AllowInsecure = true
	}
	if v := q.Get("insecure"); parseInsecureFlag(v) {
		cfg.AllowInsecure = true
	}
	if v := q.Get("alpn"); v != "" {
		cfg.ALPN = v
	}
	if v := q.Get("headerType"); v != "" {
		cfg.HeaderType = v
	}
	if v := q.Get("mode"); v != "" {
		cfg.Mode = v
	}
	if v := q.Get("serviceName"); v != "" {
		cfg.ServiceName = v
	}
	// WebSocket early-data: accept both the verbose form and the `ed`/`eh`
	// shorthands some panels emit.
	if v := q.Get("max_early_data"); v != "" {
		cfg.MaxEarlyData, _ = strconv.Atoi(v)
	}
	if v := q.Get("ed"); v != "" && cfg.MaxEarlyData == 0 {
		cfg.MaxEarlyData, _ = strconv.Atoi(v)
	}
	if v := q.Get("early_data_header_name"); v != "" {
		cfg.EarlyDataHeaderName = v
	}
	if v := q.Get("eh"); v != "" && cfg.EarlyDataHeaderName == "" {
		cfg.EarlyDataHeaderName = v
	}

	if cfg.SNI == "" {
		cfg.SNI = cfg.Host
	}
	if cfg.SNI == "" {
		cfg.SNI = cfg.Address
	}

	// Some gRPC panels carry the service name in the path field; derive it only
	// for gRPC so ws/httpupgrade configs don't pick up a spurious serviceName.
	if cfg.Network == "grpc" && cfg.ServiceName == "" && cfg.Path != "" && cfg.Path != "/" {
		cfg.ServiceName = strings.TrimPrefix(cfg.Path, "/")
	}

	if frag := parsed.Fragment; frag != "" {
		cfg.Remark = frag
	}

	return cfg, nil
}

func (c *ProxyConfig) WithEndpoint(endpoint string) *ProxyConfig {
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return c
	}
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = c.Port
	}
	clone := *c
	// We're about to repoint Address at a raw scan IP. For CDN-fronted configs
	// the original hostname doubles as the implicit TLS SNI (and, via the WS/
	// httpupgrade Host fallback, the routing host) whenever those aren't set
	// explicitly. Once Address is an IP, xray would send SNI:<ip> — which
	// Cloudflare can't route — failing every Phase-2 validation and producing a
	// dead exported config. Pin the original hostname into SNI first so both the
	// validation tunnel and the emitted share URL keep working. Mirrors the WS
	// Host->SNI fallback in buildStreamSettings.
	if clone.SNI == "" && net.ParseIP(c.Address) == nil {
		clone.SNI = c.Address
	}
	clone.Address = host
	clone.Port = port
	return &clone
}

func (c *ProxyConfig) GenerateShareURL() string {
	if c.Protocol == "vmess" {
		return c.generateVMessShareURL()
	}

	addr := c.Address
	if strings.Contains(addr, ":") {
		addr = "[" + addr + "]"
	}

	hostPort := fmt.Sprintf("%s:%d", addr, c.Port)

	u := url.URL{
		Scheme:   c.Protocol,
		User:     url.User(c.UUID),
		Host:     hostPort,
		Fragment: c.Remark,
	}

	q := url.Values{}
	// VLESS share links must always carry an encryption field (always "none"
	// today); strict clients reject a vless URL that omits it. Other protocols
	// only emit an explicit, non-default value.
	if c.Protocol == "vless" {
		enc := c.Encryption
		if enc == "" {
			enc = "none"
		}
		q.Set("encryption", enc)
	} else if c.Encryption != "" && c.Encryption != "none" {
		q.Set("encryption", c.Encryption)
	}
	if c.Security != "" && c.Security != "none" {
		q.Set("security", c.Security)
	}
	if c.SNI != "" {
		q.Set("sni", c.SNI)
	}
	if c.Fingerprint != "" {
		q.Set("fp", c.Fingerprint)
	}
	if c.Network != "" && c.Network != "tcp" {
		q.Set("type", c.Network)
	}
	if c.Host != "" {
		q.Set("host", c.Host)
	}
	if c.Path != "" {
		q.Set("path", c.Path)
	}
	if c.Network == "ws" && c.Host == "" {
		q.Set("host", c.SNI)
	}
	if c.PacketEncoding != "" {
		q.Set("packetEncoding", c.PacketEncoding)
	}
	if c.Flow != "" {
		q.Set("flow", c.Flow)
	}
	if c.PublicKey != "" {
		q.Set("pbk", c.PublicKey)
	}
	if c.ShortId != "" {
		q.Set("sid", c.ShortId)
	}
	if c.SpiderX != "" {
		q.Set("spx", c.SpiderX)
	}
	if c.AllowInsecure {
		q.Set("allowInsecure", "1")
	}
	if c.ALPN != "" {
		q.Set("alpn", c.ALPN)
	}
	if c.HeaderType != "" {
		q.Set("headerType", c.HeaderType)
	}
	if c.Mode != "" {
		q.Set("mode", c.Mode)
	}
	if c.ServiceName != "" {
		q.Set("serviceName", c.ServiceName)
	}
	if c.MaxEarlyData > 0 {
		q.Set("max_early_data", strconv.Itoa(c.MaxEarlyData))
	}
	if c.EarlyDataHeaderName != "" {
		q.Set("early_data_header_name", c.EarlyDataHeaderName)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (c *ProxyConfig) generateVMessShareURL() string {
	vmess := map[string]interface{}{
		"v":    "2",
		"ps":   c.Remark,
		"add":  c.Address,
		"port": fmt.Sprintf("%d", c.Port),
		"id":   c.UUID,
		"aid":  "0",
		"scy":  c.Encryption,
		"net":  c.Network,
		"type": c.HeaderType,
		"host": c.Host,
		"path": c.Path,
	}

	if c.Security == "tls" {
		vmess["tls"] = "tls"
	}
	if c.SNI != "" {
		vmess["sni"] = c.SNI
	}
	if c.ALPN != "" {
		vmess["alpn"] = c.ALPN
	}
	if c.Fingerprint != "" {
		vmess["fp"] = c.Fingerprint
	}
	if c.AllowInsecure {
		vmess["allowInsecure"] = "1"
	}

	b, _ := json.Marshal(vmess)
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

type VLESSOutboundSettings struct {
	VNext []VNextServer `json:"vnext"`
}

type VNextServer struct {
	Address string      `json:"address"`
	Port    int         `json:"port"`
	Users   []VLessUser `json:"users"`
}

type VLessUser struct {
	ID         string `json:"id"`
	Encryption string `json:"encryption"`
	Flow       string `json:"flow,omitempty"`
}

type TrojanOutboundSettings struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	Level    int    `json:"level,omitempty"`
}

type VMessOutboundSettings struct {
	Address     string `json:"address"`
	Port        int    `json:"port"`
	ID          string `json:"id"`
	Security    string `json:"security"`
	Level       int    `json:"level,omitempty"`
	Experiments string `json:"experiments,omitempty"`
}

type StreamSettings struct {
	Network             string          `json:"network"`
	Security            string          `json:"security"`
	TLSSettings         json.RawMessage `json:"tlsSettings,omitempty"`
	RealitySettings     json.RawMessage `json:"realitySettings,omitempty"`
	WSSettings          json.RawMessage `json:"wsSettings,omitempty"`
	GRPCSettings        json.RawMessage `json:"grpcSettings,omitempty"`
	KCPSettings         json.RawMessage `json:"kcpSettings,omitempty"`
	RawSettings         json.RawMessage `json:"rawSettings,omitempty"`
	HTTPUpgradeSettings json.RawMessage `json:"httpupgradeSettings,omitempty"`
}

type TLSSettings struct {
	ServerName    string   `json:"serverName"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	AllowInsecure bool     `json:"allowInsecure"`
	ALPN          []string `json:"alpn,omitempty"`
}

type RealitySettings struct {
	ServerName  string `json:"serverName"`
	Fingerprint string `json:"fingerprint,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	ShortId     string `json:"shortId,omitempty"`
	SpiderX     string `json:"spiderX,omitempty"`
}

type WSSettings struct {
	Path                string            `json:"path"`
	Headers             map[string]string `json:"headers,omitempty"`
	MaxEarlyData        int               `json:"maxEarlyData,omitempty"`
	EarlyDataHeaderName string            `json:"earlyDataHeaderName,omitempty"`
}

type GRPCSettings struct {
	ServiceName string `json:"serviceName"`
	MultiMode   bool   `json:"multiMode,omitempty"`
}

type KCPSettings struct {
	MTU              int  `json:"mtu,omitempty"`
	TTI              int  `json:"tti,omitempty"`
	UplinkCapacity   int  `json:"uplinkCapacity,omitempty"`
	DownlinkCapacity int  `json:"downlinkCapacity,omitempty"`
	Congestion       bool `json:"congestion,omitempty"`
	ReadBufferSize   int  `json:"readBufferSize,omitempty"`
	WriteBufferSize  int  `json:"writeBufferSize,omitempty"`
}

type HTTPUpgradeSettings struct {
	Path    string            `json:"path"`
	Host    string            `json:"host,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type RawSettings struct {
	Header json.RawMessage `json:"header,omitempty"`
}

func (c *ProxyConfig) buildOutboundSettings() json.RawMessage {
	switch c.Protocol {
	case "vless":
		settings, _ := json.Marshal(VLESSOutboundSettings{
			VNext: []VNextServer{
				{
					Address: c.Address,
					Port:    c.Port,
					Users: []VLessUser{
						{
							ID:         c.UUID,
							Encryption: c.Encryption,
							Flow:       c.Flow,
						},
					},
				},
			},
		})
		return settings
	case "trojan":
		settings, _ := json.Marshal(TrojanOutboundSettings{
			Address:  c.Address,
			Port:     c.Port,
			Password: c.UUID,
		})
		return settings
	case "vmess":
		sec := c.Encryption
		if sec == "" || sec == "none" {
			sec = "auto"
		}
		settings, _ := json.Marshal(VMessOutboundSettings{
			Address:  c.Address,
			Port:     c.Port,
			ID:       c.UUID,
			Security: sec,
		})
		return settings
	default:
		settings, _ := json.Marshal(VLESSOutboundSettings{
			VNext: []VNextServer{
				{
					Address: c.Address,
					Port:    c.Port,
					Users: []VLessUser{
						{
							ID:         c.UUID,
							Encryption: c.Encryption,
						},
					},
				},
			},
		})
		return settings
	}
}

func (c *ProxyConfig) buildStreamSettings() json.RawMessage {
	hasSecurity := c.Security == "tls" || c.Security == "xtls" || c.Security == "reality"
	hasNetwork := c.Network != "" && c.Network != "tcp" && c.Network != "raw"
	// A tcp/raw transport with an "http" header obfuscation still needs stream
	// settings (RawSettings) even with no TLS — otherwise validation runs plain
	// TCP and silently disagrees with the exported config.
	hasRawHeader := c.HeaderType == "http" && (c.Network == "" || c.Network == "tcp" || c.Network == "raw")

	if !hasSecurity && !hasNetwork && !hasRawHeader {
		return nil
	}

	ss := StreamSettings{
		Network:  c.Network,
		Security: c.Security,
	}

	if c.Security == "tls" || c.Security == "xtls" {
		tlsCfg := TLSSettings{
			ServerName:    c.SNI,
			Fingerprint:   c.Fingerprint,
			AllowInsecure: c.AllowInsecure,
		}
		if c.Fingerprint == "" {
			tlsCfg.Fingerprint = "random"
		}
		if c.ALPN != "" {
			tlsCfg.ALPN = strings.Split(c.ALPN, ",")
		}
		tlsJSON, _ := json.Marshal(tlsCfg)
		ss.TLSSettings = tlsJSON
	}

	if c.Security == "reality" {
		realityCfg := RealitySettings{
			ServerName:  c.SNI,
			Fingerprint: c.Fingerprint,
			PublicKey:   c.PublicKey,
			ShortId:     c.ShortId,
			SpiderX:     c.SpiderX,
		}
		if c.Fingerprint == "" {
			realityCfg.Fingerprint = "random"
		}
		if realityCfg.ShortId == "" {
			realityCfg.ShortId = "0"
		}
		realityJSON, _ := json.Marshal(realityCfg)
		ss.RealitySettings = realityJSON
	}

	switch c.Network {
	case "ws", "websocket":
		// WSSettings.Path has no omitempty, so an empty Path would marshal to
		// "path":"" — a literal empty path, not the "/" xray clients assume.
		wsPath := c.Path
		if wsPath == "" {
			wsPath = "/"
		}
		wsCfg := WSSettings{
			Path:                wsPath,
			MaxEarlyData:        c.MaxEarlyData,
			EarlyDataHeaderName: c.EarlyDataHeaderName,
		}
		// Cloudflare routes CDN-fronted configs by the WS Host header. Real
		// clients (and GenerateShareURL) fall back to the SNI when no explicit
		// host= is present; mirror that here so validation hits the same origin
		// the exported config would — otherwise xray sends Host:<edge-IP> and
		// Cloudflare can't route, failing every Phase-2 check.
		wsHost := c.Host
		if wsHost == "" {
			wsHost = c.SNI
		}
		if wsHost != "" {
			wsCfg.Headers = map[string]string{
				"Host": wsHost,
			}
		}
		wsJSON, _ := json.Marshal(wsCfg)
		ss.WSSettings = wsJSON

	case "grpc":
		grpcCfg := GRPCSettings{
			ServiceName: c.ServiceName,
			MultiMode:   c.Mode == "multi",
		}
		if grpcCfg.ServiceName == "" {
			grpcCfg.ServiceName = strings.TrimPrefix(c.Path, "/")
		}
		grpcJSON, _ := json.Marshal(grpcCfg)
		ss.GRPCSettings = grpcJSON

	case "kcp", "mkcp":
		kcpCfg := KCPSettings{}
		kcpJSON, _ := json.Marshal(kcpCfg)
		ss.KCPSettings = kcpJSON

	case "httpupgrade":
		// Same Host->SNI fallback as ws: CF needs the fronting host to route.
		hugHost := c.Host
		if hugHost == "" {
			hugHost = c.SNI
		}
		hugPath := c.Path
		if hugPath == "" {
			hugPath = "/"
		}
		hugCfg := HTTPUpgradeSettings{
			Path: hugPath,
			Host: hugHost,
		}
		hugJSON, _ := json.Marshal(hugCfg)
		ss.HTTPUpgradeSettings = hugJSON

	case "tcp", "raw", "":
		if c.HeaderType == "http" {
			rawCfg := RawSettings{
				Header: json.RawMessage(`{"type": "http"}`),
			}
			rawJSON, _ := json.Marshal(rawCfg)
			ss.RawSettings = rawJSON
		}
	}

	ssJSON, _ := json.Marshal(ss)
	return ssJSON
}

// BuildXrayJSONBatch builds a SINGLE xray config that proxies a whole batch of
// endpoints at once: one SOCKS inbound per endpoint (ports basePort, basePort+1,
// …), one outbound per endpoint (the shared proxy config repointed at that
// endpoint's IP via WithEndpoint), and a routing rule wiring each inbound to its
// own outbound. Phase-2 previously spawned one xray PROCESS per endpoint — the
// config-write + exec + port-wait of that spawn was the dominant cost. Collapsing
// a batch into one process keeps full per-endpoint isolation (separate ports,
// separate outbounds, independent 204 checks) while cutting process spawns by the
// batch factor. Returns the config path and the per-endpoint SOCKS ports (aligned
// to the endpoints slice).
func (c *ProxyConfig) BuildXrayJSONBatch(endpoints []string, basePort int) (configPath string, ports []int, err error) {
	if c.UUID == "" {
		return "", nil, fmt.Errorf("empty UUID/password")
	}

	dirName := filepath.Join("_xray_clean", fmt.Sprintf("batch_%d", basePort))
	return buildBatchXrayConfig(endpoints, basePort, dirName,
		func(ep, outTag string) (OutboundConfig, error) {
			// Each endpoint gets the shared proxy config repointed at its IP, with the
			// original hostname pinned into SNI (WithEndpoint handles that) so CDN
			// routing keeps working — same correctness invariant as the single path.
			epCfg := c.WithEndpoint(ep)
			ob := OutboundConfig{
				Tag:      outTag,
				Protocol: epCfg.Protocol,
				Settings: epCfg.buildOutboundSettings(),
			}
			if ss := epCfg.buildStreamSettings(); ss != nil {
				ob.StreamSettings = ss
			}
			return ob, nil
		})
}
