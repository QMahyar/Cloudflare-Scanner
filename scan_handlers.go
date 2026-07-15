package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ScanJob struct {
	ID            string
	Status        string
	Progress      int
	Total         int
	Results       []ScanResult
	Config        *WarpConfig
	Endpoints     []string
	Noise         NoiseConfig
	OutCount      int
	Concurrency   int
	Attempts      int
	TimeoutMs     int
	StopAfter     int
	Successes     int
	TargetReached bool
	Cancel        chan struct{}
	cancelOnce    sync.Once
	mu            sync.Mutex
}

// stop closes the Cancel channel at most once, so concurrent stop requests
// cannot panic with "close of closed channel".
func (j *ScanJob) stop() {
	j.cancelOnce.Do(func() { close(j.Cancel) })
}

// releaseScanJobInputs drops pure inputs (endpoint slices and WARP config
// credentials) that are no longer needed after a job reaches a terminal status.
// Results stay until jobTTL cleanup so the UI can still poll them. Caller must
// hold job.mu.
func releaseScanJobInputs(job *ScanJob) {
	job.Endpoints = nil
	job.Config = nil
}

type scanRequest struct {
	Noise       bool        `json:"noise"`
	NoiseConfig NoiseConfig `json:"noiseConfig"`
	IPv4        bool        `json:"ipv4"`
	IPv6        bool        `json:"ipv6"`
	Count       int         `json:"count"`
	OutCount    int         `json:"outCount"`
	Concurrency int         `json:"concurrency"`
	Attempts    int         `json:"attempts"`
	TimeoutMs   int         `json:"timeoutMs"`
	StopAfter   int         `json:"stop_after"`
}

func handleScanStart(xrayPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		// Remove any multipart parts that spilled to temp files. Harmless today
		// (the body cap == the in-memory budget, so nothing spills), but correct
		// idiom and the safety net if either limit is ever raised independently.
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}

		var cfg *WarpConfig
		file, _, err := r.FormFile("config")
		if err == nil {
			defer file.Close()
			tmpFile, err := os.CreateTemp("", "warp-*.conf")
			if err != nil {
				jsonError(w, fmt.Sprintf("temp file: %v", err), 500)
				return
			}
			defer os.Remove(tmpFile.Name())
			if _, err := io.Copy(tmpFile, file); err != nil {
				tmpFile.Close()
				jsonError(w, fmt.Sprintf("read uploaded config: %v", err), 400)
				return
			}
			tmpFile.Close()
			cfg, err = ParseWarpConfig(tmpFile.Name())
			if err != nil {
				jsonError(w, fmt.Sprintf("%v", err), 400)
				return
			}
		}

		var req scanRequest
		jsonStr := r.FormValue("params")
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
				jsonError(w, fmt.Sprintf("invalid params: %v", err), 400)
				return
			}
		}

		if req.Count <= 0 {
			req.Count = 100
		}
		if req.Count > maxScanCount {
			req.Count = maxScanCount
		}
		if req.OutCount <= 0 {
			req.OutCount = 10
		}
		req.OutCount = clampInt(req.OutCount, 1, maxOutCount)
		if req.Concurrency > maxEndpointConcurrency {
			req.Concurrency = maxEndpointConcurrency
		}
		if req.Attempts <= 0 {
			req.Attempts = 2
		}
		if req.Attempts > 5 {
			req.Attempts = 5
		}
		if req.StopAfter < 0 {
			req.StopAfter = 0
		}
		if req.StopAfter > req.Count {
			req.StopAfter = req.Count
		}
		// 0 means "use the scanner default"; otherwise clamp to a sane window.
		if req.TimeoutMs > 0 {
			if req.TimeoutMs < 100 {
				req.TimeoutMs = 100
			}
			if req.TimeoutMs > 60000 {
				req.TimeoutMs = 60000
			}
		}
		if !req.IPv4 && !req.IPv6 {
			req.IPv4 = true
		}

		noise := NoiseConfig{}
		if req.Noise {
			noise = DefaultNoise()
			if req.NoiseConfig.Type != "" {
				noise = req.NoiseConfig
			}
			if noise.Count <= 0 {
				noise.Count = 5
			}
			if err := noise.Validate(); err != nil {
				jsonError(w, fmt.Sprintf("invalid noise: %v", err), 400)
				return
			}
		}

		endpoints := GenerateEndpoints(req.Count, req.IPv4, req.IPv6)

		job := &ScanJob{
			Status:      "running",
			Total:       len(endpoints),
			Config:      cfg,
			Endpoints:   endpoints,
			Noise:       noise,
			OutCount:    req.OutCount,
			Concurrency: req.Concurrency,
			Attempts:    req.Attempts,
			TimeoutMs:   req.TimeoutMs,
			StopAfter:   req.StopAfter,
			Cancel:      make(chan struct{}),
		}

		scanJobsMu.Lock()
		if countActiveScanJobsLocked() >= maxConcurrentJobs {
			scanJobsMu.Unlock()
			jsonError(w, "too many concurrent scans (max 2)", http.StatusTooManyRequests)
			return
		}
		jobCounter++
		jobID := fmt.Sprintf("job_%d", jobCounter)
		job.ID = jobID
		scanJobs[jobID] = job
		scanJobsMu.Unlock()

		go runScan(job, xrayPath)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": jobID, "total": fmt.Sprintf("%d", job.Total)})
	}
}

