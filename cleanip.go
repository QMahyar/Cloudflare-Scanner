package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

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
	"104.16.0.0/12",
	"172.64.0.0/17",
	"172.64.128.0/18",
	"172.64.192.0/19",
	"172.64.224.0/22",
	"172.64.229.0/24",
	"172.64.230.0/23",
	"172.64.232.0/21",
	"172.64.240.0/21",
	"172.64.248.0/21",
	"172.65.0.0/16",
	"172.66.0.0/16",
	"172.67.0.0/16",
	"131.0.72.0/22",
}

// CFCDNPorts is the official list of Cloudflare CDN-supported TCP ports.
// HTTP:  80, 8080, 8880, 2052, 2082, 2086, 2095
// HTTPS: 443, 8443, 2053, 2083, 2087, 2096
var CFCDNPorts = []int{80, 443, 2052, 2053, 2082, 2083, 2086, 2087, 2095, 2096, 8080, 8443, 8880}

// maxNearbyEndpoints bounds the total endpoints the nearby pass emits, so that
// seeding from every Phase-1 responder (× countPerIP × ports) can't explode.
const maxNearbyEndpoints = 4096

// cleanSocksPortBase hands out non-overlapping SOCKS port windows for clean-IP
// Phase-2 batches. It is process-global (not per-job) so two clean scans running
// at once can't allocate the same window and fight over the same xray ports.
var cleanSocksPortBase atomic.Int32

var cfIPv6CIDRs = []string{
	"2400:cb00:2049::/48",
	"2400:cb00:f00e::/48",
	"2606:4700::/32",
	"2606:4700:10::/48",
	"2606:4700:130::/48",
	"2606:4700:3000::/48",
	"2606:4700:3001::/48",
	"2606:4700:3002::/48",
	"2606:4700:3003::/48",
	"2606:4700:3004::/48",
	"2606:4700:3005::/48",
	"2606:4700:3006::/48",
	"2606:4700:3007::/48",
	"2606:4700:3008::/48",
	"2606:4700:3009::/48",
	"2606:4700:3010::/48",
	"2606:4700:3011::/48",
	"2606:4700:3012::/48",
	"2606:4700:3013::/48",
	"2606:4700:3014::/48",
	"2606:4700:3015::/48",
	"2606:4700:3016::/48",
	"2606:4700:3017::/48",
	"2606:4700:3018::/48",
	"2606:4700:3019::/48",
	"2606:4700:3020::/48",
	"2606:4700:3021::/48",
	"2606:4700:3022::/48",
	"2606:4700:3023::/48",
	"2606:4700:3024::/48",
	"2606:4700:3025::/48",
	"2606:4700:3026::/48",
	"2606:4700:3027::/48",
	"2606:4700:3028::/48",
	"2606:4700:3029::/48",
	"2606:4700:3030::/48",
	"2606:4700:3031::/48",
	"2606:4700:3032::/48",
	"2606:4700:3033::/48",
	"2606:4700:3034::/48",
	"2606:4700:3035::/48",
	"2606:4700:3036::/48",
	"2606:4700:3037::/48",
	"2606:4700:3038::/48",
	"2606:4700:3039::/48",
	"2606:4700:a0::/48",
	"2606:4700:a1::/48",
	"2606:4700:a8::/48",
	"2606:4700:a9::/48",
	"2606:4700:a::/48",
	"2606:4700:b::/48",
	"2606:4700:c::/48",
	"2606:4700:d0::/48",
	"2606:4700:d1::/48",
	"2606:4700:d::/48",
	"2606:4700:e0::/48",
	"2606:4700:e1::/48",
	"2606:4700:e2::/48",
	"2606:4700:e3::/48",
	"2606:4700:e4::/48",
	"2606:4700:e5::/48",
	"2606:4700:e6::/48",
	"2606:4700:e7::/48",
	"2606:4700:e::/48",
	"2606:4700:f1::/48",
	"2606:4700:f2::/48",
	"2606:4700:f3::/48",
	"2606:4700:f4::/48",
	"2606:4700:f5::/48",
	"2606:4700:f::/48",
	"2803:f800:50::/48",
	"2803:f800:51::/48",
	"2a06:98c1:3100::/48",
	"2a06:98c1:3101::/48",
	"2a06:98c1:3102::/48",
	"2a06:98c1:3103::/48",
	"2a06:98c1:3104::/48",
	"2a06:98c1:3105::/48",
	"2a06:98c1:3106::/48",
	"2a06:98c1:3107::/48",
	"2a06:98c1:3108::/48",
	"2a06:98c1:3109::/48",
	"2a06:98c1:310a::/48",
	"2a06:98c1:310b::/48",
	"2a06:98c1:310c::/48",
	"2a06:98c1:310d::/48",
	"2a06:98c1:310e::/48",
	"2a06:98c1:310f::/48",
	"2a06:98c1:3120::/48",
	"2a06:98c1:3121::/48",
	"2a06:98c1:3122::/48",
	"2a06:98c1:3123::/48",
	"2a06:98c1:3200::/48",
	"2a06:98c1:50::/48",
	"2a06:98c1:51::/48",
	"2a06:98c1:54::/48",
	"2a06:98c1:58::/48",
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
				offset := uint32(rng.Intn(256))
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
	copy(ip, ipnet.IP)
	fullBytes := ones / 8
	for i := fullBytes; i < 16; i++ {
		ip[i] = byte(rng.Intn(256))
	}
	return ip.String()
}

