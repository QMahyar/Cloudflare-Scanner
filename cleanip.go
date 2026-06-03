package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

func (g *CleanIPGenerator) GenerateIPs(count int, useIPv4, useIPv6 bool, port int) []string {
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

	for attempts := 0; len(endpoints) < v4Count && attempts < v4Count*20; attempts++ {
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
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", ip, port))
	}

	for attempts := 0; len(endpoints) < v4Count+v6Count && attempts < v6Count*20; attempts++ {
		cidr := v6CIDRList[g.rng.Intn(len(v6CIDRList))]
		ip := randomIPv6InCIDR(cidr, g.rng)
		if seen[ip] {
			continue
		}
		seen[ip] = true
		endpoints = append(endpoints, fmt.Sprintf("[%s]:%d", ip, port))
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

func generateNearbyIPs(working []CleanIPResult, countPerIP int, port int) []string {
	rng := rand.New(rand.NewSource(rand.Int63()))
	seen := make(map[string]bool)
	var result []string

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

		if 		ip4 := ip.To4(); ip4 != nil {
			// /24 subnet: x.y.z.0
			base := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8
			for attempts := 0; len(result) < len(working)*countPerIP && attempts < countPerIP*50; attempts++ {
				offset := uint32(rng.Intn(256))
				ipU32 := base | offset
				s := fmt.Sprintf("%d.%d.%d.%d", byte(ipU32>>24), byte(ipU32>>16), byte(ipU32>>8), byte(ipU32))
				if seen[s] {
					continue
				}
				seen[s] = true
				result = append(result, fmt.Sprintf("%s:%d", s, port))
				if len(result) >= len(working)*countPerIP {
					return result
				}
			}
		} else {
			// IPv6 /64 subnet: randomize last 64 bits
			for attempts := 0; len(result) < len(working)*countPerIP && attempts < countPerIP*50; attempts++ {
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
				result = append(result, fmt.Sprintf("[%s]:%d", s, port))
				if len(result) >= len(working)*countPerIP {
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
}

type CleanIPJob struct {
	ID                string
	Status            string
	Progress          int
	Total             int
	Phase1Progress    int
	Phase1Total       int
	Phase2Progress    int
	Phase2Total       int
	Config            *ProxyConfig
	Endpoints         []string
	Phase1Results     []CleanIPResult
	Phase2Results     []CleanIPResult
	Phase2Count       int
	SkipPhase2        bool
	NearbyScan        bool
	NearbyCount       int
	Phase1Probes      int
	Phase2Probes      int
	NearbyPhase1Results []CleanIPResult
	NearbyPhase2Results []CleanIPResult
	Cancel              chan struct{}
	cancelOnce          sync.Once
	mu                  sync.Mutex
}

func (j *CleanIPJob) stop() {
	j.cancelOnce.Do(func() { close(j.Cancel) })
}

func runCleanPhase1TCP(endpoints []string, timeout time.Duration, cancel chan struct{}, job *CleanIPJob, concurrency int) []CleanIPResult {
	var mu sync.Mutex
	var wg sync.WaitGroup
	if concurrency <= 0 {
		concurrency = 500
	}
	sem := make(chan struct{}, concurrency)
	results := make([]CleanIPResult, 0, len(endpoints))

	for i, ep := range endpoints {
		select {
		case <-cancel:
			return results
		default:
		}

		wg.Add(1)
		go func(endpoint string, idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			conn, err := net.DialTimeout("tcp", endpoint, timeout)
			if err != nil {
				if job != nil {
					job.mu.Lock()
					job.Phase1Progress++
					job.mu.Unlock()
				}
				return
			}
			conn.Close()
			latency := time.Since(start)

			result := CleanIPResult{
				Endpoint: endpoint,
				Latency:  latency,
				Success:  true,
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if job != nil {
				job.mu.Lock()
				job.Phase1Results = append(job.Phase1Results, result)
				job.Phase1Progress++
				job.mu.Unlock()
			}
		}(ep, i)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

	return results
}

func validateWithXray(cfg *ProxyConfig, endpoint string, xrayPath string, socksPort int, timeout time.Duration) CleanIPResult {
	if cfg.UUID == "" {
		return CleanIPResult{Endpoint: endpoint, Error: "no UUID in config"}
	}

	configPath, err := cfg.BuildXrayJSON(endpoint, socksPort)
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("xray config: %v", err)}
	}

	cmd := exec.Command(xrayPath, "run", "-c", configPath)
	cmd.Dir = filepath.Dir(configPath)

	stderrPath := filepath.Join(filepath.Dir(configPath), "stderr.log")
	f, err := os.Create(stderrPath)
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("create stderr log: %v", err)}
	}
	cmd.Stderr = f

	if err := cmd.Start(); err != nil {
		f.Close()
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("start xray: %v", err)}
	}
	f.Close() // child process holds its own handle; release ours
	defer func() {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	deadline := time.Now().Add(4 * time.Second)
	started := false
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			conn.Close()
			started = true
			break
		}
		time.Sleep(80 * time.Millisecond)
	}

	if !started {
		return CleanIPResult{Endpoint: endpoint, Error: "xray startup timeout"}
	}

	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("socks connect: %v", err)}
	}

	if err := socks5Handshake(conn, "www.gstatic.com", 80); err != nil {
		conn.Close()
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("socks5: %v", err)}
	}

	req := "GET /generate_204 HTTP/1.1\r\nHost: www.gstatic.com\r\nConnection: close\r\nUser-Agent: Mozilla/5.0\r\n\r\n"
	conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("http write: %v", err)}
	}

	resp, err := httpReadResponse(conn)
	conn.Close()
	if err != nil {
		return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("http read: %v", err)}
	}

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		latency := time.Since(start)
		return CleanIPResult{Endpoint: endpoint, Success: true, Latency: latency}
	}

	return CleanIPResult{Endpoint: endpoint, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func httpReadResponse(conn net.Conn) (*httpSimpleResponse, error) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	resp := &httpSimpleResponse{}
	data := string(buf[:n])
	parts := strings.SplitN(data, "\r\n", 2)
	if len(parts) > 0 {
		statusParts := strings.SplitN(parts[0], " ", 3)
		if len(statusParts) >= 2 {
			fmt.Sscanf(statusParts[1], "%d", &resp.StatusCode)
		}
	}
	return resp, nil
}

