package main

import (
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
}

func ParseProxyURL(rawURL string) (*ProxyConfig, error) {
	if !strings.Contains(rawURL, "://") {
		return nil, fmt.Errorf("invalid URL: missing scheme")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	proto := parsed.Scheme
	if proto != "vless" && proto != "trojan" {
		return nil, fmt.Errorf("unsupported protocol: %s (only vless/trojan)", proto)
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

	host := parsed.Host
	if strings.Contains(host, "]:") {
		parts := strings.SplitN(host, "]:", 2)
		cfg.Address = strings.TrimPrefix(parts[0], "[")
		cfg.Port, _ = strconv.Atoi(parts[1])
	} else if strings.Contains(host, ":") {
		hostParts := strings.SplitN(host, ":", 2)
		cfg.Address = hostParts[0]
		cfg.Port, _ = strconv.Atoi(hostParts[1])
	} else {
		cfg.Address = host
		cfg.Port = 443
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

	if cfg.SNI == "" {
		cfg.SNI = cfg.Host
	}
	if cfg.SNI == "" {
		cfg.SNI = cfg.Address
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
	if c.Protocol != "vless" {
		return c.RawURL
	}

	addr := c.Address
	if strings.Contains(addr, ":") {
		addr = "[" + addr + "]"
	}

	hostPort := fmt.Sprintf("%s:%d", addr, c.Port)

	u := url.URL{
		Scheme:   "vless",
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

	u.RawQuery = q.Encode()
	return u.String()
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

type StreamSettings struct {
	Network     string           `json:"network"`
	Security    string           `json:"security"`
	TLSSettings json.RawMessage  `json:"tlsSettings,omitempty"`
	WSSettings  json.RawMessage  `json:"wsSettings,omitempty"`
	GRPCSettings json.RawMessage `json:"grpcSettings,omitempty"`
}

type TLSSettings struct {
	ServerName    string `json:"serverName"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	AllowInsecure bool   `json:"allowInsecure"`
}

type WSSettings struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
}

type GRPCSettings struct {
	ServiceName string `json:"serviceName"`
}

func (c *ProxyConfig) BuildXrayJSON(endpoint string, socksPort int) (configPath string, err error) {
	cfg := &ProxyConfig{}
	if endpoint != "" {
		cfg = c.WithEndpoint(endpoint)
	} else {
		cfg = c
	}

	tag := fmt.Sprintf("vless_%d", socksPort)
	configDir := filepath.Join(os.TempDir(), "_xray_clean", tag)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create work dir: %w", err)
	}

	logFile := filepath.Join(configDir, "xray.log")
	os.WriteFile(logFile, []byte{}, 0644)

	flow := ""
	vnextSettings := VLESSOutboundSettings{
		VNext: []VNextServer{
			{
				Address: cfg.Address,
				Port:    cfg.Port,
				Users: []VLessUser{
					{
						ID:         cfg.UUID,
						Encryption: cfg.Encryption,
						Flow:       flow,
					},
				},
			},
		},
	}

	vnextJSON, _ := json.Marshal(vnextSettings)

	socksSettings, _ := json.Marshal(map[string]interface{}{
		"auth": "noauth",
		"udp":  true,
	})

	var streamSettings json.RawMessage
	if cfg.Security == "tls" || cfg.Security == "reality" || cfg.Network != "tcp" {
		ss := StreamSettings{
			Network:  cfg.Network,
			Security: cfg.Security,
		}

		if cfg.Security == "tls" {
			tlsCfg := TLSSettings{
				ServerName:    cfg.SNI,
				Fingerprint:   cfg.Fingerprint,
				AllowInsecure: false,
			}
			if cfg.Fingerprint == "" {
				tlsCfg.Fingerprint = "random"
			}
			tlsJSON, _ := json.Marshal(tlsCfg)
			ss.TLSSettings = tlsJSON
		}

		if cfg.Network == "ws" || cfg.Network == "websocket" {
			wsCfg := WSSettings{
				Path: cfg.Path,
			}
			if cfg.Host != "" {
				wsCfg.Headers = map[string]string{
					"Host": cfg.Host,
				}
			}
			wsJSON, _ := json.Marshal(wsCfg)
			ss.WSSettings = wsJSON
		}

		if cfg.Network == "grpc" {
			grpcCfg := GRPCSettings{
				ServiceName: strings.TrimPrefix(cfg.Path, "/"),
			}
			grpcJSON, _ := json.Marshal(grpcCfg)
			ss.GRPCSettings = grpcJSON
		}

		ssJSON, _ := json.Marshal(ss)
		streamSettings = ssJSON
	}

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
				Settings: vnextJSON,
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