type CleanIPResult struct {
	Endpoint string
	Latency  time.Duration
	Success  bool
	Error    string
	Attempts int
	Passes   int
	Best     time.Duration
	Jitter   time.Duration
	Colo     string
	Loc      string
}

type CleanIPJob struct {
	ID                  string
	Status              string
	Progress            int
	Total               int
	Phase1Progress      int
	Phase1Total         int
	Phase2Progress      int
	Phase2Total         int
	Config              *ProxyConfig
	Endpoints           []string
	Phase1Results       []CleanIPResult
	Phase2Results       []CleanIPResult
	Phase2Count         int
	SkipPhase2          bool
	NearbyScan          bool
	NearbyCount         int
	Phase1Probes        int
	Phase2Probes        int
	TimeoutMs           int
	Phase2TimeoutMs     int
	StopAfter           int
	ScanPorts           []int
	NearbyPhase1Results []CleanIPResult
	NearbyPhase2Results []CleanIPResult
	Cancel              chan struct{}
	cancelOnce          sync.Once
	mu                  sync.Mutex
}

func (j *CleanIPJob) stop() {
	j.cancelOnce.Do(func() { close(j.Cancel) })
}

// dialReachable TCP-dials endpoint, retrying ONLY on timeout (up to maxAttempts).
// A single dropped SYN under the high-concurrency burst would otherwise discard
// an IP whose real RTT is well under the deadline — the cause of "tight timeout
// finds nothing, loose timeout finds the same IPs fast". A refused/unreachable
// port won't change on retry, so those return immediately and don't pay the cost.
func dialReachable(endpoint string, timeout time.Duration, maxAttempts int) (time.Duration, bool) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", endpoint, timeout)
		if err == nil {
			conn.Close()
			return time.Since(start), true
		}
		var ne net.Error
		if !errors.As(err, &ne) || !ne.Timeout() {
			return 0, false
		}
	}
	return 0, false
}