func runScan(job *ScanJob, xrayPath string) {
	defer scheduleScanJobCleanup(job.ID)

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	go func() {
		select {
		case <-job.Cancel:
			ctxCancel()
		case <-ctx.Done():
		}
	}()

	scanner := NewScanner(job.Config, job.Noise, xrayPath)
	if job.Config == nil {
		scanner.TCPOnly = true
	}
	if job.Concurrency > 0 {
		scanner.Concurrency = job.Concurrency
	}
	if job.TimeoutMs > 0 {
		scanner.Timeout = time.Duration(job.TimeoutMs) * time.Millisecond
	}

	// The noise/AmneziaWG fallback spawns an xray process per endpoint. Route it
	// through the pooled batch runner (one process per batch) instead. The default
	// native-handshake path and the TCP-only path are process-free and stay on the
	// per-endpoint loop below, untouched.
	if job.Config != nil && job.Noise.Type != "" {
		runScanNoiseBatched(ctx, job, scanner)
		return
	}

	// Fixed worker pool: a feeder pushes endpoints through a channel and
	// `concurrency` workers drain it. This caps live goroutines at `concurrency`
	// instead of spawning one parked goroutine per endpoint (a 100k-endpoint scan
	// otherwise parks 100k goroutines on the semaphore at once).
	concurrency := clampInt(scanner.Concurrency, 1, maxEndpointConcurrency)
	if len(job.Endpoints) < concurrency {
		concurrency = len(job.Endpoints)
	}

	work := make(chan string)
	go func() {
		defer close(work)
		for _, ep := range job.Endpoints {
			select {
			case <-ctx.Done():
				return
			case work <- ep:
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for endpoint := range work {
				if ctx.Err() != nil {
					return
				}
				result := scanner.testEndpointAttempts(ctx, endpoint, job.Attempts)

				// Drop a result that landed after the scan was cancelled or the
				// stop-after target was reached: the caller is about to publish a
				// final snapshot and late appenders must not keep ticking
				// Progress/Results on a "done"/"cancelled" job.
				if ctx.Err() != nil {
					return
				}

				job.mu.Lock()
				job.Results = append(job.Results, result)
				job.Progress = len(job.Results)
				if result.Success {
					job.Successes++
				}
				reached := job.StopAfter > 0 && job.Successes >= job.StopAfter
				if reached {
					job.TargetReached = true
				}
				job.mu.Unlock()

				// Auto-stop: enough working endpoints found — cancel the rest.
				if reached {
					job.stop()
				}
			}
		}()
	}
	wg.Wait()

	job.mu.Lock()
	// TargetReached is a success even though ctx is cancelled (stop() cancels it);
	// a bare ctx cancel without the target is a user stop.
	if !job.TargetReached && ctx.Err() != nil {
		job.Status = "cancelled"
	} else {
		job.Status = "done"
	}
	releaseScanJobInputs(job)
	results := append([]ScanResult(nil), job.Results...)
	job.mu.Unlock()

	sortScanResults(results)

	job.mu.Lock()
	job.Results = results
	job.Progress = len(results)
	job.mu.Unlock()
}

// noiseConcurrentBatches returns how many xray batch processes to run at once
// for a WARP noise scan: ceil(concurrency/batchSize), floored at 1 and capped
// at maxBatches when maxBatches > 0.
func noiseConcurrentBatches(concurrency, batchSize, maxBatches int) int {
	if batchSize < 1 {
		batchSize = 16
	}
	n := (concurrency + batchSize - 1) / batchSize
	if n < 1 {
		n = 1
	}
	if maxBatches > 0 && n > maxBatches {
		n = maxBatches
	}
	return n
}

// runScanNoiseBatched validates WARP endpoints through pooled xray processes (one
// process per batch) instead of one process per endpoint. It is used only for the
// noise/AmneziaWG fallback; the native-handshake and TCP-only paths are
// process-free and never reach here. Honors stop-after and cancellation, retries
// each batch's failures once, and keeps partial results on cancel.
func runScanNoiseBatched(ctx context.Context, job *ScanJob, scanner *Scanner) {
	const batchSize = 16
	concurrentBatches := noiseConcurrentBatches(scanner.Concurrency, batchSize, maxNoiseConcurrentBatches)

	allocPortBase := func() int {
		// Non-overlapping port windows in the WARP band (+10800), clear of the
		// clean-IP Phase-2 band (+20799). The counter is process-global so
		// concurrent WARP noise scans don't collide on the same ports.
		n := int(warpSocksPortBase.Add(1))
		// The span MUST be an exact multiple of batchSize, or successive windows
		// straddle the wrap boundary and overlap (9000 % 16 == 8 did exactly that,
		// colliding ports after ~562 batches). 8992 == 16*562 partitions cleanly and
		// stays clear of the clean-IP band at +20799. Mirrors cleanip.go's +20799/20000.
		return 10800 + (n*batchSize)%8992
	}

	runBatches[ScanResult](
		ctx, job.Cancel, job.Endpoints, batchSize, concurrentBatches, allocPortBase,
		func(batch []string, basePort int) []ScanResult {
			return scanner.scanBatchNoise(ctx, batch, basePort)
		},
		func(r ScanResult) bool { return r.Success },
		func(r *ScanResult) { r.Attempts = 2 },
		func(res []ScanResult) {
			job.mu.Lock()
			// If the job was cancelled by the user, drop this late batch's
			// results — the caller's terminal snapshot is authoritative and late
			// appends would only keep ticking Progress after Stop. A stop-after
			// completion (TargetReached) is a success path: keep the results so
			// we don't discard valid endpoints found by concurrent batches.
			if ctx.Err() != nil && !job.TargetReached {
				job.mu.Unlock()
				return
			}
			for i := range res {
				if res[i].Attempts == 0 {
					res[i].Attempts = 1
				}
				if res[i].Success && res[i].Passes == 0 {
					res[i].Passes = 1
					res[i].Best = res[i].Latency
				}
				if res[i].Success {
					res[i].Loss = lossPercent(res[i].Passes, res[i].Attempts)
					res[i].Score = qualityScore(res[i].Latency, res[i].Jitter, res[i].Loss)
				}
				job.Results = append(job.Results, res[i])
				if res[i].Success {
					job.Successes++
				}
			}
			job.Progress = len(job.Results)
			reached := job.StopAfter > 0 && job.Successes >= job.StopAfter
			if reached {
				job.TargetReached = true
			}
			job.mu.Unlock()

			if reached {
				job.stop()
			}
		},
	)

	job.mu.Lock()
	if !job.TargetReached {
		select {
		case <-ctx.Done():
			job.Status = "cancelled"
		default:
			job.Status = "done"
		}
	} else {
		job.Status = "done"
	}
	releaseScanJobInputs(job)
	results := append([]ScanResult(nil), job.Results...)
	job.mu.Unlock()

	sortScanResults(results)

	job.mu.Lock()
	job.Results = results
	job.Progress = len(results)
	job.mu.Unlock()
}

func handleScanStop(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	scanJobsMu.Lock()
	job, ok := scanJobs[jobID]
	scanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	job.mu.Lock()
	status := job.Status
	job.mu.Unlock()

	if status != "running" {
		jsonError(w, "scan not running", 400)
		return
	}

	job.stop()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func handleScanStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	scanJobsMu.Lock()
	job, ok := scanJobs[jobID]
	scanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	job.mu.Lock()
	status := job.Status
	progress := job.Progress
	total := job.Total
	job.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   status,
		"progress": progress,
		"total":    total,
	})
}

func handleScanEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	scanJobsMu.Lock()
	job, ok := scanJobs[jobID]
	scanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	streamSSE(w, r, func() (map[string]interface{}, bool) {
		job.mu.Lock()
		status := job.Status
		progress := job.Progress
		total := job.Total
		job.mu.Unlock()
		return map[string]interface{}{
			"status":   status,
			"progress": progress,
			"total":    total,
		}, status == "done" || status == "cancelled"
	})
}

func handleScanResults(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	scanJobsMu.Lock()
	job, ok := scanJobs[jobID]
	scanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	job.mu.Lock()
	results := append([]ScanResult(nil), job.Results...)
	outCount := job.OutCount
	status := job.Status
	job.mu.Unlock()

	showN := outCount
	if showN <= 0 || showN > len(results) {
		showN = len(results)
	}

	type resultEntry struct {
		Endpoint string  `json:"endpoint"`
		Latency  string  `json:"latency"`
		Success  bool    `json:"success"`
		Error    string  `json:"error,omitempty"`
		Attempts int     `json:"attempts,omitempty"`
		Passes   int     `json:"passes,omitempty"`
		Best     string  `json:"best,omitempty"`
		Jitter   string  `json:"jitter,omitempty"`
		Loss     float64 `json:"loss"`
		Score    int     `json:"score"`
	}

	entries := make([]resultEntry, 0, showN)
	for _, r := range results {
		if !r.Success {
			continue
		}
		entries = append(entries, resultEntry{
			Endpoint: r.Endpoint,
			Latency:  r.Latency.Round(time.Millisecond).String(),
			Success:  true,
			Attempts: r.Attempts,
			Passes:   r.Passes,
			Best:     r.Best.Round(time.Millisecond).String(),
			Jitter:   r.Jitter.Round(time.Millisecond).String(),
			Loss:     r.Loss,
			Score:    r.Score,
		})
		if len(entries) >= showN {
			break
		}
	}

	type rawEntry struct {
		Endpoint string  `json:"endpoint"`
		Latency  string  `json:"latency"`
		Attempts int     `json:"attempts,omitempty"`
		Passes   int     `json:"passes,omitempty"`
		Best     string  `json:"best,omitempty"`
		Jitter   string  `json:"jitter,omitempty"`
		Loss     float64 `json:"loss"`
		Score    int     `json:"score"`
	}

	raw := make([]rawEntry, 0)
	for _, r := range results {
		if r.Success {
			raw = append(raw, rawEntry{
				Endpoint: r.Endpoint,
				Latency:  r.Latency.Round(time.Millisecond).String(),
				Attempts: r.Attempts,
				Passes:   r.Passes,
				Best:     r.Best.Round(time.Millisecond).String(),
				Jitter:   r.Jitter.Round(time.Millisecond).String(),
				Loss:     r.Loss,
				Score:    r.Score,
			})
		}
	}

	failures := make([]failEntry, 0)
	reasons := map[string]int{}
	failedCount := 0
	for _, r := range results {
		if !r.Success {
			failedCount++
			reasons[summarizeWarpFailure(r.Error)]++
			if len(failures) < 40 {
				failures = append(failures, failEntry{Endpoint: r.Endpoint, Error: r.Error})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries":      entries,
		"raw":          raw,
		"failures":     failures,
		"fail_reasons": reasons,
		"failed_count": failedCount,
		"total":        len(entries),
		"scanned":      len(results),
		"status":       status,
	})
}

// resolveApplyOutputDir turns the user-supplied output_dir into an absolute
// directory. Empty → exeDir. Relative → filepath.Join(exeDir, cleaned).
// Absolute → cleaned absolute path (any drive/folder the OS allows).
func resolveApplyOutputDir(exeDir, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return exeDir, nil
	}
	out := filepath.Clean(raw)
	if !filepath.IsAbs(out) {
		// Reject half-rooted forms that Clean leaves non-absolute on some OSes.
		if strings.HasPrefix(out, "/") || strings.HasPrefix(out, `\`) {
			return "", fmt.Errorf("output_dir must be a relative path or an absolute path")
		}
		out = filepath.Join(exeDir, out)
	}
	// Final Clean after Join.
	out = filepath.Clean(out)
	if out == "" || out == "." {
		return "", fmt.Errorf("invalid output_dir")
	}
	return out, nil
}

func handleApplyEndpoint(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<22)
	if err := r.ParseMultipartForm(10 << 22); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	endpoint := r.FormValue("endpoint")
	if endpoint == "" {
		jsonError(w, "endpoint required", 400)
		return
	}
	if err := validateEndpointHostPort(endpoint); err != nil {
		jsonError(w, fmt.Sprintf("invalid endpoint: %v", err), 400)
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		jsonError(w, fmt.Sprintf("cannot get exe path: %v", err), 500)
		return
	}
	exeDir := filepath.Dir(exePath)

	outputDir, err := resolveApplyOutputDir(exeDir, r.FormValue("output_dir"))
	if err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		jsonError(w, fmt.Sprintf("cannot create output dir: %v", err), 400)
		return
	}

	files := r.MultipartForm.File["configs"]
	if len(files) == 0 {
		jsonError(w, "at least one config file required", 400)
		return
	}

	type fileResult struct {
		Name    string `json:"name"`
		Path    string `json:"path,omitempty"`
		Content string `json:"content,omitempty"`
		Error   string `json:"error,omitempty"`
	}

	results := make([]fileResult, 0, len(files))
	saved := 0

	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			results = append(results, fileResult{Name: fh.Filename, Error: err.Error()})
			continue
		}

		content, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			results = append(results, fileResult{Name: fh.Filename, Error: err.Error()})
			continue
		}

		text := string(content)
		lines := strings.Split(text, "\n")
		inPeer := false
		replaced := false
		newLines := make([]string, 0, len(lines))

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				inPeer = strings.ToLower(trimmed) == "[peer]"
			}
			if inPeer && strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "endpoint") {
				newLines = append(newLines, "Endpoint = "+endpoint)
				replaced = true
				continue
			}
			newLines = append(newLines, line)
		}

		if !replaced {
			results = append(results, fileResult{Name: fh.Filename, Error: "no Endpoint line found in [Peer]"})
			continue
		}

		// filepath.Base strips any directory components the browser may include.
		outPath := filepath.Join(outputDir, filepath.Base(fh.Filename))
		modified := strings.Join(newLines, "\n")
		if err := os.WriteFile(outPath, []byte(modified), 0644); err != nil {
			results = append(results, fileResult{Name: fh.Filename, Error: err.Error()})
			continue
		}

		saved++
		results = append(results, fileResult{Name: fh.Filename, Path: outPath, Content: modified})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"saved":   saved,
		"total":   len(files),
	})
}
