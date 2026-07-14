package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func ParseRawConfigs(rawText string) []*ProxyConfig {
	replacer := strings.NewReplacer(",", " ", ";", " ", "|", " ")
	return parseConfigTokens(strings.Fields(replacer.Replace(rawText)))
}

func parseConfigTokens(tokens []string) []*ProxyConfig {
	configs := make([]*ProxyConfig, 0, len(tokens))
	for _, token := range tokens {
		if !strings.HasPrefix(token, "vless://") && !strings.HasPrefix(token, "trojan://") && !strings.HasPrefix(token, "vmess://") {
			continue
		}
		if cfg, err := ParseProxyURL(token); err == nil {
			configs = append(configs, cfg)
		}
	}
	return configs
}

func blockedSubscriptionIP(ip net.IP) bool {
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 100.64.0.0/10 CGNAT (RFC 6598) — not covered by IsPrivate().
		if ip4[0] == 100 && ip4[1]&0xc0 == 0x40 {
			return true
		}
		// 0.0.0.0/8 "this host on this network" (RFC 1122).
		if ip4[0] == 0 {
			return true
		}
	}
	return false
}

func validateSubscriptionURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("only http/https subscription URLs are allowed")
	}
	return u, nil
}

func subscriptionHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		// Control runs after DNS resolution with the concrete remote IP, immediately
		// before connect. Validating here — rather than pre-resolving in DialContext
		// and re-resolving to dial — closes the resolve-then-dial TOCTOU that a
		// rebinding DNS server could use to slip a private/loopback address past the
		// check. It also re-checks every IP the happy-eyeballs dialer tries and every
		// hop after a redirect.
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || blockedSubscriptionIP(ip) {
				return fmt.Errorf("subscription host resolves to a private or local address")
			}
			return nil
		},
	}
	transport := &http.Transport{DialContext: dialer.DialContext}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if _, err := validateSubscriptionURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

func FetchSubscription(rawURL string) (string, error) {
	u, err := validateSubscriptionURL(rawURL)
	if err != nil {
		return "", err
	}
	client := subscriptionHTTPClient()
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
	return parseConfigTokens(strings.Fields(strings.TrimSpace(text))), nil
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
//
//	{remark} original remark   {ip}/{host} endpoint host   {port} endpoint port
//	{ep} host:port   {proto} protocol   {n} 1-based running index
//
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
			// Pin the original hostname as SNI before repointing Address at a scan
			// IP, exactly like WithEndpoint does. Without this, CDN-fronted configs
			// (where SNI is empty and implicitly the Address hostname) would send
			// SNI:<scan-IP> — which Cloudflare can't route — producing dead configs.
			if clone.SNI == "" && net.ParseIP(cfg.Address) == nil {
				clone.SNI = cfg.Address
			}
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