func runCleanPhase1TCP(ctx context.Context, endpoints []string, timeout time.Duration, cancel chan struct{}, job *CleanIPJob, concurrency int, stopAfter int) []CleanIPResult {
	var mu sync.Mutex
	var wg sync.WaitGroup
	if concurrency <= 0 {
		concurrency = 500
	}
	sem := semaphore.NewWeighted(int64(concurrency))
	results := make([]CleanIPResult, 0, len(endpoints))

	// localStop lets phase 1 finish early once stopAfter responders are found,
	// WITHOUT closing the job's global Cancel (which would also abort phase 2).
	localStop := make(chan struct{})
	var stopOnce sync.Once
	stopNow := func() { stopOnce.Do(func() { close(localStop) }) }

	// phaseCtx is cancelled on either job cancel or local early-stop, so a
	// single context drives all semaphore Acquire calls without per-goroutine
	// cancel watchers.
	phaseCtx, phaseCancel := context.WithCancel(ctx)
	defer phaseCancel()
	go func() {
		select {
		case <-localStop:
			phaseCancel()
		case <-phaseCtx.Done():
		}
	}()

	for i, ep := range endpoints {
		select {
		case <-cancel:
			return results
		case <-localStop:
			// Target reached — stop launching new probes; in-flight ones drain.
			goto wait
		default:
		}

		wg.Add(1)
		go func(endpoint string, idx int) {
			defer wg.Done()
			if err := sem.Acquire(phaseCtx, 1); err != nil {
				return
			}
			defer sem.Release(1)

			latency, ok := dialReachable(endpoint, timeout, 2)
			if !ok {
				if job != nil {
					job.mu.Lock()
					job.Phase1Progress++
					job.mu.Unlock()
				}
				return
			}

			// Colo/loc are enriched separately (see enrichColo) for a bounded set
			// of the fastest responders — keeping the trace round-trip out of this
			// dial loop so dense ranges aren't throttled to ~2s per responder.
			result := CleanIPResult{
				Endpoint: endpoint,
				Latency:  latency,
				Success:  true,
				Attempts: 1,
				Passes:   1,
				Best:     latency,
			}

			mu.Lock()
			results = append(results, result)
			n := len(results)
			mu.Unlock()

			if job != nil {
				job.mu.Lock()
				job.Phase1Results = append(job.Phase1Results, result)
				job.Phase1Progress++
				job.mu.Unlock()
			}

			if stopAfter > 0 && n >= stopAfter {
				stopNow()
			}
		}(ep, i)
	}

wait:
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-cancel:
		// Exit fast with partial results. In-flight goroutines keep appending to
		// `results` under mu in the background, so snapshot under the same lock
		// rather than reading/sorting it concurrently (which would race the
		// appenders on the slice header).
		mu.Lock()
		partial := append([]CleanIPResult(nil), results...)
		mu.Unlock()
		sort.Slice(partial, func(i, j int) bool {
			return partial[i].Latency < partial[j].Latency
		})
		return partial
	case <-localStop:
		// target reached; let in-flight finish so partial results are coherent
		<-done
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

	return results
}

func probeCloudflareTrace(ctx context.Context, endpoint, sni string, timeout time.Duration) (colo, loc string) {
	_, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", ""
	}
	// /cdn-cgi/trace is answered by the Cloudflare edge that terminates the
	// connection, and exists on every CF-proxied hostname. The well-known
	// speed.cloudflare.com SNI is exactly what DPI on filtered ISPs resets on
	// sight, which is why a direct edge probe returns nothing there and the colo
	// column stays empty. Reusing the user's own config SNI — which their working
	// config proves is unblocked — lets the probe complete and report that IP's
	// real colo. Falls back to speed.cloudflare.com when no domain SNI is given
	// (e.g. one-phase scans with no config).
	host := strings.TrimSpace(sni)
	if host == "" || net.ParseIP(host) != nil {
		host = "speed.cloudflare.com"
	}
	scheme := "http"
	if port == "443" || port == "8443" || port == "2053" || port == "2083" || port == "2087" || port == "2096" {
		scheme = "https"
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, endpoint)
		},
		TLSClientConfig:   &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12},
		DisableKeepAlives: true,
	}
	// One-shot transport per probe — release its connection promptly instead of
	// leaving idle conns to linger until GC (up to ~150 probes per scan).
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+host+"/cdn-cgi/trace", nil)
	if err != nil {
		return "", ""
	}
	req.Host = host
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "colo=") {
			colo = strings.TrimSpace(strings.TrimPrefix(line, "colo="))
		}
		if strings.HasPrefix(line, "loc=") {
			loc = strings.TrimSpace(strings.TrimPrefix(line, "loc="))
		}
	}
	return colo, loc
}

// ipOnly returns the host portion of an "ip:port" / "[ipv6]:port" endpoint.
func ipOnly(endpoint string) string {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return ""
	}
	return host
}

