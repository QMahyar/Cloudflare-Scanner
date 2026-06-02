package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
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
		if !strings.HasPrefix(token, "vless://") && !strings.HasPrefix(token, "trojan://") {
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
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	return string(body), nil
}

func ParseSubscription(content string) ([]*ProxyConfig, error) {
	text := content
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(content))
	}
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
		if !strings.HasPrefix(line, "vless://") && !strings.HasPrefix(line, "trojan://") {
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
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		c.Protocol, c.UUID, c.Encryption, c.Security, c.SNI,
		c.Fingerprint, c.Network, c.Host, c.Path, c.PacketEncoding,
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

func GenerateReplacedConfigs(configs []*ProxyConfig, endpoints []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(configs)*len(endpoints))

	for _, cfg := range configs {
		for _, epRaw := range endpoints {
			ep := strings.TrimSpace(epRaw)
			if ep == "" {
				continue
			}
			clone := *cfg
			if host, portStr, err := net.SplitHostPort(ep); err == nil {
				clone.Address = host
				if p, _ := strconv.Atoi(portStr); p > 0 {
					clone.Port = p
				}
			}
			if clone.Remark != "" {
				clone.Remark = cfg.Remark + " @ " + ep
			} else {
				clone.Remark = ep
			}
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