type httpSimpleResponse struct {
	StatusCode int
}

func runCleanScan(job *CleanIPJob, xrayPath string) {
	phase1Timeout := 3 * time.Second
	phase2Timeout := 5 * time.Second

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

	phase1Results := runCleanPhase1TCP(job.Endpoints, phase1Timeout, job.Cancel, job, p1Probes)

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

	// Nearby scan: expand around working phase 1 results
	var nearbyPhase1Results []CleanIPResult
	if job.NearbyScan && len(phase1Results) > 0 {
		nearbyCount := job.NearbyCount
		if nearbyCount <= 0 {
			nearbyCount = 10
		}
		// take top 10 working IPs to expand around
		topForNearby := phase1Results
		if len(topForNearby) > 10 {
			topForNearby = topForNearby[:10]
		}
		// determine port from first working result
		port := 443
		if len(topForNearby) > 0 {
			if cfg := job.Config; cfg != nil {
				port = cfg.Port
			}
		}
		nearbyIPs := generateNearbyIPs(topForNearby, nearbyCount, port)
		if len(nearbyIPs) > 0 {
			job.mu.Lock()
			savedPhase1Total := job.Phase1Total
			savedPhase1Progress := job.Phase1Progress
			job.mu.Unlock()

			nearbyPhase1Results = runCleanPhase1TCP(nearbyIPs, phase1Timeout, job.Cancel, nil, p1Probes)

			// restore job progress to original (nearby is extra)
			job.mu.Lock()
			job.Phase1Total = savedPhase1Total
			job.Phase1Progress = savedPhase1Progress
			job.NearbyPhase1Results = nearbyPhase1Results
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
	job.Phase2Total = len(tcpResults)
	job.Phase2Progress = 0
	job.Status = "running-phase2"
	job.mu.Unlock()

	if len(tcpResults) == 0 {
		job.mu.Lock()
		job.Status = "done"
		job.mu.Unlock()
		return
	}

	var portCounter atomic.Int32
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, p2Probes)
	phase2Results := make([]CleanIPResult, 0, len(tcpResults))
	var nearbyPhase2Results []CleanIPResult
	if len(nearbyTcpResults) > 0 {
		nearbyPhase2Results = make([]CleanIPResult, 0, len(nearbyTcpResults))
	}

	for _, pr := range tcpResults {
		select {
		case <-job.Cancel:
			sort.Slice(phase2Results, func(i, j int) bool {
				return phase2Results[i].Latency < phase2Results[j].Latency
			})
			mu.Lock()
			job.mu.Lock()
			job.Phase2Results = phase2Results
			job.Phase2Progress = len(phase2Results)
			job.Status = "cancelled"
			job.mu.Unlock()
			mu.Unlock()
			return
		default:
		}

		wg.Add(1)
		go func(ep string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			socksPort := int(portCounter.Add(1)) + 20799
			result := validateWithXray(job.Config, ep, xrayPath, socksPort, phase2Timeout)

			mu.Lock()
			phase2Results = append(phase2Results, result)
			progress := len(phase2Results)
			mu.Unlock()

			mu.Lock()
			job.mu.Lock()
			job.Phase2Results = make([]CleanIPResult, len(phase2Results))
			copy(job.Phase2Results, phase2Results)
			job.Phase2Progress = progress
			job.mu.Unlock()
			mu.Unlock()
		}(pr.Endpoint)
	}

	wg.Wait()

	// Phase 2 for nearby results
	if len(nearbyTcpResults) > 0 {
		var nearbyWg sync.WaitGroup
		for _, pr := range nearbyTcpResults {
			select {
			case <-job.Cancel:
				job.mu.Lock()
				job.Status = "cancelled"
				job.mu.Unlock()
				return
			default:
			}

			nearbyWg.Add(1)
			go func(ep string) {
				defer nearbyWg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				socksPort := int(portCounter.Add(1)) + 20799
				result := validateWithXray(job.Config, ep, xrayPath, socksPort, phase2Timeout)

				mu.Lock()
				nearbyPhase2Results = append(nearbyPhase2Results, result)
				mu.Unlock()
			}(pr.Endpoint)
		}
		nearbyWg.Wait()

		sort.Slice(nearbyPhase2Results, func(i, j int) bool {
			return nearbyPhase2Results[i].Latency < nearbyPhase2Results[j].Latency
		})
	}

	select {
	case <-job.Cancel:
		job.mu.Lock()
		job.Status = "cancelled"
		job.mu.Unlock()
		return
	default:
	}

	sort.Slice(phase2Results, func(i, j int) bool {
		return phase2Results[i].Latency < phase2Results[j].Latency
	})

	job.mu.Lock()
	job.Phase2Results = phase2Results
	job.NearbyPhase2Results = nearbyPhase2Results
	job.Phase2Progress = len(phase2Results)
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
