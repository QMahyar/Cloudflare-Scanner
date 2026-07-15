package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

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
