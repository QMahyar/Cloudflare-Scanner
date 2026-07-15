package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

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
