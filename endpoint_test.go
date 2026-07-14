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
