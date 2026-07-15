package main

import "testing"

func TestSummarizeWarpFailure(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			"no handshake response from peer",
			"No WireGuard handshake — UDP likely blocked/throttled by your ISP (try enabling UDP Noise)",
		},
		{
			"xray startup timeout after 6s",
			"xray didn't come up in time (slow start or crash)",
		},
		{
			"start xray: exec format error",
			"xray failed to launch (check the xray binary / config)",
		},
		{
			"config: invalid wireguard peer",
			"xray failed to launch (check the xray binary / config)",
		},
		{
			"socks connect: connection refused",
			"couldn't reach xray's local SOCKS port",
		},
		{
			"socks5 handshake failed",
			"couldn't reach xray's local SOCKS port",
		},
		{
			"i/o timeout waiting for response",
			"probe timed out — endpoint unreachable or UDP filtered",
		},
		{
			"context deadline exceeded",
			"probe timed out — endpoint unreachable or UDP filtered",
		},
		{
			"connection reset by peer",
			"connection reset/refused (DPI filtering or dead endpoint)",
		},
		{
			"forcibly closed by the remote host",
			"connection reset/refused (DPI filtering or dead endpoint)",
		},
		{
			"connection refused",
			"connection reset/refused (DPI filtering or dead endpoint)",
		},
		{
			"invalid private key material",
			"config credentials rejected (bad/expired .conf keys)",
		},
		{
			"peer public key decode failed",
			"config credentials rejected (bad/expired .conf keys)",
		},
		{
			"cancelled by user",
			"cancelled",
		},
		{
			// "i/o timeout" matches before the tcp dial prefix — document actual precedence
			"tcp dial 1.2.3.4:443: i/o timeout",
			"probe timed out — endpoint unreachable or UDP filtered",
		},
		{
			// "refused" matches before tcp dial prefix
			"tcp dial 1.2.3.4:443: connection refused",
			"connection reset/refused (DPI filtering or dead endpoint)",
		},
		{
			"tcp dial 1.2.3.4:443: no route to host",
			"TCP port closed (reachability-only check, not a working WARP endpoint)",
		},
		{
			"",
			"unknown",
		},
		{
			"some novel failure we have not classified",
			"some novel failure we have not classified",
		},
	}
	for _, c := range cases {
		if got := summarizeWarpFailure(c.in); got != c.want {
			t.Errorf("summarizeWarpFailure(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