// buildColoMap probes /cdn-cgi/trace for up to maxIPs distinct responders
// (deduped by IP, fastest first since results arrive latency-sorted) and returns
// an IP -> {colo, loc} map. It only reads results, never mutates them, so it is
// safe to run lock-free against a published slice. Bounded + concurrent so it
// stays off the Phase-1 dial hot path regardless of how many IPs responded.
func buildColoMap(ctx context.Context, results []CleanIPResult, sni string, maxIPs, concurrency int) map[string][2]string {
	if maxIPs <= 0 {
		maxIPs = 150
	}
	if concurrency <= 0 {
		concurrency = 48
	}

	type target struct{ ip, endpoint string }
	var targets []target
	seen := make(map[string]bool)
	for _, r := range results {
		if !r.Success {
			continue
		}
		ip := ipOnly(r.Endpoint)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		targets = append(targets, target{ip: ip, endpoint: r.Endpoint})
		if len(targets) >= maxIPs {
			break
		}
	}

	coloMap := make(map[string][2]string, len(targets))
	if len(targets) == 0 {
		return coloMap
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(int64(concurrency))
	for _, tgt := range targets {
		wg.Add(1)
		go func(t target) {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				return
			}
			defer sem.Release(1)
			colo, loc := probeCloudflareTrace(ctx, t.endpoint, sni, 2*time.Second)
			if colo == "" && loc == "" {
				return
			}
			mu.Lock()
			coloMap[t.ip] = [2]string{colo, loc}
			mu.Unlock()
		}(tgt)
	}
	wg.Wait()
	return coloMap
}

// applyColo writes the colo/loc from a coloMap onto every matching result.
// Callers must hold job.mu when results is the published job slice.
func applyColo(results []CleanIPResult, coloMap map[string][2]string) {
	if len(coloMap) == 0 {
		return
	}
	for i := range results {
		if cl, ok := coloMap[ipOnly(results[i].Endpoint)]; ok {
			results[i].Colo = cl[0]
			results[i].Loc = cl[1]
		}
	}
}

// summarizeFailure collapses a raw Phase-2 error into a short, actionable
// category for the UI's failure-reason breakdown. It turns a wall of identical
// low-level strings into "12× xray startup timeout" so the user can tell a
// broken xray from a routing/Host problem from a too-tight timeout.
func summarizeFailure(err string) string {
	e := strings.ToLower(err)
	switch {
	case strings.Contains(e, "startup timeout"):
		return "xray didn't come up in time (slow start or crash)"
	case strings.Contains(e, "start xray"), strings.Contains(e, "xray config"):
		return "xray failed to launch (check the xray binary / config)"
	case strings.Contains(e, "no uuid"), strings.Contains(e, "empty uuid"), strings.Contains(e, "empty address"), strings.Contains(e, "invalid port"):
		return "incomplete config (UUID/address/port missing)"
	case strings.Contains(e, "socks connect"):
		return "couldn't reach xray's local SOCKS port"
	case strings.Contains(e, "socks5"):
		return "tunnel handshake failed (proxy refused the connection)"
	case strings.Contains(e, "forcibly closed"), strings.Contains(e, "connection reset"), strings.Contains(e, "reset by peer"), strings.Contains(e, "unexpected eof"):
		return "connection reset mid-handshake (likely ISP/DPI filtering or a dead origin)"
	case strings.Contains(e, "connection refused"):
		return "origin refused the connection (dead endpoint or wrong port)"
	case strings.Contains(e, "http write"), strings.Contains(e, "http read"):
		return "no usable response through the tunnel (timeout / reset)"
	case strings.HasPrefix(e, "http "):
		return err + " (Cloudflare reached but didn't route — check SNI/Host)"
	case strings.Contains(e, "cancelled"):
		return "cancelled"
	default:
		return err
	}
}

// withXrayCause formats a Phase-2 transport error, appending the deepest cause
// from xray's log when one is available so the failure is self-explanatory.
func withXrayCause(stage string, err error, logPath string) string {
	if cause := extractXrayError(logPath); cause != "" {
		return fmt.Sprintf("%s: %v | xray: %s", stage, err, cause)
	}
	return fmt.Sprintf("%s: %v", stage, err)
}

// extractXrayError mines xray's run log for the concrete reason a Phase-2 tunnel
// failed. The error we observe locally is always a generic "http read timeout"
// once the SOCKS CONNECT is (optimistically) accepted; the real cause — a TLS
// reset, a refused dial, a routing rejection — only lives in xray's log. Folding
// it into the result turns "no usable response" into something actionable (e.g.
// distinguishing ISP/DPI filtering from a dead origin). Returns "" when nothing
// useful is found.
func extractXrayError(logPath string) string {
	data, err := os.ReadFile(logPath)
	if err != nil || len(data) == 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		low := strings.ToLower(l)
		if !strings.Contains(low, "[error]") && !strings.Contains(low, "[warning]") {
			continue
		}
		if !strings.Contains(low, "failed") && !strings.Contains(low, "forcibly closed") &&
			!strings.Contains(low, "refused") && !strings.Contains(low, "reset") &&
			!strings.Contains(low, "eof") && !strings.Contains(low, "rejected") {
			continue
		}
		// Keep only the deepest cause (xray chains them with "> ").
		if idx := strings.LastIndex(l, "> "); idx >= 0 {
			l = l[idx+2:]
		}
		l = strings.TrimSpace(l)
		if len(l) > 160 {
			l = l[:160] + "…"
		}
		return l
	}
	return ""
}

