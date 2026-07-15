package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// One xray process serves a whole Phase-2 batch, so its log interleaves every
// endpoint's errors. extractXrayErrorFrom must attribute a cause to an endpoint
// only when a log line names that endpoint's IP — otherwise one IP's failure
// leaks onto a neighbor (two endpoints reporting an error that names a third IP).
func TestExtractXrayErrorScopesByIP(t *testing.T) {
	log := []byte(strings.Join([]string{
		`2026/01/01 [Warning] read tcp 10.0.0.1:5000->172.65.28.218:443: wsarecv: forcibly closed by the remote host.`,
		`2026/01/01 [Info] something harmless`,
	}, "\n"))

	// The endpoint named in the log gets its concrete cause.
	if got := extractXrayErrorFrom(log, "172.65.28.218"); got == "" || !strings.Contains(got, "forcibly closed") {
		t.Errorf("named endpoint: expected the forcibly-closed cause, got %q", got)
	}
	// A different endpoint in the same batch must NOT inherit it.
	if got := extractXrayErrorFrom(log, "172.65.252.236"); got != "" {
		t.Errorf("unnamed endpoint: expected no attribution, got %q", got)
	}
	// With no hint (single-endpoint use), the last error line still surfaces.
	if got := extractXrayErrorFrom(log, ""); !strings.Contains(got, "forcibly closed") {
		t.Errorf("no hint: expected the last error line, got %q", got)
	}
}

func TestCloudflareIPv4PoolIncludesOfficial172Range(t *testing.T) {
	ip := net.ParseIP("172.71.255.255")
	matched := false
	for _, cidr := range cfIPv4CIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err == nil && ipnet.Contains(ip) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("Cloudflare IPv4 pool misses 172.71.255.255 from official 172.64.0.0/13")
	}
}

func TestGenerateCleanIPs(t *testing.T) {
	gen := NewCleanIPGenerator()
	ips := gen.GenerateIPs(100, true, false, []int{443})

	if len(ips) != 100 {
		t.Fatalf("expected 100 IPs, got %d", len(ips))
	}

	// Verify all IPs are valid and in Cloudflare ranges
	for _, ep := range ips {
		host, _, err := net.SplitHostPort(ep)
		if err != nil {
			t.Errorf("invalid endpoint %s: %v", ep, err)
			continue
		}

		ip := net.ParseIP(host)
		if ip == nil {
			t.Errorf("invalid IP: %s", host)
			continue
		}

		if ip.To4() == nil {
			t.Errorf("expected IPv4, got IPv6: %s", host)
			continue
		}

		_ = ip
	}

	// Print some stats
	seen := make(map[string]int)
	for _, cidr := range cfIPv4CIDRs {
		seen[cidr] = 0
	}

	for _, ep := range ips {
		host, _, _ := net.SplitHostPort(ep)
		for _, cidr := range cfIPv4CIDRs {
			_, ipnet, _ := net.ParseCIDR(cidr)
			if ipnet.Contains(net.ParseIP(host)) {
				seen[cidr]++
				break
			}
		}
	}

	fmt.Println("IP distribution across CIDRs:")
	total := 0
	for _, cidr := range cfIPv4CIDRs {
		count := seen[cidr]
		if count > 0 {
			fmt.Printf("  %s: %d IPs\n", cidr, count)
			total += count
		}
	}
	fmt.Printf("Total: %d / %d\n", total, len(ips))
	if total < len(ips) {
		t.Errorf("only %d of %d IPs matched known CIDRs", total, len(ips))
	}
}

func TestWeightCalculation(t *testing.T) {
	for _, ci := range v4CIDRInfo {
		_, ipnet, err := net.ParseCIDR(ci.cidr)
		if err != nil {
			t.Errorf("invalid CIDR: %s", ci.cidr)
			continue
		}
		ones, _ := ipnet.Mask.Size()

		expectedWeight := 1
		if ones < 24 {
			expectedWeight = 1 << (24 - ones)
		}

		if ci.weight != expectedWeight {
			t.Errorf("CIDR %s: expected weight %d, got %d", ci.cidr, expectedWeight, ci.weight)
		}
	}
	fmt.Printf("Total v4 weight: %d\n", v4TotalWeight)
	fmt.Printf("Number of v4 CIDRs: %d\n", len(v4CIDRInfo))
}

func TestIPv6Generation(t *testing.T) {
	gen := NewCleanIPGenerator()
	ips := gen.GenerateIPs(10, false, true, []int{443})

	if len(ips) != 10 {
		t.Fatalf("expected 10 IPv6 IPs, got %d", len(ips))
	}

	for _, ep := range ips {
		host, _, err := net.SplitHostPort(ep)
		if err != nil {
			t.Errorf("invalid endpoint %s: %v", ep, err)
			continue
		}
		_ = host
		ip := net.ParseIP(host)
		if ip == nil {
			t.Errorf("invalid IP: %s", host)
			continue
		}
		if ip.To16() == nil || ip.To4() != nil {
			t.Errorf("expected IPv6: %s", host)
		}

		// Verify it's in one of our CIDRs
		matched := false
		for _, cidr := range cfIPv6CIDRs {
			_, ipnet, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			if ipnet.Contains(ip) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("IP %s not in any known CIDR", host)
		}
	}
}

