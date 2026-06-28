package main

import (
	"fmt"
	"net"
	"strings"
	"testing"
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
