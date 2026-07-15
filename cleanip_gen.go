package main

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
)

// cfIPv4CIDRs are Cloudflare's published IPv4 ranges from https://www.cloudflare.com/ips-v4 .
// Keep this list official and compact; older scanner builds split 172.64.0.0/13
// into partial ranges and accidentally missed 172.68.0.0–172.71.255.255.
var cfIPv4CIDRs = []string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
}

// CFCDNPorts is the official list of Cloudflare CDN-supported TCP ports.
// HTTP:  80, 8080, 8880, 2052, 2082, 2086, 2095
// HTTPS: 443, 8443, 2053, 2083, 2087, 2096
var CFCDNPorts = []int{80, 443, 2052, 2053, 2082, 2083, 2086, 2087, 2095, 2096, 8080, 8443, 8880}

// maxNearbyEndpoints bounds the total endpoints the nearby pass emits, so that
// seeding from every Phase-1 responder (× countPerIP × ports) can't explode.
const maxNearbyEndpoints = 4096

// cfIPv6CIDRs are Cloudflare's published IPv6 ranges from https://www.cloudflare.com/ips-v6 .
var cfIPv6CIDRs = []string{
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

type CleanIPGenerator struct {
	rng *rand.Rand
}

func NewCleanIPGenerator() *CleanIPGenerator {
	return &CleanIPGenerator{
		rng: rand.New(rand.NewSource(rand.Int63())),
	}
}

type cidrInfo struct {
	cidr     string
	weight   int
	network4 uint32
	hostBits int
}

var v4CIDRInfo []cidrInfo
var v4TotalWeight int
var v6CIDRList []string

func init() {
	v4CIDRInfo = make([]cidrInfo, 0, len(cfIPv4CIDRs))
	for _, cidr := range cfIPv4CIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		ones, bits := ipnet.Mask.Size()
		ip4 := ipnet.IP.To4()
		if ip4 == nil {
			continue
		}
		network := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
		hostBits := bits - ones
		weight := 1
		if ones < 24 {
			weight = 1 << (24 - ones)
		}
		v4CIDRInfo = append(v4CIDRInfo, cidrInfo{
			cidr:     cidr,
			weight:   weight,
			network4: network,
			hostBits: hostBits,
		})
		v4TotalWeight += weight
	}
	v6CIDRList = cfIPv6CIDRs
}

