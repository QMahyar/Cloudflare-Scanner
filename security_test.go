package main

import (
	"net"
	"testing"
)

// blockedSubscriptionIP guards FetchSubscription against SSRF. These cases pin
// the ranges that must stay blocked (and the public ones that must stay allowed)
// so a future edit can't silently reopen the hole.
func TestBlockedSubscriptionIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},        // loopback
		{"127.5.6.7", true},        // loopback /8
		{"0.0.0.0", true},          // unspecified
		{"0.1.2.3", true},          // 0.0.0.0/8
		{"10.0.0.1", true},         // private
		{"172.16.5.5", true},       // private
		{"192.168.1.1", true},      // private
		{"169.254.1.1", true},      // link-local
		{"100.64.0.1", true},       // CGNAT
		{"100.127.255.255", true},  // CGNAT upper edge
		{"100.128.0.1", false},     // just past CGNAT — public
		{"100.63.255.255", false},  // just before CGNAT — public
		{"1.1.1.1", false},         // public
		{"104.16.5.3", false},      // public (Cloudflare)
		{"8.8.8.8", false},         // public
		{"::1", true},              // IPv6 loopback
		{"fc00::1", true},          // IPv6 ULA (private)
		{"fe80::1", true},          // IPv6 link-local
		{"::ffff:127.0.0.1", true}, // IPv4-mapped loopback
		{"2606:4700::1", false},    // public IPv6 (Cloudflare)
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := blockedSubscriptionIP(ip); got != c.blocked {
			t.Errorf("blockedSubscriptionIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
}

// isLoopbackHost is the DNS-rebinding guard: only loopback Host headers reach the
// API. A rebinding page always carries the attacker's own hostname in Host.
func TestIsLoopbackHost(t *testing.T) {
	cases := []struct {
		host string
		ok   bool
	}{
		{"", true}, // no Host (non-browser); browsers always send one
		{"127.0.0.1", true},
		{"127.0.0.1:8080", true},
		{"localhost", true},
		{"localhost:51000", true},
		{"127.9.9.9:1234", true}, // loopback /8
		{"[::1]:8080", true},
		{"::1", true},
		{"evil.com", false},
		{"evil.com:8080", false},
		{"169.254.169.254", false},  // cloud metadata
		{"192.168.1.5:8080", false}, // LAN (can't reach a loopback bind, but reject anyway)
		{"attacker.example:443", false},
	}
	for _, c := range cases {
		if got := isLoopbackHost(c.host); got != c.ok {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", c.host, got, c.ok)
		}
	}
}

// validateEndpointHostPort must reject control chars / whitespace in the host and
// bad ports — the value is written verbatim into a WireGuard .conf, so a newline
// would inject config lines.
func TestValidateEndpointHostPort(t *testing.T) {
	valid := []string{"1.2.3.4:8886", "[2606:4700::1]:443", "example.com:2053"}
	for _, ep := range valid {
		if err := validateEndpointHostPort(ep); err != nil {
			t.Errorf("expected %q valid, got %v", ep, err)
		}
	}
	invalid := []string{
		"1.2.3.4\nInjected = x:443", // newline injection
		"host\t:443",               // tab in host
		"ho st:443",                // space in host
		"1.2.3.4:0",                // port too low
		"1.2.3.4:70000",            // port too high
		"1.2.3.4:abc",              // non-numeric port
		"1.2.3.4",                  // no port
		":443",                     // empty host
	}
	for _, ep := range invalid {
		if err := validateEndpointHostPort(ep); err == nil {
			t.Errorf("expected %q rejected, got nil", ep)
		}
	}
}