// socks204Probe runs the SOCKS5 + GET /generate_204 check against an already-up
// local SOCKS port and returns the result. It is the shared core of the pooled
// validateBatchWithXray path, keeping the success criterion (exact 204) and the
// xray-log enrichment in one place. The caller owns the xray process and its
// lifecycle; logPath points at that process's run log for failure-cause extraction.
func socks204Probe(ctx context.Context, endpoint string, socksPort int, timeout time.Duration, logPath string) CleanIPResult {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	start := time.Now()

	var d net.Dialer
	dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
	defer dialCancel()
	conn, err := d.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("socks connect: %v", err)}
	}
	defer conn.Close()

	// Single deadline covering both SOCKS5 handshake and HTTP round-trip.
	conn.SetDeadline(time.Now().Add(timeout))

	if err := socks5Handshake(conn, "www.gstatic.com", 80); err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("socks5: %v", err)}
	}

	req := "GET /generate_204 HTTP/1.1\r\nHost: www.gstatic.com\r\nConnection: close\r\nUser-Agent: Mozilla/5.0\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: withXrayCause("http write", err, logPath)}
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: withXrayCause("http read", err, logPath)}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 204 {
		return CleanIPResult{Endpoint: endpoint, Success: true, Latency: time.Since(start)}
	}
	return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// validateBatchWithXray validates a whole batch of endpoints through a SINGLE
// xray process: one SOCKS inbound + outbound + routing rule per endpoint (see
// BuildXrayJSONBatch). This trades N process spawns (each with its own exec +
// up-to-4s port-wait) for one, the dominant Phase-2 cost. Each endpoint is still
// probed independently (its own port, its own 204 check), and the probes run
// concurrently against the shared process. Results are aligned to the input
// endpoints slice. Honors ctx cancellation between and during probes.
func validateBatchWithXray(ctx context.Context, cfg *ProxyConfig, endpoints []string, xrayPath string, basePort int, timeout time.Duration) []CleanIPResult {
	results := make([]CleanIPResult, len(endpoints))
	for i, ep := range endpoints {
		results[i] = CleanIPResult{Endpoint: ep, Error: "not run"}
	}

	select {
	case <-ctx.Done():
		for i := range results {
			results[i].Error = "cancelled"
		}
		return results
	default:
	}

	configPath, ports, err := cfg.BuildXrayJSONBatch(endpoints, basePort)
	if err != nil {
		for i := range results {
			results[i].Error = fmt.Sprintf("xray config: %v", err)
		}
		return results
	}
	defer os.RemoveAll(filepath.Dir(configPath))

	cmd := exec.Command(xrayPath, "run", "-c", configPath)
	cmd.Dir = filepath.Dir(configPath)
	stderrPath := filepath.Join(filepath.Dir(configPath), "stderr.log")
	if f, ferr := os.Create(stderrPath); ferr == nil {
		cmd.Stderr = f
		defer f.Close()
	}

	if err := cmd.Start(); err != nil {
		for i := range results {
			results[i].Error = fmt.Sprintf("start xray: %v", err)
		}
		return results
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	// Wait for the LAST inbound port to come up. xray binds inbounds in order, so
	// once the highest port accepts connections the whole batch is listening.
	lastAddr := fmt.Sprintf("127.0.0.1:%d", ports[len(ports)-1])
	startupDeadline := time.Now().Add(6 * time.Second)
	started := false
	for time.Now().Before(startupDeadline) {
		select {
		case <-ctx.Done():
			for i := range results {
				results[i].Error = "cancelled"
			}
			return results
		default:
		}
		conn, derr := net.DialTimeout("tcp", lastAddr, 300*time.Millisecond)
		if derr == nil {
			conn.Close()
			started = true
			break
		}
		time.Sleep(80 * time.Millisecond)
	}
	if !started {
		for i := range results {
			results[i].Error = "xray startup timeout"
		}
		return results
	}

	logPath := filepath.Join(filepath.Dir(configPath), "xray.log")

	// Probe every endpoint concurrently against the shared process.
	var wg sync.WaitGroup
	for i := range endpoints {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results[idx] = CleanIPResult{Endpoint: endpoints[idx], Error: "cancelled"}
				return
			default:
			}
			results[idx] = socks204Probe(ctx, endpoints[idx], ports[idx], timeout, logPath)
		}(i)
	}
	wg.Wait()
	return results
}

