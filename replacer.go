package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func ParseRawConfigs(rawText string) []*ProxyConfig {
	replacer := strings.NewReplacer(",", " ", ";", " ", "|", " ")
	rawText = replacer.Replace(rawText)
	tokens := strings.Fields(rawText)
	configs := make([]*ProxyConfig, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if !strings.HasPrefix(token, "vless://") && !strings.HasPrefix(token, "trojan://") && !strings.HasPrefix(token, "vmess://") {
			continue
		}
		cfg, err := ParseProxyURL(token)
		if err != nil {
			continue
		}
		configs = append(configs, cfg)
	}
	return configs
}

func FetchSubscription(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("only http/https subscription URLs are allowed")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	return string(body), nil
}

func ParseSubscription(content string) ([]*ProxyConfig, error) {
	text := content
	decoded, err := decodeBase64Loose(content)
	if err == nil {
		text = string(decoded)
	}

	lines := strings.Split(strings.TrimSpace(text), "\n")
	configs := make([]*ProxyConfig, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "vless://") && !strings.HasPrefix(line, "trojan://") && !strings.HasPrefix(line, "vmess://") {
			continue
		}
		cfg, err := ParseProxyURL(line)
		if err != nil {
			continue
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

func ConfigFingerprint(c *ProxyConfig) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%t|%s|%s|%s|%s|%d|%s",
		c.Protocol, c.UUID, c.Encryption, c.Security, c.SNI,
		c.Fingerprint, c.Network, c.Host, c.Path, c.PacketEncoding,
		c.Flow, c.PublicKey, c.ShortId, c.SpiderX, c.AllowInsecure,
		c.ALPN, c.HeaderType, c.Mode, c.ServiceName,
		c.MaxEarlyData, c.EarlyDataHeaderName,
	)
}

func DeduplicateConfigs(configs []*ProxyConfig) []*ProxyConfig {
	seen := make(map[string]bool)
	result := make([]*ProxyConfig, 0, len(configs))

	for _, cfg := range configs {
		fp := ConfigFingerprint(cfg)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		result = append(result, cfg)
	}

	return result
}

// applyNameTemplate fills a remark template. Supported placeholders:
//   {remark} original remark   {ip}/{host} endpoint host   {port} endpoint port
//   {ep} host:port   {proto} protocol   {n} 1-based running index
// An empty template falls back to the legacy "<remark> @ <host:port>" behavior.
func applyNameTemplate(tmpl string, cfg *ProxyConfig, host string, port, n int) string {
	ep := net.JoinHostPort(host, strconv.Itoa(port))
	if strings.TrimSpace(tmpl) == "" {
		if cfg.Remark != "" {
			return cfg.Remark + " @ " + ep
		}
		return ep
	}
	r := strings.NewReplacer(
		"{remark}", cfg.Remark,
		"{ip}", host,
		"{host}", host,
		"{port}", strconv.Itoa(port),
		"{ep}", ep,
		"{proto}", cfg.Protocol,
		"{n}", strconv.Itoa(n),
	)
	return r.Replace(tmpl)
}

func GenerateReplacedConfigs(configs []*ProxyConfig, endpoints []string) []string {
	return GenerateReplacedConfigsNamed(configs, endpoints, "")
}

func GenerateReplacedConfigsNamed(configs []*ProxyConfig, endpoints []string, nameTemplate string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(configs)*len(endpoints))

	n := 0
	for _, cfg := range configs {
		for _, epRaw := range endpoints {
			ep := strings.TrimSpace(epRaw)
			if ep == "" {
				continue
			}
			host, portStr, err := net.SplitHostPort(ep)
			if err != nil {
				continue
			}
			p, _ := strconv.Atoi(portStr)
			if p <= 0 || p > 65535 {
				continue
			}
			n++
			clone := *cfg
			clone.Address = host
			clone.Port = p
			clone.Remark = applyNameTemplate(nameTemplate, cfg, host, p, n)
			u := clone.GenerateShareURL()
			if seen[u] {
				continue
			}
			seen[u] = true
			result = append(result, u)
		}
	}

	return result
}
