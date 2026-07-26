package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type WarpConfig struct {
	PrivateKey string
	Addresses  []string
	PublicKey  string
	Reserved   []int
	MTU        int
	// Endpoint is optional — filled from [Peer] Endpoint or the host of a
	// wg:// / wireguard:// share URI. The endpoint scanner still generates its
	// own candidates; this is kept so apply/replacer flows can reuse it.
	Endpoint string
	// AmneziaWG / Hogwarts junk parameters (parsed for round-trip + native
	// probe). Zero values mean "not set".
	Jc   int
	Jmin int
	Jmax int
	H1   int
	H2   int
	H3   int
	H4   int
}

// ParseWarpConfig reads a WireGuard/WARP .conf from disk and returns the
// credentials needed for the native handshake and xray noise path.
func ParseWarpConfig(path string) (*WarpConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open config: %w", err)
	}
	return ParseWarpConfigText(string(data))
}

// ParseWarpConfigText accepts any of the formats Throne / v2rayN / Amnezia /
// official WARP clients emit:
//
//   - Standard INI: [Interface] / [Peer] blocks (PrivateKey, Address, PublicKey, …)
//   - Share URIs:   wg://host:port?private_key=…&public_key=…&local_address=…
//   - Alias scheme: wireguard://… (same query keys as wg://)
//
// local_address may use "-" or "," as the dual-stack separator (Throne uses
// "-"); reserved may be "255,212,70" or "255-212-70".
func ParseWarpConfigText(text string) (*WarpConfig, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty config")
	}

	// Strip a UTF-8 BOM some Windows editors prepend.
	text = strings.TrimPrefix(text, "\ufeff")

	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "wg://") || strings.HasPrefix(lower, "wireguard://") {
		// Take the first line only — paste boxes sometimes append notes.
		line := text
		if i := strings.IndexAny(line, "\r\n"); i >= 0 {
			line = line[:i]
		}
		return parseWarpURI(strings.TrimSpace(line))
	}
	return parseWarpINI(text)
}

