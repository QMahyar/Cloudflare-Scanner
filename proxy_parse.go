package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

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
