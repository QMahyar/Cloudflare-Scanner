package main

import (
	"encoding/base64"
	"encoding/json"
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