// TestGenerateNearbyIPsSkipsNetworkBroadcast verifies that generateNearbyIPs
// never emits .0 (network) or .255 (broadcast) addresses in the /24 expansion.
func TestGenerateNearbyIPsSkipsNetworkBroadcast(t *testing.T) {
	working := []CleanIPResult{{Endpoint: "104.18.1.5:443", Success: true}}
	ips := generateNearbyIPs(working, 100, []int{443})
	if len(ips) == 0 {
		t.Fatal("expected some nearby IPs")
	}
	for _, ep := range ips {
		host, _, err := net.SplitHostPort(ep)
		if err != nil {
			continue
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.To4() == nil {
			continue
		}
		lastOctet := ip.To4()[3]
		if lastOctet == 0 {
			t.Errorf("network address .0 generated: %s", ep)
		}
		if lastOctet == 255 {
			t.Errorf("broadcast address .255 generated: %s", ep)
		}
	}
}

// TestNoiseValidateHexPrecompiled verifies that the hex noise type validates
// correctly and that the regex is only compiled once (not per-call).
func TestNoiseValidateHexPrecompiled(t *testing.T) {
	tests := []struct {
		packet string
		ok     bool
	}{
		{"deadbeef", true},
		{"0123456789abcdefABCDEF", true},
		{"xyz", false},
		{"", false},
	}
	for _, tt := range tests {
		n := NoiseConfig{Type: "hex", Packet: tt.packet, Delay: "1-5", Count: 5}
		err := n.Validate()
		if tt.ok && err != nil {
			t.Errorf("hex %q: expected valid, got %v", tt.packet, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("hex %q: expected error, got nil", tt.packet)
		}
	}
}

// TestConfigReservedByteValidation verifies that S1/S2/S3 outside 0-255 are rejected.
func TestConfigReservedByteValidation(t *testing.T) {
	tmp := t.TempDir() + "/test.conf"
	content := `[Interface]
PrivateKey = AAA=
Address = 172.16.0.2/32
S1 = 999
`
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseWarpConfig(tmp)
	if err == nil {
		t.Error("expected error for S1=999, got nil")
	}
}

// TestSanitizeScanPorts locks the clean-scan port bound: valid ports pass,
// out-of-range drop, duplicates collapse, the result never exceeds maxScanPorts,
// and an empty/all-invalid input defaults to 443. This bounds the downstream
// (v4+v6)*len(ports) endpoint allocation.
func TestSanitizeScanPorts(t *testing.T) {
	if got := sanitizeScanPorts(nil); len(got) != 1 || got[0] != 443 {
		t.Fatalf("nil input should default to [443], got %v", got)
	}
	if got := sanitizeScanPorts([]int{0, 70000, -5}); len(got) != 1 || got[0] != 443 {
		t.Fatalf("all-invalid input should default to [443], got %v", got)
	}
	if got := sanitizeScanPorts([]int{443, 443, 2053, 2053, 443}); len(got) != 2 {
		t.Fatalf("duplicates should collapse to 2, got %v", got)
	}
	// A huge duplicate-free list must be capped at maxScanPorts.
	big := make([]int, 0, 1000)
	for p := 1; p <= 1000; p++ {
		big = append(big, p)
	}
	if got := sanitizeScanPorts(big); len(got) != maxScanPorts {
		t.Fatalf("expected cap at %d, got %d", maxScanPorts, len(got))
	}
}

// TestGenerateIPsBoundedByPorts confirms the endpoint count stays IP*ports once
// the port list is sanitized — the multiplication that makes an unbounded port
// list dangerous.
func TestGenerateIPsBoundedByPorts(t *testing.T) {
	g := NewCleanIPGenerator()
	ports := sanitizeScanPorts([]int{443, 2053, 8443})
	eps := g.GenerateIPs(100, true, false, ports)
	if len(eps) > 100*len(ports) {
		t.Fatalf("endpoint count %d exceeds ipCount*ports %d", len(eps), 100*len(ports))
	}
}

func TestSummarizeFailure(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			"xray startup timeout",
			"xray didn't come up in time (slow start or crash)",
		},
		{
			"start xray: permission denied",
			"xray failed to launch (check the xray binary / config)",
		},
		{
			"xray config write failed",
			"xray failed to launch (check the xray binary / config)",
		},
		{
			"no uuid in config",
			"incomplete config (UUID/address/port missing)",
		},
		{
			"empty address field",
			"incomplete config (UUID/address/port missing)",
		},
		{
			"invalid port 0",
			"incomplete config (UUID/address/port missing)",
		},
		{
			"socks connect: dial tcp 127.0.0.1:1080",
			"couldn't reach xray's local SOCKS port",
		},
		{
			"socks5 auth failed",
			"tunnel handshake failed (proxy refused the connection)",
		},
		{
			"forcibly closed by the remote host",
			"connection reset mid-handshake (likely ISP/DPI filtering or a dead origin)",
		},
		{
			"connection reset by peer",
			"connection reset mid-handshake (likely ISP/DPI filtering or a dead origin)",
		},
		{
			"unexpected eof",
			"connection reset mid-handshake (likely ISP/DPI filtering or a dead origin)",
		},
		{
			"connection refused",
			"origin refused the connection (dead endpoint or wrong port)",
		},
		{
			"http write: broken pipe",
			"no usable response through the tunnel (timeout / reset)",
		},
		{
			"http read timeout",
			"no usable response through the tunnel (timeout / reset)",
		},
		{
			"http 403",
			"http 403 (Cloudflare reached but didn't route — check SNI/Host)",
		},
		{
			"cancelled",
			"cancelled",
		},
		{
			"some uncategorized error",
			"some uncategorized error",
		},
	}
	for _, c := range cases {
		if got := summarizeFailure(c.in); got != c.want {
			t.Errorf("summarizeFailure(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIPOnly(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.2.3.4:443", "1.2.3.4"},
		{"[2606:4700::1]:443", "2606:4700::1"},
		{"nope", ""},
		{"", ""},
		{"1.2.3.4", ""}, // bare IP without port fails SplitHostPort
	}
	for _, c := range cases {
		if got := ipOnly(c.in); got != c.want {
			t.Errorf("ipOnly(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestApplyColo(t *testing.T) {
	// Empty map is a no-op (no panic).
	results := []CleanIPResult{
		{Endpoint: "1.2.3.4:443"},
		{Endpoint: "5.6.7.8:443"},
	}
	applyColo(results, nil)
	applyColo(results, map[string][2]string{})
	if results[0].Colo != "" || results[1].Colo != "" {
		t.Fatalf("empty map should leave colo empty, got %+v", results)
	}

	applyColo(results, map[string][2]string{
		"1.2.3.4": {"FRA", "DE"},
	})
	if results[0].Colo != "FRA" || results[0].Loc != "DE" {
		t.Errorf("matching row: got colo=%q loc=%q", results[0].Colo, results[0].Loc)
	}
	if results[1].Colo != "" || results[1].Loc != "" {
		t.Errorf("non-matching row should stay empty, got colo=%q loc=%q", results[1].Colo, results[1].Loc)
	}
}

func TestApplyQuality(t *testing.T) {
	// Empty map is a no-op.
	results := []CleanIPResult{
		{Endpoint: "1.2.3.4:443", Latency: 50 * time.Millisecond},
		{Endpoint: "5.6.7.8:443", Latency: 80 * time.Millisecond, Jitter: 10 * time.Millisecond, Best: 70 * time.Millisecond},
	}
	applyQuality(results, nil)
	applyQuality(results, map[string]qualitySample{})
	if results[0].Loss != 0 || results[0].Score != 0 {
		t.Fatalf("empty map should leave quality fields zero, got loss=%v score=%d", results[0].Loss, results[0].Score)
	}

	sample := qualitySample{
		best:   40 * time.Millisecond,
		median: 45 * time.Millisecond,
		jitter: 5 * time.Millisecond,
		loss:   25,
	}
	applyQuality(results, map[string]qualitySample{
		"1.2.3.4": sample,
	})

	if results[0].Loss != 25 {
		t.Errorf("loss = %v, want 25", results[0].Loss)
	}
	if results[0].Jitter != 5*time.Millisecond {
		t.Errorf("jitter should fill when zero: got %v", results[0].Jitter)
	}
	if results[0].Best != 40*time.Millisecond {
		t.Errorf("best should take sample.best when zero: got %v", results[0].Best)
	}
	wantScore := qualityScore(results[0].Latency, results[0].Jitter, results[0].Loss)
	if results[0].Score != wantScore {
		t.Errorf("score = %d, want %d", results[0].Score, wantScore)
	}
	// Non-matching row untouched.
	if results[1].Loss != 0 || results[1].Jitter != 10*time.Millisecond {
		t.Errorf("non-matching row mutated: %+v", results[1])
	}

	// Existing non-zero Jitter is preserved; Best only improves when sample is smaller.
	results[1].Endpoint = "1.2.3.4:8443"
	applyQuality(results[1:], map[string]qualitySample{
		"1.2.3.4": {
			best:   90 * time.Millisecond, // worse than existing Best=70ms
			jitter: 99 * time.Millisecond,
			loss:   10,
		},
	})
	if results[1].Jitter != 10*time.Millisecond {
		t.Errorf("existing jitter should be preserved, got %v", results[1].Jitter)
	}
	if results[1].Best != 70*time.Millisecond {
		t.Errorf("best should not regress to a worse sample, got %v", results[1].Best)
	}
	if results[1].Loss != 10 {
		t.Errorf("loss = %v, want 10", results[1].Loss)
	}
}
