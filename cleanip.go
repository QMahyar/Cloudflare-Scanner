package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// cleanSocksPortBase hands out non-overlapping SOCKS port windows for clean-IP
// Phase-2 batches. It is process-global (not per-job) so two clean scans running
// at once can't allocate the same window and fight over the same xray ports.
var cleanSocksPortBase atomic.Int32

type CleanIPResult struct {
	Endpoint string
	Latency  time.Duration
	Success  bool
	Error    string
	Attempts int
	Passes   int
	Best     time.Duration
	Jitter   time.Duration
	Loss     float64 // packet-loss % from the quality pass (0–100)
	Score    int     // 0–100 quality rank (latency+jitter+loss)
	H3       bool    // endpoint answered an HTTP/3 (QUIC) probe
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

// releaseCleanJobInputs drops pure input slices that are no longer needed after
// a job reaches a terminal status. Phase results stay until jobTTL cleanup so
// the UI can still poll them. Caller must hold job.mu.
func releaseCleanJobInputs(job *CleanIPJob) {
	job.Endpoints = nil
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
		releaseCleanJobInputs(job)
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
	// Enrich the fastest responders with colo, quality (loss/jitter/score), and
	// h3 reachability — three independent, bounded, top-N passes. They're run
	// concurrently so the QUIC handshake cost hides under the colo/quality work
	// instead of adding to the wall clock. Quality is capped tighter than colo
	// (k dials per IP); h3 tighter still (a full QUIC handshake per IP).
	qualityCap := coloCap
	if qualityCap > 96 {
		qualityCap = 96
	}
	h3Cap := qualityCap
	if h3Cap > 64 {
		h3Cap = 64
	}
	// colo + quality run concurrently (both important and usually fast). The h3
	// pass runs AFTER, never alongside: a full QUIC handshake is the most
	// expensive probe, and on UDP-blocked networks every one times out — run
	// concurrently it starved the colo TLS handshakes and left the Colo column
	// empty. Keeping h3 last also lets buildH3Map early-bail when QUIC is blocked.
	var coloMap map[string][2]string
	var qMap map[string]qualitySample
	var ewg sync.WaitGroup
	ewg.Add(2)
	go func() { defer ewg.Done(); coloMap = buildColoMap(ctx, phase1Results, coloSNI, coloCap, 48) }()
	go func() { defer ewg.Done(); qMap = measureQuality(ctx, phase1Results, 4, qualityCap, 48, phase1Timeout) }()
	ewg.Wait()
	h3Map := buildH3Map(ctx, phase1Results, coloSNI, h3Cap, 24, 3*time.Second)
	job.mu.Lock()
	applyColo(job.Phase1Results, coloMap)
	applyQuality(job.Phase1Results, qMap)
	applyH3(job.Phase1Results, h3Map)
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
			nearbyQ := measureQuality(ctx, nearbyPhase1Results, 4, qualityCap, 48, phase1Timeout)
			for k, v := range nearbyQ {
				qMap[k] = v // merge so nearby Phase-2 results inherit the sample by IP
			}
			nearbyH3 := buildH3Map(ctx, nearbyPhase1Results, coloSNI, h3Cap, 24, 3*time.Second)
			for k, v := range nearbyH3 {
				h3Map[k] = v
			}
			job.mu.Lock()
			applyColo(job.NearbyPhase1Results, nearbyColo)
			applyQuality(job.NearbyPhase1Results, nearbyQ)
			applyH3(job.NearbyPhase1Results, nearbyH3)
			job.mu.Unlock()
		}
	}

	if job.SkipPhase2 {
		job.mu.Lock()
		job.Status = "done"
		releaseCleanJobInputs(job)
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
		releaseCleanJobInputs(job)
		job.mu.Unlock()
		return
	}

	// Defensive: Phase 2 dereferences the config. The handler guarantees a config
	// whenever Phase 2 runs (it's only skipped in one-phase mode, which sets
	// SkipPhase2), but guard the nil so a future wiring change can't panic.
	if job.Config == nil {
		job.mu.Lock()
		job.Status = "done"
		releaseCleanJobInputs(job)
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
		// Stride monotonically and wrap inside a band that stays clear of the WARP
		// path's +10800 band below AND of Linux's default ephemeral range (32768+)
		// above — an inbound bind landing on an OS-assigned ephemeral source port
		// fails sporadically under load. 20799 + 11968 caps the top port at 32766
		// (< 32768). 11968 == 16*748 partitions cleanly so windows never straddle
		// the wrap; a process is always killed before reuse and concurrentBatches
		// (<=16) is far below 748. Process-global counter so concurrent clean scans
		// don't collide.
		n := int(cleanSocksPortBase.Add(1))
		return 20799 + (n*phase2BatchSize)%11968
	}

	// runPhase2Batches splits endpoints into batches, runs up to concurrentBatches
	// xray processes at once, retries an endpoint's failures once in a follow-up
	// batch (mirroring the old 2-attempt behavior without paying 2× on the common
	// success path), and calls onBatch with each completed batch's results.
	// Returns true if the job was cancelled mid-run.
	runPhase2Batches := func(endpoints []string, onBatch func([]CleanIPResult)) bool {
		return runBatches[CleanIPResult](
			ctx, job.Cancel, endpoints, phase2BatchSize, concurrentBatches, allocPortBase,
			func(batch []string, basePort int) []CleanIPResult {
				return validateBatchWithXray(ctx, &validationCfg, batch, xrayPath, basePort, phase2Timeout)
			},
			func(r CleanIPResult) bool { return r.Success },
			func(r *CleanIPResult) { r.Attempts = 2 },
			func(res []CleanIPResult) {
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
			},
		)
	}

	var mu sync.Mutex
	phase2Results := make([]CleanIPResult, 0, len(tcpResults))

	mainEps := make([]string, len(tcpResults))
	for i, pr := range tcpResults {
		mainEps[i] = pr.Endpoint
	}

	cancelledMain := runPhase2Batches(mainEps, func(res []CleanIPResult) {
		// Stop publishing the moment the job is cancelled: the caller is about to
		// snapshot the partial results and flip status to "cancelled", and late
		// batches arriving after that must not keep ticking progress/results.
		if ctx.Err() != nil {
			return
		}
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
		releaseCleanJobInputs(job)
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
			if ctx.Err() != nil {
				return
			}
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
			releaseCleanJobInputs(job)
			job.mu.Unlock()
			return
		}
		sortCleanIPResults(nearbyPhase2Results)
	}

	select {
	case <-job.Cancel:
		job.mu.Lock()
		job.Status = "cancelled"
		releaseCleanJobInputs(job)
		job.mu.Unlock()
		return
	default:
	}

	sortCleanIPResults(phase2Results)

	job.mu.Lock()
	applyColo(phase2Results, coloMap)
	applyColo(nearbyPhase2Results, coloMap)
	applyQuality(phase2Results, qMap)
	applyQuality(nearbyPhase2Results, qMap)
	applyH3(phase2Results, h3Map)
	applyH3(nearbyPhase2Results, h3Map)
	job.Phase2Results = phase2Results
	job.NearbyPhase2Results = nearbyPhase2Results
	job.Phase2Progress = len(phase2Results) + len(nearbyPhase2Results)
	job.Status = "done"
	releaseCleanJobInputs(job)
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
