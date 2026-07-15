package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"strconv"
	"strings"
)

// IPRange is an inclusive [lo, hi] address range. IPv4 values live in
// [0, 2^32); IPv6 values in [0, 2^128). big.Int lets both families share one
// code path (a single /32 IPv6 block already overflows uint64).
type IPRange struct {
	v6 bool
	lo *big.Int
	hi *big.Int
}

// maxEnumerate caps full enumeration so a pasted "small" range can't blow up
// memory. Above this (or above the requested count) we switch to sampling.
const maxEnumerate = 500000

func newRangeRNG() *rand.Rand {
	return rand.New(rand.NewSource(rand.Int63()))
}

// ParseIPRanges parses a block of text where each non-empty, non-comment line is
// a CIDR (104.16.0.0/13), a dash range (104.16.0.0-104.16.5.255 or short
// 104.16.0.0-255), or a single IP (104.16.1.1). IPv4 and IPv6 are both accepted.
// Valid ranges are returned; unparseable lines are collected in bad.
func ParseIPRanges(text string) (ranges []IPRange, bad []string) {
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// Allow a trailing inline comment (e.g. "104.16.0.0/13 # cloudflare").
		if i := strings.IndexAny(line, "#;"); i > 0 {
			line = strings.TrimSpace(line[:i])
		}
		if r, ok := parseOneRange(line); ok {
			ranges = append(ranges, r)
		} else {
			bad = append(bad, line)
		}
	}
	return ranges, bad
}

func parseOneRange(line string) (IPRange, bool) {
	switch {
	case strings.Contains(line, "/"):
		return parseCIDRRange(line)
	case strings.Contains(line, "-"):
		return parseDashRange(line)
	default:
		return parseSingleIP(line)
	}
}

func parseCIDRRange(line string) (IPRange, bool) {
	_, ipnet, err := net.ParseCIDR(line)
	if err != nil {
		return IPRange{}, false
	}
	ones, bits := ipnet.Mask.Size()
	if bits == 0 {
		return IPRange{}, false
	}
	lo := new(big.Int).SetBytes(ipnet.IP)
	size := new(big.Int).Lsh(big.NewInt(1), uint(bits-ones))
	hi := new(big.Int).Add(lo, size)
	hi.Sub(hi, big.NewInt(1))
	return IPRange{v6: bits == 128, lo: lo, hi: hi}, true
}

func parseDashRange(line string) (IPRange, bool) {
	i := strings.IndexByte(line, '-')
	a := net.ParseIP(strings.TrimSpace(line[:i]))
	if a == nil {
		return IPRange{}, false
	}
	v6 := a.To4() == nil
	lo := new(big.Int).SetBytes(ipBytes(a, v6))

	bStr := strings.TrimSpace(line[i+1:])
	var hi *big.Int
	if b := net.ParseIP(bStr); b != nil {
		if (b.To4() == nil) != v6 {
			return IPRange{}, false // mixed families
		}
		hi = new(big.Int).SetBytes(ipBytes(b, v6))
	} else {
		h, ok := parseShortHi(a, v6, bStr)
		if !ok {
			return IPRange{}, false
		}
		hi = h
	}
	if hi.Cmp(lo) < 0 {
		return IPRange{}, false
	}
	return IPRange{v6: v6, lo: lo, hi: hi}, true
}

// parseShortHi handles the abbreviated upper bound of a dash range: the last
// octet for IPv4 (104.16.0.0-255) or the last hextet for IPv6 (2606:4700::-ffff).
func parseShortHi(a net.IP, v6 bool, b string) (*big.Int, bool) {
	out := make([]byte, len(ipBytes(a, v6)))
	copy(out, ipBytes(a, v6))
	if !v6 {
		n, err := strconv.Atoi(b)
		if err != nil || n < 0 || n > 255 {
			return nil, false
		}
		out[3] = byte(n)
		return new(big.Int).SetBytes(out), true
	}
	n, err := strconv.ParseUint(b, 16, 16)
	if err != nil {
		return nil, false
	}
	out[14] = byte(n >> 8)
	out[15] = byte(n)
	return new(big.Int).SetBytes(out), true
}