func parseWarpURI(raw string) (*WarpConfig, error) {
	// Throne / v2rayN share links often leave base64 keys only partially URL-encoded
	// (literal "+" and "/" in public_key). Go's url.Query() treats "+" as space
	// (application/x-www-form-urlencoded), which corrupts WireGuard keys into
	// "bmXOC F1…" and later fails the native handshake with "invalid public key".
	// Parse host with url.Parse, then decode the query with PathUnescape so "+"
	// stays "+" while %3D / %2B still decode correctly.
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse warp URI: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "wg" && scheme != "wireguard" {
		return nil, fmt.Errorf("unsupported warp URI scheme: %s", u.Scheme)
	}

	q, err := parseQueryKeepPlus(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("parse warp URI query: %w", err)
	}

	cfg := &WarpConfig{
		Reserved: []int{0, 0, 0},
		MTU:      1280,
	}

	// Accept both snake_case (Throne / sing-box style) and the camelCase some
	// exporters use.
	cfg.PrivateKey = firstQuery(q, "private_key", "privateKey", "secretKey", "secret_key")
	cfg.PublicKey = firstQuery(q, "public_key", "publicKey", "peer_public_key", "peerPublicKey")
	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("warp URI missing private_key")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("warp URI missing public_key")
	}

	// local_address / address: "v4/32-v6/128" (Throne) or "v4/32,v6/128".
	addrRaw := firstQuery(q, "local_address", "localAddress", "address", "addresses")
	if addrRaw == "" {
		return nil, fmt.Errorf("warp URI missing local_address")
	}
	for _, a := range splitAddressList(addrRaw) {
		cfg.Addresses = append(cfg.Addresses, normalizeWarpAddress(a))
	}
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("warp URI has empty local_address")
	}

	if mtu := firstQuery(q, "mtu", "MTU"); mtu != "" {
		if n, err := strconv.Atoi(mtu); err == nil && n > 0 {
			cfg.MTU = n
		}
	}

	if res := firstQuery(q, "reserved", "Reserved"); res != "" {
		parsed, err := parseReservedList(res)
		if err != nil {
			return nil, err
		}
		cfg.Reserved = parsed
	}

	// Endpoint from the URI host:port (engage.cloudflareclient.com:2408).
	if host := u.Hostname(); host != "" {
		port := u.Port()
		if port == "" {
			port = "2408"
		}
		cfg.Endpoint = host + ":" + port
	}

	for len(cfg.Reserved) < 3 {
		cfg.Reserved = append(cfg.Reserved, 0)
	}
	cfg.Reserved = cfg.Reserved[:3]

	if err := cfg.validateKeys(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// parseQueryKeepPlus is like url.ParseQuery but uses PathUnescape so a literal
// "+" in base64 keys is preserved (QueryUnescape turns "+" into a space).
func parseQueryKeepPlus(rawQuery string) (url.Values, error) {
	q := make(url.Values)
	if rawQuery == "" {
		return q, nil
	}
	for _, pair := range strings.Split(rawQuery, "&") {
		if pair == "" {
			continue
		}
		key, val, _ := strings.Cut(pair, "=")
		k, err := url.PathUnescape(key)
		if err != nil {
			return nil, err
		}
		v, err := url.PathUnescape(val)
		if err != nil {
			return nil, err
		}
		q.Add(k, v)
	}
	return q, nil
}

// validateKeys checks WireGuard Curve25519 keys are 32-byte base64 (standard or
// raw). Rejects sample placeholders like "<your-private-key>" with a clear hint.
func (cfg *WarpConfig) validateKeys() error {
	if err := validateWGKey("private key", cfg.PrivateKey); err != nil {
		return err
	}
	if err := validateWGKey("public key", cfg.PublicKey); err != nil {
		return err
	}
	return nil
}

func validateWGKey(label, b64 string) error {
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return fmt.Errorf("missing %s", label)
	}
	low := strings.ToLower(b64)
	if strings.Contains(b64, "<") || strings.Contains(b64, ">") ||
		strings.Contains(low, "your-private") || strings.Contains(low, "your-public") ||
		strings.Contains(low, "paste") || strings.Contains(low, "placeholder") {
		return fmt.Errorf("%s looks like a placeholder — replace it with a real base64 WireGuard key (44 characters, often ending in =)", label)
	}
	// StdEncoding first (padded "…="); RawStdEncoding for unpadded exports.
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(b64, "="))
		if err != nil {
			return fmt.Errorf("invalid %s (not base64): check that + and / were not corrupted when copying the link", label)
		}
	}
	if len(raw) != 32 {
		return fmt.Errorf("invalid %s: decoded %d bytes, WireGuard keys must be 32", label, len(raw))
	}
	return nil
}