func runCleanScan(job *CleanIPJob, xrayPath string) {
	defer scheduleCleanJobCleanup(job.ID)

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	go func() {
		select {
		case <-job.Cancel:
			ctxCancel()
		case <-ctx.Done():
		}
	}()

	phase1Timeout := 3 * time.Second
	phase2Timeout := 5 * time.Second
	// User-configurable per-probe TCP dial timeout for phase 1 (the reachability
	// probe). 0 keeps the default. Validated/clamped server-side.
	if job.TimeoutMs > 0 {
		phase1Timeout = time.Duration(job.TimeoutMs) * time.Millisecond
	}
	// User-configurable per-attempt deadline for the phase-2 xray validation
	// (SOCKS5 handshake + 204 round-trip). 0 keeps the default.
	if job.Phase2TimeoutMs > 0 {
		phase2Timeout = time.Duration(job.Phase2TimeoutMs) * time.Millisecond
	}

	p1Probes := job.Phase1Probes
	if p1Probes <= 0 {
		p1Probes = 500
	}
	p2Probes := job.Phase2Probes
	if p2Probes <= 0 {
		p2Probes = 12
	}

	job.mu.Lock()
	job.Status = "running-phase1"
	job.Phase1Total = len(job.Endpoints)
	job.Phase1Progress = 0
	job.Phase2Total = 0
	job.Phase2Progress = 0
	job.mu.Unlock()

	phase1Results := runCleanPhase1TCP(ctx, job.Endpoints, phase1Timeout, job.Cancel, job, p1Probes, job.StopAfter)

	select {
	case <-job.Cancel:
		job.mu.Lock()
		job.Status = "cancelled"
		job.mu.Unlock()
		return
	default:
	}

	job.mu.Lock()
	job.Phase1Results = phase1Results
	job.mu.Unlock()

	// Enrich the fastest responders with their Cloudflare colo/country in a
	// bounded, concurrent pass — kept off the Phase-1 dial loop. Covers at least
	// the Phase-2 candidates plus a display buffer, and is reused for Phase 2.
	coloCap := job.Phase2Count
	if coloCap < 150 {
		coloCap = 150
	}
	// Use the config's SNI for the direct edge trace — it's a CF-proxied hostname
	// the user's working config proves is unblocked, unlike the well-known
	// speed.cloudflare.com SNI that DPI resets. Empty in one-phase mode (no
	// config), where probeCloudflareTrace falls back to the default SNI.
	coloSNI := ""
	if job.Config != nil {
		coloSNI = job.Config.SNI
	}
	coloMap := buildColoMap(ctx, phase1Results, coloSNI, coloCap, 48)
	job.mu.Lock()
	applyColo(job.Phase1Results, coloMap)
	job.mu.Unlock()

	// Nearby scan: expand around working phase 1 results
	var nearbyPhase1Results []CleanIPResult
	if job.NearbyScan && len(phase1Results) > 0 {
		nearbyCount := job.NearbyCount
		if nearbyCount <= 0 {
			nearbyCount = 10
		}
		// Expand around every working IP found in Phase 1 (not just the
		// fastest few). generateNearbyIPs caps the total it emits so this
		// stays bounded even with many responders.
		topForNearby := phase1Results
		// use job's selected ports for nearby scan
		nearbyPorts := job.ScanPorts
		if len(nearbyPorts) == 0 {
			nearbyPorts = []int{443}
			if cfg := job.Config; cfg != nil {
				nearbyPorts = []int{cfg.Port}
			}
		}
		nearbyIPs := generateNearbyIPs(topForNearby, nearbyCount, nearbyPorts)
		if len(nearbyIPs) > 0 {
			job.mu.Lock()
			savedPhase1Total := job.Phase1Total
			savedPhase1Progress := job.Phase1Progress
			job.mu.Unlock()

			nearbyPhase1Results = runCleanPhase1TCP(ctx, nearbyIPs, phase1Timeout, job.Cancel, nil, p1Probes, 0)

			// restore job progress to original (nearby is extra)
			job.mu.Lock()
			job.Phase1Total = savedPhase1Total
			job.Phase1Progress = savedPhase1Progress
			job.NearbyPhase1Results = nearbyPhase1Results
			job.mu.Unlock()

			nearbyColo := buildColoMap(ctx, nearbyPhase1Results, coloSNI, coloCap, 48)
			for k, v := range nearbyColo {
				coloMap[k] = v
			}
			job.mu.Lock()
			applyColo(job.NearbyPhase1Results, nearbyColo)
			job.mu.Unlock()
		}
	}

	if job.SkipPhase2 {
		job.mu.Lock()
		job.Status = "done"
		job.mu.Unlock()
		return
	}

	job.mu.Lock()
	topN := job.Phase2Count
	job.mu.Unlock()

	tcpResults := phase1Results
	if len(tcpResults) > topN {
		tcpResults = tcpResults[:topN]
	}

	// also run phase 2 on nearby results if present
	var nearbyTcpResults []CleanIPResult
	if len(nearbyPhase1Results) > 0 {
		nearbyTcpResults = nearbyPhase1Results
		if len(nearbyTcpResults) > topN {
			nearbyTcpResults = nearbyTcpResults[:topN]
		}
	}

	job.mu.Lock()
	job.Phase2Total = len(tcpResults) + len(nearbyTcpResults)
	job.Phase2Progress = 0
	job.Status = "running-phase2"
	job.mu.Unlock()

	if len(tcpResults) == 0 {
		job.mu.Lock()
		job.Status = "done"
		job.mu.Unlock()
		return
	}

	// For the HTTP validation probe, mux/xudp can interfere with the single
	// test request (GET /generate_204). Strip PacketEncoding so xray never
	// enables mux concurrency during Phase 2.
	validationCfg := *job.Config
	validationCfg.PacketEncoding = ""

	// Phase 2 validates endpoints in BATCHES: a single xray process serves a whole
	// batch (one SOCKS inbound + outbound + routing rule per endpoint — see
	// BuildXrayJSONBatch), instead of one process per endpoint. The old per-probe
	// spawn (exec + up-to-4s port wait) was the dominant Phase-2 cost; batching
	// collapses it by the batch factor. p2Probes (the old per-endpoint concurrency
	// knob) now bounds how many endpoints are in flight at once, mapped onto
	// (batchSize × concurrentBatches) so parallelism is preserved with far fewer
	// process launches.
	const phase2BatchSize = 16
	concurrentBatches := (p2Probes + phase2BatchSize - 1) / phase2BatchSize
	if concurrentBatches < 1 {
		concurrentBatches = 1
	}

	allocPortBase := func() int {
		// Each batch gets a non-overlapping window of phase2BatchSize ports.
		// Stride monotonically and wrap inside a wide band well below the OS
		// ephemeral range; a process is always killed before its window could be
		// reused. Offset 20799 keeps clear of the WARP path's +10799 band. The
		// counter is process-global so concurrent clean scans don't collide.
		n := int(cleanSocksPortBase.Add(1))
		return 20799 + (n*phase2BatchSize)%20000
	}

	// runPhase2Batches splits endpoints into batches, runs up to concurrentBatches
	// xray processes at once, retries an endpoint's failures once in a follow-up
	// batch (mirroring the old 2-attempt behavior without paying 2× on the common
	// success path), and calls onBatch with each completed batch's results.
	// Returns true if the job was cancelled mid-run.
	runPhase2Batches := func(endpoints []string, onBatch func([]CleanIPResult)) bool {
		var batches [][]string
		for i := 0; i < len(endpoints); i += phase2BatchSize {
			end := i + phase2BatchSize
			if end > len(endpoints) {
				end = len(endpoints)
			}
			batches = append(batches, endpoints[i:end])
		}

		sem := semaphore.NewWeighted(int64(concurrentBatches))
		var wg sync.WaitGroup
		var cancelled atomic.Bool

		for _, b := range batches {
			select {
			case <-job.Cancel:
				cancelled.Store(true)
			default:
			}
			if cancelled.Load() {
				break
			}

			wg.Add(1)
			go func(batch []string) {
				defer wg.Done()
				if err := sem.Acquire(ctx, 1); err != nil {
					return
				}
				defer sem.Release(1)

				res := validateBatchWithXray(ctx, &validationCfg, batch, xrayPath, allocPortBase(), phase2Timeout)

				// Retry the ones that failed, once, in a fresh batch — a single
				// dropped TLS/handshake under load shouldn't condemn an IP.
				var retryIdx []int
				var retryEps []string
				for i, r := range res {
					if !r.Success {
						retryIdx = append(retryIdx, i)
						retryEps = append(retryEps, batch[i])
					}
				}
				if len(retryEps) > 0 {
					select {
					case <-ctx.Done():
					default:
						rres := validateBatchWithXray(ctx, &validationCfg, retryEps, xrayPath, allocPortBase(), phase2Timeout)
						for j, rr := range rres {
							if rr.Success {
								rr.Attempts = 2
								res[retryIdx[j]] = rr
							} else {
								res[retryIdx[j]].Attempts = 2
							}
						}
					}
				}

				for i := range res {
					if res[i].Attempts == 0 {
						res[i].Attempts = 1
					}
					if res[i].Success && res[i].Passes == 0 {
						res[i].Passes = 1
						res[i].Best = res[i].Latency
					}
				}
				onBatch(res)
			}(b)
		}

		wg.Wait()
		return cancelled.Load()
	}

	var mu sync.Mutex
	phase2Results := make([]CleanIPResult, 0, len(tcpResults))

	mainEps := make([]string, len(tcpResults))
	for i, pr := range tcpResults {
		mainEps[i] = pr.Endpoint
	}

	cancelledMain := runPhase2Batches(mainEps, func(res []CleanIPResult) {
		mu.Lock()
		phase2Results = append(phase2Results, res...)
		progress := len(phase2Results)
		snapshot := make([]CleanIPResult, progress)
		copy(snapshot, phase2Results)
		mu.Unlock()

		job.mu.Lock()
		job.Phase2Results = snapshot
		job.Phase2Progress = progress
		job.mu.Unlock()
	})

	if cancelledMain {
		sortCleanIPResults(phase2Results)
		job.mu.Lock()
		job.Phase2Results = phase2Results
		job.Phase2Progress = len(phase2Results)
		job.Status = "cancelled"
		job.mu.Unlock()
		return
	}

	// Phase 2 for nearby results
	var nearbyPhase2Results []CleanIPResult
	if len(nearbyTcpResults) > 0 {
		nearbyEps := make([]string, len(nearbyTcpResults))
		for i, pr := range nearbyTcpResults {
			nearbyEps[i] = pr.Endpoint
		}
		var nmu sync.Mutex
		cancelledNearby := runPhase2Batches(nearbyEps, func(res []CleanIPResult) {
			nmu.Lock()
			nearbyPhase2Results = append(nearbyPhase2Results, res...)
			nmu.Unlock()
			job.mu.Lock()
			job.Phase2Progress += len(res)
			job.mu.Unlock()
		})
		if cancelledNearby {
			job.mu.Lock()
			job.Status = "cancelled"
			job.mu.Unlock()
			return
		}
		sortCleanIPResults(nearbyPhase2Results)
	}

	select {
	case <-job.Cancel:
		job.mu.Lock()
		job.Status = "cancelled"
		job.mu.Unlock()
		return
	default:
	}

	sortCleanIPResults(phase2Results)

	job.mu.Lock()
	applyColo(phase2Results, coloMap)
	applyColo(nearbyPhase2Results, coloMap)
	job.Phase2Results = phase2Results
	job.NearbyPhase2Results = nearbyPhase2Results
	job.Phase2Progress = len(phase2Results) + len(nearbyPhase2Results)
	job.Status = "done"
	job.mu.Unlock()
}

func (c *ProxyConfig) GenerateExport(endpoints []string) []string {
	urls := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		clone := c.WithEndpoint(ep)
		urls = append(urls, clone.GenerateShareURL())
	}
	return urls
}