func (g *CleanIPGenerator) GenerateIPs(count int, useIPv4, useIPv6 bool, ports []int) []string {
	if len(ports) == 0 {
		ports = []int{443}
	}

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

	seen := make(map[string]bool)
	v4IPs := make([]string, 0, v4Count)
	v6IPs := make([]string, 0, v6Count)

	for attempts := 0; len(v4IPs) < v4Count && attempts < v4Count*20; attempts++ {
		idx := pickWeighted(g.rng, v4CIDRInfo, v4TotalWeight)
		if idx < 0 {
			break
		}
		ci := v4CIDRInfo[idx]
		offset := g.rng.Intn(1 << ci.hostBits)
		n := ci.network4 + uint32(offset)
		ip := fmt.Sprintf("%d.%d.%d.%d", byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
		if seen[ip] {
			continue
		}
		seen[ip] = true
		v4IPs = append(v4IPs, ip)
	}

	for attempts := 0; len(v6IPs) < v6Count && attempts < v6Count*20; attempts++ {
		cidr := v6CIDRList[g.rng.Intn(len(v6CIDRList))]
		ip := randomIPv6InCIDR(cidr, g.rng)
		if seen[ip] {
			continue
		}
		seen[ip] = true
		v6IPs = append(v6IPs, ip)
	}

	endpoints := make([]string, 0, (len(v4IPs)+len(v6IPs))*len(ports))
	for _, ip := range v4IPs {
		for _, p := range ports {
			endpoints = append(endpoints, fmt.Sprintf("%s:%d", ip, p))
		}
	}
	for _, ip := range v6IPs {
		for _, p := range ports {
			endpoints = append(endpoints, fmt.Sprintf("[%s]:%d", ip, p))
		}
	}
	return capEndpoints(endpoints)
}

// capEndpoints truncates the endpoint list to maxScanCount so port
// multiplication cannot exceed the allocation budget that maxScanCount
// was meant to enforce. maxScanCount lives in httpserver.go (same package).
func capEndpoints(endpoints []string) []string {
	if len(endpoints) > maxScanCount {
		return endpoints[:maxScanCount]
	}
	return endpoints
}

func pickWeighted(rng *rand.Rand, items []cidrInfo, totalWeight int) int {
	if totalWeight <= 0 || len(items) == 0 {
		return -1
	}
	r := rng.Intn(totalWeight)
	for i, ci := range items {
		if r < ci.weight {
			return i
		}
		r -= ci.weight
	}
	return len(items) - 1
}

func generateNearbyIPs(working []CleanIPResult, countPerIP int, ports []int) []string {
	if len(ports) == 0 {
		ports = []int{443}
	}
	rng := rand.New(rand.NewSource(rand.Int63()))
	seen := make(map[string]bool)
	var result []string

	maxResults := len(working) * countPerIP * len(ports)
	if maxResults > maxNearbyEndpoints {
		maxResults = maxNearbyEndpoints
	}

	for _, wr := range working {
		ep := wr.Endpoint
		// strip port to get IP
		host := ep
		if strings.Contains(ep, ":") {
			// handle IPv6 [::1]:port vs IPv4 x.x.x.x:port
			if ep[0] == '[' {
				idx := strings.LastIndex(ep, "]:")
				if idx > 0 {
					host = ep[1:idx]
				}
			} else {
				idx := strings.LastIndex(ep, ":")
				if idx > 0 {
					host = ep[:idx]
				}
			}
		}

		ip := net.ParseIP(host)
		if ip == nil {
			continue
		}

		if ip4 := ip.To4(); ip4 != nil {
			// /24 subnet: x.y.z.0
			base := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8
			for attempts := 0; len(result) < maxResults && attempts < countPerIP*50; attempts++ {
				offset := uint32(rng.Intn(254) + 1) // skip .0 (network) and .255 (broadcast)
				ipU32 := base | offset
				s := fmt.Sprintf("%d.%d.%d.%d", byte(ipU32>>24), byte(ipU32>>16), byte(ipU32>>8), byte(ipU32))
				if seen[s] {
					continue
				}
				seen[s] = true
				for _, p := range ports {
					result = append(result, fmt.Sprintf("%s:%d", s, p))
				}
				if len(result) >= maxResults {
					return result
				}
			}
		} else {
			// IPv6 /64 subnet: randomize last 64 bits
			for attempts := 0; len(result) < maxResults && attempts < countPerIP*50; attempts++ {
				out := make(net.IP, 16)
				copy(out, ip)
				// randomize bytes 8-15 (last 64 bits)
				for i := 8; i < 16; i++ {
					out[i] = byte(rng.Intn(256))
				}
				s := out.String()
				if seen[s] {
					continue
				}
				seen[s] = true
				for _, p := range ports {
					result = append(result, fmt.Sprintf("[%s]:%d", s, p))
				}
				if len(result) >= maxResults {
					return result
				}
			}
		}
	}

	return result
}

func randomIPv6InCIDR(cidr string, rng *rand.Rand) string {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}
	ones, _ := ipnet.Mask.Size()
	ip := make(net.IP, 16)
	copy(ip, ipnet.IP.To16())
	// Randomize exactly the host bits (everything after the first `ones` bits),
	// masking at the bit level. Preserving whole bytes (ones/8) only works for
	// byte-aligned prefixes; a /29 like 2a06:98c0::/29 has 5 network bits in byte
	// 3, so the old code generated addresses OUTSIDE the published Cloudflare
	// range — wasting scan budget on IPs that aren't Cloudflare's.
	for i := 0; i < 16; i++ {
		bitStart := i * 8
		switch {
		case bitStart >= ones:
			// Fully a host byte — randomize all 8 bits.
			ip[i] = byte(rng.Intn(256))
		case bitStart+8 > ones:
			// Straddling byte — top (ones-bitStart) bits are network, low bits host.
			hostBits := bitStart + 8 - ones
			mask := byte((1 << hostBits) - 1)
			ip[i] = (ip[i] &^ mask) | (byte(rng.Intn(256)) & mask)
		}
		// else fully a network byte — leave as-is.
	}
	return ip.String()
}
