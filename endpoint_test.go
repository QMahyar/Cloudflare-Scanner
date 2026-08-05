package main

import (
	"net"
	"testing"
)

// TestGenerateExactCount verifies the generator returns exactly the requested
// number of endpoints when the count is within the available address pool.
func TestGenerateExactCount(t *testing.T) {
	eps := GenerateEndpoints(100, true, false)
	if len(eps) != 100 {
		t.Fatalf("expected 100 endpoints, got %d", len(eps))
	}
	for _, ep := range eps {
		if _, _, err := net.SplitHostPort(ep); err != nil {
			t.Errorf("invalid endpoint %q: %v", ep, err)
		}
	}
}

// TestGenerateBoundedOnExhaustedPool guards against the infinite loop that used
// to occur when count exceeded the finite IPv4 address pool
// (len(ipv4Prefixes)*256 unique IPs). The generator must return promptly with at
// most the pool size rather than spinning forever. If the attempt bound
// regresses, this test hangs and the suite's timeout fails it.
func TestGenerateBoundedOnExhaustedPool(t *testing.T) {
	poolSize := len(ipv4Prefixes) * 256
	eps := GenerateEndpoints(poolSize*2, true, false) // far more than the pool can supply
	if len(eps) == 0 {
		t.Fatal("expected some endpoints, got none")
	}
	if len(eps) > poolSize {
		t.Fatalf("returned %d endpoints, exceeds unique IPv4 pool of %d", len(eps), poolSize)
	}
	// An IPv4-only request must never emit IPv6 endpoints, even when the v4 pool
	// is exhausted (regression: the v6 backfill loop must not run when v6Count==0).
	for _, ep := range eps {
		host, _, err := net.SplitHostPort(ep)
		if err != nil {
			t.Fatalf("invalid endpoint %q: %v", ep, err)
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.To4() == nil {
			t.Fatalf("ipv4-only request leaked non-IPv4 endpoint: %q", ep)
		}
	}
	// Sanity: a request within the pool still hits its target exactly.
	if got := len(GenerateEndpoints(1000, true, false)); got != 1000 {
		t.Fatalf("expected 1000 endpoints for in-pool count, got %d", got)
	}
}

// TestGenerateEndpointsNoDuplicateEndpoints verifies that every generated
// endpoint (ip:port) is unique, even when the IP pool is small relative to the
// requested count. The old code marked an IP as seen before checking whether its
// port was a duplicate, silently dropping that IP forever.
func TestGenerateEndpointsNoDuplicateEndpoints(t *testing.T) {
	eps := GenerateEndpoints(500, true, false)
	seen := make(map[string]bool, len(eps))
	for _, ep := range eps {
		if seen[ep] {
			t.Errorf("duplicate endpoint generated: %s", ep)
		}
		seen[ep] = true
	}
}

// TestGenerateEndpointsOddCountSplitsEvenly verifies that with both families
// enabled, odd counts round IPv4 UP instead of skewing the whole remainder to
// IPv6. The old count/2 split left count=1 with 0 v4 / 1 v6 — a pure IPv6
// endpoint that fails outright on IPv4-only networks.
func TestGenerateEndpointsOddCountSplitsEvenly(t *testing.T) {
	for _, tc := range []struct {
		count, wantV4, wantV6 int
	}{
		{1, 1, 0},
		{2, 1, 1},
		{3, 2, 1},
		{4, 2, 2},
		{5, 3, 2},
	} {
		eps := GenerateEndpoints(tc.count, true, true)
		if len(eps) != tc.count {
			t.Fatalf("count=%d: got %d endpoints", tc.count, len(eps))
		}
		v4, v6 := 0, 0
		for _, ep := range eps {
			host, _, err := net.SplitHostPort(ep)
			if err != nil {
				t.Fatalf("count=%d: invalid endpoint %q", tc.count, ep)
			}
			if net.ParseIP(host).To4() != nil {
				v4++
			} else {
				v6++
			}
		}
		if v4 != tc.wantV4 || v6 != tc.wantV6 {
			t.Errorf("count=%d: got v4=%d v6=%d, want v4=%d v6=%d", tc.count, v4, v6, tc.wantV4, tc.wantV6)
		}
	}
}
