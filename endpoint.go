package main

import (
	"fmt"
	"math/rand"
)

var (
	ipv4Prefixes = []string{
		"188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.",
		"162.159.192.", "162.159.193.", "162.159.195.",
		"8.34.146.", "8.39.214.", "8.39.204.", "8.6.112.",
		"8.35.211.", "8.39.125.", "8.47.69.",
	}

	ipv6Prefixes = []string{
		"2606:4700:d0::", "2606:4700:d1::",
		"2606:4700:110::", "2606:4700:111::",
	}

	ports = []int{
		500, 854, 859, 864, 878, 880, 890, 891, 894, 903,
		908, 928, 934, 939, 942, 943, 945, 946, 955, 968,
		987, 988, 1002, 1010, 1014, 1018, 1070, 1074, 1180,
		1387, 1701, 1843, 2371, 2408, 2506, 3138, 3476, 3581,
		3854, 4177, 4198, 4233, 4500, 5279, 5956, 7103, 7152,
		7156, 7281, 7559, 8319, 8742, 8854, 8886,
	}
)

func GenerateEndpoints(count int, useIPv4, useIPv6 bool) []string {
	rng := rand.New(rand.NewSource(rand.Int63()))
	endpoints := make([]string, 0, count)
	seen := make(map[string]bool)

	v4Count, v6Count := 0, 0
	switch {
	case useIPv4 && useIPv6:
		v4Count = count / 2
		v6Count = count - v4Count
	case useIPv4:
		v4Count = count
	default:
		v6Count = count
	}

	// The IPv4 pool is finite (len(ipv4Prefixes)*256 unique IPs). Bounding the
	// attempts keeps an over-large count from spinning forever once the pool is
	// exhausted — it simply yields fewer endpoints. Mirrors CleanIPGenerator.
	for attempts := 0; len(endpoints) < v4Count && attempts < v4Count*20+len(ipv4Prefixes)*256; attempts++ {
		prefix := ipv4Prefixes[rng.Intn(len(ipv4Prefixes))]
		ip := fmt.Sprintf("%s%d", prefix, rng.Intn(256))
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = true
		ep := fmt.Sprintf("%s:%d", ip, ports[rng.Intn(len(ports))])
		if !seen[ep] {
			seen[ep] = true
			endpoints = append(endpoints, ep)
		}
	}

	// The IPv6 loop targets v6Count endpoints ON TOP OF whatever v4 produced, so
	// an exhausted/under-filled v4 pool never causes IPv6 to be emitted when none
	// was requested (v6Count == 0 → target == current len → loop never runs). The
	// attempt cap guards against a degenerate prefix set livelocking on collisions.
	v6Target := len(endpoints) + v6Count
	for attempts := 0; len(endpoints) < v6Target && attempts < v6Count*20+1024; attempts++ {
		prefix := ipv6Prefixes[rng.Intn(len(ipv6Prefixes))]
		ip := fmt.Sprintf("[%s%x:%x:%x:%x]", prefix,
			rng.Intn(65536), rng.Intn(65536),
			rng.Intn(65536), rng.Intn(65536))
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = true
		ep := fmt.Sprintf("%s:%d", ip, ports[rng.Intn(len(ports))])
		if !seen[ep] {
			seen[ep] = true
			endpoints = append(endpoints, ep)
		}
	}

	return endpoints
}
