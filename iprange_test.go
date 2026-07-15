package main

import (
	"math/big"
	"strings"
	"testing"
)

func TestParseIPRanges_Line(t *testing.T) {
	cases := []struct {
		in     string
		v6     bool
		lo, hi string
		size   int64
	}{
		{"104.16.0.0/24", false, "104.16.0.0", "104.16.0.255", 256},
		{"104.16.0.0/30", false, "104.16.0.0", "104.16.0.3", 4},
		{"104.16.0.0-104.16.0.255", false, "104.16.0.0", "104.16.0.255", 256},
		{"104.16.0.0-255", false, "104.16.0.0", "104.16.0.255", 256},
		{"104.16.1.1", false, "104.16.1.1", "104.16.1.1", 1},
		{"2606:4700::/120", true, "2606:4700::", "2606:4700::ff", 256},
		{"2606:4700::-ffff", true, "2606:4700::", "2606:4700::ffff", 65536},
	}
	for _, tc := range cases {
		ranges, bad := ParseIPRanges(tc.in)
		if len(bad) != 0 {
			t.Errorf("%q: unexpected bad lines %v", tc.in, bad)
			continue
		}
		if len(ranges) != 1 {
			t.Errorf("%q: want 1 range, got %d", tc.in, len(ranges))
			continue
		}
		r := ranges[0]
		if r.v6 != tc.v6 {
			t.Errorf("%q: v6=%v, want %v", tc.in, r.v6, tc.v6)
		}
		if got := formatIP(r.lo, r.v6); got != tc.lo {
			t.Errorf("%q: lo=%s, want %s", tc.in, got, tc.lo)
		}
		if got := formatIP(r.hi, r.v6); got != tc.hi {
			t.Errorf("%q: hi=%s, want %s", tc.in, got, tc.hi)
		}
		if got := r.size(); got.Cmp(big.NewInt(tc.size)) != 0 {
			t.Errorf("%q: size=%s, want %d", tc.in, got, tc.size)
		}
	}
}

func TestParseIPRanges_BadAndComments(t *testing.T) {
	in := strings.Join([]string{
		"104.16.0.0/24",
		"# a comment",
		"",
		"  ; another",
		"garbage",
		"1.2.3.4-2606:4700::1", // mixed families
		"104.24.0.0/13  # inline comment",
	}, "\n")

	ranges, bad := ParseIPRanges(in)
	if len(ranges) != 2 {
		t.Errorf("want 2 valid ranges (the two CIDRs), got %d", len(ranges))
	}
	if len(bad) != 2 {
		t.Errorf("want 2 bad lines (garbage + mixed-family), got %d: %v", len(bad), bad)
	}
}

func TestGenerateFromRanges_Enumerate(t *testing.T) {
	ranges, _ := ParseIPRanges("104.16.0.0/30") // 4 addresses
	eps := GenerateFromRanges(ranges, 1000, []int{443, 80}, newRangeRNG())

	if len(eps) != 8 { // 4 IPs × 2 ports, fully enumerated (4 <= 1000)
		t.Fatalf("want 8 endpoints, got %d: %v", len(eps), eps)
	}
	seen := map[string]bool{}
	for _, e := range eps {
		if seen[e] {
			t.Errorf("duplicate endpoint %s", e)
		}
		seen[e] = true
		if !strings.HasPrefix(e, "104.16.0.") {
			t.Errorf("endpoint outside /30: %s", e)
		}
	}
}

func TestGenerateFromRanges_Sample(t *testing.T) {
	ranges, _ := ParseIPRanges("104.16.0.0/16") // 65536 addresses → must sample
	eps := GenerateFromRanges(ranges, 100, []int{443}, newRangeRNG())

	if len(eps) != 100 {
		t.Fatalf("want 100 sampled endpoints, got %d", len(eps))
	}
	seen := map[string]bool{}
	for _, e := range eps {
		if seen[e] {
			t.Errorf("duplicate sampled endpoint %s", e)
		}
		seen[e] = true
	}
}

func TestGenerateFromRanges_IPv6Bracketed(t *testing.T) {
	ranges, _ := ParseIPRanges("2606:4700::/126") // 4 addresses
	eps := GenerateFromRanges(ranges, 100, []int{443}, newRangeRNG())

	if len(eps) != 4 {
		t.Fatalf("want 4 endpoints, got %d: %v", len(eps), eps)
	}
	for _, e := range eps {
		if !strings.HasPrefix(e, "[") || !strings.HasSuffix(e, "]:443") {
			t.Errorf("IPv6 endpoint not bracketed: %s", e)
		}
	}
}

func TestGenerateFromRanges_CappedAfterPorts(t *testing.T) {
	ranges, err := ParseIPRanges("104.16.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	ports := []int{443, 8443, 2053}
	eps := GenerateFromRanges(ranges, 10, ports, newRangeRNG())
	if len(eps) > 30 {
		t.Fatalf("endpoint count %d exceeds ip*ports 30", len(eps))
	}
	if len(eps) > maxScanCount {
		t.Fatalf("endpoint count %d exceeds maxScanCount %d", len(eps), maxScanCount)
	}
}