func parseWarpINI(text string) (*WarpConfig, error) {
	cfg := &WarpConfig{
		Reserved: []int{0, 0, 0},
		MTU:      1280,
	}

	var section string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		// Strip inline comments only when they look like full-line comments;
		// values themselves (e.g. I1 hex blobs) can contain "#".
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line)
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch section {
		case "[interface]":
			switch key {
			case "privatekey":
				cfg.PrivateKey = val
			case "address":
				for _, a := range splitAddressList(val) {
					cfg.Addresses = append(cfg.Addresses, normalizeWarpAddress(a))
				}
			case "reserved":
				parsed, err := parseReservedList(val)
				if err != nil {
					return nil, err
				}
				cfg.Reserved = parsed
			case "s1":
				if err := setReservedByte(cfg, 0, val, "S1"); err != nil {
					return nil, err
				}
			case "s2":
				if err := setReservedByte(cfg, 1, val, "S2"); err != nil {
					return nil, err
				}
			case "s3":
				if err := setReservedByte(cfg, 2, val, "S3"); err != nil {
					return nil, err
				}
			case "mtu":
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					cfg.MTU = n
				}
			case "jc":
				cfg.Jc, _ = strconv.Atoi(val)
			case "jmin":
				cfg.Jmin, _ = strconv.Atoi(val)
			case "jmax":
				cfg.Jmax, _ = strconv.Atoi(val)
			case "h1":
				cfg.H1, _ = strconv.Atoi(val)
			case "h2":
				cfg.H2, _ = strconv.Atoi(val)
			case "h3":
				cfg.H3, _ = strconv.Atoi(val)
			case "h4":
				cfg.H4, _ = strconv.Atoi(val)
			}
		case "[peer]":
			switch key {
			case "publickey":
				cfg.PublicKey = val
			case "endpoint":
				cfg.Endpoint = val
			case "reserved":
				// Some clients put Reserved under [Peer] instead of [Interface].
				if isDefaultReserved(cfg.Reserved) {
					parsed, err := parseReservedList(val)
					if err != nil {
						return nil, err
					}
					cfg.Reserved = parsed
				}
			}
		}
	}

	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("config missing [Interface] PrivateKey")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("config missing [Peer] PublicKey")
	}
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("config missing [Interface] Address")
	}

	for len(cfg.Reserved) < 3 {
		cfg.Reserved = append(cfg.Reserved, 0)
	}
	cfg.Reserved = cfg.Reserved[:3]

	if err := cfg.validateKeys(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func firstQuery(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

// splitAddressList splits on comma OR hyphen-between-CIDRs. Throne's wg:// URIs
// use "v4/32-v6/128"; standard .conf uses "v4/32, v6/128". A bare hyphen inside
// an IPv6 address never appears because IPv6 here is always written with
// colons, and we only split on "-" when both sides look like addresses
// (contain "." or ":").
func splitAddressList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// Normalize hyphen dual-stack separators to commas first, but only the
	// "IPv4-IPv6" form (digit/colon on both sides of a single '-').
	if strings.Contains(raw, "-") && !strings.Contains(raw, ",") {
		raw = splitHybridAddresses(raw)
	}
	var out []string
	for _, a := range strings.Split(raw, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			out = append(out, a)
		}
	}
	return out
}

// splitHybridAddresses turns "172.16.0.2/32-2606:4700:…/128" into a
// comma-separated list. Walks the string and splits on '-' only when the
// left side already looks like a finished address (has '/' or is pure IPv4).
func splitHybridAddresses(raw string) string {
	// Fast path: exactly one '-' separating two obvious addresses.
	// Find '-' that sits between an IPv4-ish token and an IPv6-ish token.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '-' {
			continue
		}
		left, right := raw[:i], raw[i+1:]
		if looksLikeAddr(left) && looksLikeAddr(right) {
			return left + "," + right
		}
	}
	return raw
}

func looksLikeAddr(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// CIDR or bare IP: must contain '.' (v4) or ':' (v6).
	return strings.Contains(s, ".") || strings.Contains(s, ":")
}

func normalizeWarpAddress(a string) string {
	a = strings.TrimSpace(a)
	if a == "" {
		return a
	}
	// WireGuard/xray expect CIDR. Many client exports (including official WARP
	// .confs and Throne URIs) omit the prefix — bare IPv4 → /32, bare IPv6 → /128.
	if !strings.Contains(a, "/") {
		if strings.Contains(a, ":") {
			a += "/128"
		} else {
			a += "/32"
		}
	}
	return a
}

// parseReservedList accepts "255,212,70", "255-212-70", or "255 212 70".
func parseReservedList(val string) ([]int, error) {
	val = strings.TrimSpace(val)
	val = strings.ReplaceAll(val, "-", ",")
	val = strings.ReplaceAll(val, " ", ",")
	var out []int
	for _, b := range strings.Split(val, ",") {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		n, err := strconv.Atoi(b)
		if err != nil || n < 0 || n > 255 {
			return nil, fmt.Errorf("Reserved byte must be 0-255, got %s", b)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return []int{0, 0, 0}, nil
	}
	return out, nil
}

func setReservedByte(cfg *WarpConfig, idx int, val, name string) error {
	for len(cfg.Reserved) <= idx {
		cfg.Reserved = append(cfg.Reserved, 0)
	}
	// Only overwrite the zero default — an explicit Reserved= line wins over S1/S2/S3.
	if cfg.Reserved[idx] != 0 {
		return nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 || n > 255 {
		return fmt.Errorf("%s must be 0-255, got %s", name, val)
	}
	cfg.Reserved[idx] = n
	return nil
}

func isDefaultReserved(r []int) bool {
	for _, b := range r {
		if b != 0 {
			return false
		}
	}
	return true
}
