package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
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
}

func parseInsecureFlag(v string) bool {
	return v == "1" || strings.EqualFold(v, "true")
}

func ParseVMessURL(rawURL string) (*ProxyConfig, error) {
	if !strings.HasPrefix(rawURL, "vmess://") {
		return nil, fmt.Errorf("not a vmess URL")
	}

	b64 := strings.TrimPrefix(rawURL, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(b64)
	}
	if err != nil {
		return nil, fmt.Errorf("vmess base64 decode: %w", err)
	}

	var vmess struct {
		V            string `json:"v"`
		Remark       string `json:"ps"`
		Address      string `json:"add"`
		Port         string `json:"port"`
		ID           string `json:"id"`
		Aid          string `json:"aid"`
		Security     string `json:"scy"`
		Network      string `json:"net"`
		Type         string `json:"type"`
		Host         string `json:"host"`
		Path         string `json:"path"`
		TLS          string `json:"tls"`
		SNI          string `json:"sni"`
		ALPN         string `json:"alpn"`
		Fingerprint  string `json:"fp"`
		AllowInsecure string `json:"allowInsecure"`
		Flow         string `json:"flow"`
	}

	if err := json.Unmarshal(decoded, &vmess); err != nil {
		return nil, fmt.Errorf("vmess json decode: %w", err)
	}

	if vmess.Address == "" {
		return nil, fmt.Errorf("vmess: empty address")
	}

	port, _ := strconv.Atoi(vmess.Port)
	if port == 0 {
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
		Protocol:      "vmess",
		UUID:          vmess.ID,
		Address:       vmess.Address,
		Port:          port,
		Encryption:    vmess.Security,
		Security:      security,
		SNI:           sni,
		Fingerprint:   vmess.Fingerprint,
		Network:       netType,
		Host:          vmess.Host,
		Path:          path,
		Flow:          vmess.Flow,
		Remark:        vmess.Remark,
		HeaderType:    vmess.Type,
		ALPN:          vmess.ALPN,
		RawURL:        rawURL,
	}

	if parseInsecureFlag(vmess.AllowInsecure) {
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
		Path:       "/",
		RawURL:     rawURL,
	}

	if parsed.User != nil {
		cfg.UUID = parsed.User.Username()
	}

	cfg.Address = parsed.Hostname()
	if portStr := parsed.Port(); portStr != "" {
		cfg.Port, _ = strconv.Atoi(portStr)
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

	if cfg.SNI == "" {
		cfg.SNI = cfg.Host
	}
	if cfg.SNI == "" {
		cfg.SNI = cfg.Address
	}

	if cfg.ServiceName == "" && cfg.Path != "" && cfg.Path != "/" {
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
	if c.Encryption != "" && c.Encryption != "none" {
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
	if c.Path != "" && c.Path != "/" {
		q.Set("path", c.Path)
	}
	if c.Network == "ws" && c.Host == "" {
		q.Set("host", c.SNI)
	}
	if c.PacketEncoding == "xudp" {
		q.Set("packetEncoding", "xudp")
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

	u.RawQuery = q.Encode()
	return u.String()
}

func (c *ProxyConfig) generateVMessShareURL() string {
	vmess := map[string]interface{}{
		"v":   "2",
		"ps":  c.Remark,
		"add": c.Address,
		"port": fmt.Sprintf("%d", c.Port),
		"id":  c.UUID,
		"aid": "0",
		"scy": c.Encryption,
		"net": c.Network,
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
	Address string     `json:"address"`
	Port    int        `json:"port"`
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
	Network            string           `json:"network"`
	Security           string           `json:"security"`
	TLSSettings        json.RawMessage  `json:"tlsSettings,omitempty"`
	RealitySettings    json.RawMessage  `json:"realitySettings,omitempty"`
	WSSettings         json.RawMessage  `json:"wsSettings,omitempty"`
	GRPCSettings       json.RawMessage  `json:"grpcSettings,omitempty"`
	KCPSettings        json.RawMessage  `json:"kcpSettings,omitempty"`
	RawSettings        json.RawMessage  `json:"rawSettings,omitempty"`
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
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
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

	if !hasSecurity && !hasNetwork {
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
		wsCfg := WSSettings{
			Path: c.Path,
		}
		if c.Host != "" {
			wsCfg.Headers = map[string]string{
				"Host": c.Host,
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
		hugCfg := HTTPUpgradeSettings{
			Path: c.Path,
			Host: c.Host,
		}
		hugJSON, _ := json.Marshal(hugCfg)
		ss.HTTPUpgradeSettings = hugJSON

	case "tcp", "raw":
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

func (c *ProxyConfig) BuildXrayJSON(endpoint string, socksPort int) (configPath string, err error) {
	cfg := &ProxyConfig{}
	if endpoint != "" {
		cfg = c.WithEndpoint(endpoint)
	} else {
		cfg = c
	}

	if cfg.Address == "" {
		return "", fmt.Errorf("empty address")
	}
	if cfg.Port == 0 {
		return "", fmt.Errorf("invalid port: 0")
	}
	if cfg.UUID == "" {
		return "", fmt.Errorf("empty UUID/password")
	}

	tag := fmt.Sprintf("vless_%d", socksPort)
	configDir := filepath.Join(os.TempDir(), "_xray_clean", tag)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create work dir: %w", err)
	}

	logFile := filepath.Join(configDir, "xray.log")
	os.WriteFile(logFile, []byte{}, 0644)

	outboundSettings := cfg.buildOutboundSettings()

	socksSettings, _ := json.Marshal(map[string]interface{}{
		"auth": "noauth",
		"udp":  true,
	})

	streamSettings := cfg.buildStreamSettings()

	var mux json.RawMessage
	if cfg.PacketEncoding != "" {
		muxRaw, _ := json.Marshal(map[string]interface{}{
			"enabled":     true,
			"concurrency": 8,
		})
		mux = muxRaw
	}

	xcfg := XrayConfig{
		Log: &LogConfig{
			Access:   logFile,
			Error:    logFile,
			Loglevel: "warning",
		},
		Inbounds: []InboundConfig{
			{
				Tag:      "socks-in",
				Port:     socksPort,
				Listen:   "127.0.0.1",
				Protocol: "socks",
				Settings: socksSettings,
			},
		},
		Outbounds: []OutboundConfig{
			{
				Tag:      "proxy",
				Protocol: cfg.Protocol,
				Settings: outboundSettings,
			},
			{
				Tag:      "direct",
				Protocol: "freedom",
			},
			{
				Tag:      "block",
				Protocol: "blackhole",
			},
		},
		Routing: &RoutingConfig{
			DomainStrategy: "AsIs",
			Rules: []RoutingRule{
				{
					Type:        "field",
					InboundTag:  []string{"socks-in"},
					OutboundTag: "proxy",
				},
			},
		},
	}

	if streamSettings != nil {
		xcfg.Outbounds[0].StreamSettings = streamSettings
	}
	if mux != nil {
		xcfg.Outbounds[0].Mux = mux
	}

	configJSON, err := json.MarshalIndent(xcfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	configPath = filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return configPath, nil
}