func parseSingleIP(line string) (IPRange, bool) {
	ip := net.ParseIP(line)
	if ip == nil {
		return IPRange{}, false
	}
	v6 := ip.To4() == nil
	n := new(big.Int).SetBytes(ipBytes(ip, v6))
	return IPRange{v6: v6, lo: n, hi: new(big.Int).Set(n)}, true
}

func ipBytes(ip net.IP, v6 bool) []byte {
	if v6 {
		return ip.To16()
	}
	return ip.To4()
}

func (r IPRange) size() *big.Int {
	s := new(big.Int).Sub(r.hi, r.lo)
	return s.Add(s, big.NewInt(1))
}

func formatIP(n *big.Int, v6 bool) string {
	width := 4
	if v6 {
		width = 16
	}
	b := n.Bytes()
	out := make(net.IP, width)
	if len(b) > width {
		b = b[len(b)-width:]
	}
	copy(out[width-len(b):], b)
	return out.String()
}

// GenerateFromRanges turns parsed ranges into "ip:port" endpoints. Smart mode:
// if the ranges hold no more than `count` addresses (and stay under
// maxEnumerate) every IP is tested; otherwise it samples `count` unique IPs
// weighted by range size. Each IP is crossed with every port.
func GenerateFromRanges(ranges []IPRange, count int, ports []int, rng *rand.Rand) []string {
	if len(ranges) == 0 {
		return nil
	}
	if len(ports) == 0 {
		ports = []int{443}
	}
	if count <= 0 {
		count = 1000
	}
	if rng == nil {
		rng = newRangeRNG()
	}

	total := new(big.Int)
	for _, r := range ranges {
		total.Add(total, r.size())
	}

	var ips []string
	if total.Cmp(big.NewInt(int64(count))) <= 0 && total.Cmp(big.NewInt(maxEnumerate)) <= 0 {
		ips = enumerateAll(ranges)
	} else {
		ips = sampleN(ranges, total, count, rng)
	}

	endpoints := make([]string, 0, len(ips)*len(ports))
	for _, ip := range ips {
		bracketed := strings.Contains(ip, ":") // IPv6 textual form
		for _, p := range ports {
			if bracketed {
				endpoints = append(endpoints, fmt.Sprintf("[%s]:%d", ip, p))
			} else {
				endpoints = append(endpoints, fmt.Sprintf("%s:%d", ip, p))
			}
		}
	}
	return capEndpoints(endpoints)
}

func enumerateAll(ranges []IPRange) []string {
	seen := make(map[string]bool)
	var ips []string
	one := big.NewInt(1)
	for _, r := range ranges {
		for cur := new(big.Int).Set(r.lo); cur.Cmp(r.hi) <= 0; cur.Add(cur, one) {
			s := formatIP(cur, r.v6)
			if seen[s] {
				continue
			}
			seen[s] = true
			ips = append(ips, s)
		}
	}
	return ips
}

func sampleN(ranges []IPRange, total *big.Int, count int, rng *rand.Rand) []string {
	seen := make(map[string]bool)
	ips := make([]string, 0, count)
	for attempts := 0; len(ips) < count && attempts < count*20; attempts++ {
		idx := new(big.Int).Rand(rng, total)
		r, offset := locate(ranges, idx)
		s := formatIP(new(big.Int).Add(r.lo, offset), r.v6)
		if seen[s] {
			continue
		}
		seen[s] = true
		ips = append(ips, s)
	}
	return ips
}

// locate maps a global index in [0, total) to the owning range and the offset
// inside it, walking the ranges in order and subtracting each one's size.
func locate(ranges []IPRange, idx *big.Int) (IPRange, *big.Int) {
	cur := new(big.Int).Set(idx)
	for _, r := range ranges {
		if sz := r.size(); cur.Cmp(sz) < 0 {
			return r, cur
		} else {
			cur.Sub(cur, sz)
		}
	}
	last := ranges[len(ranges)-1]
	return last, new(big.Int).Sub(last.size(), big.NewInt(1))
}
