package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed all:ui/dist
var uiFS embed.FS

// distFS is the embedded Vite build output (ui/dist) rooted so that
// "index.html" and "assets/..." resolve directly. Initialized in startServer.
var distFS fs.FS

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

var errFolderSelectionCancelled = errors.New("folder selection cancelled")

func handleSelectOutputDir(w http.ResponseWriter, r *http.Request) {
	dir, err := selectOutputDir()
	if err != nil {
		if errors.Is(err, errFolderSelectionCancelled) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"cancelled": true})
			return
		}
		jsonError(w, fmt.Sprintf("folder picker failed: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"path": dir})
}

func selectOutputDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return selectOutputDirWindows()
	case "darwin":
		return selectOutputDirDarwin()
	default:
		return selectOutputDirLinux()
	}
}

func selectOutputDirWindows() (string, error) {
	script := `Add-Type -AssemblyName System.Windows.Forms; $dlg = New-Object System.Windows.Forms.FolderBrowserDialog; $dlg.Description = "Select output folder"; $dlg.ShowNewFolderButton = $true; if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($dlg.SelectedPath) } else { exit 2 }`
	// Resolve powershell.exe by absolute path rather than via PATH lookup, so a
	// stray powershell.exe in an untrusted working/PATH directory can't be run.
	// Mirrors the absolute cmd.exe path used in openBrowser (main.go).
	psPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(psPath); err != nil {
		psPath = "powershell" // fall back to PATH if SystemRoot layout is unusual
	}
	cmd := exec.Command(psPath, "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return "", errFolderSelectionCancelled
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errFolderSelectionCancelled
	}
	return filepath.Clean(path), nil
}

func selectOutputDirDarwin() (string, error) {
	cmd := exec.Command("/usr/bin/osascript",
		"-e", `set selectedFolder to choose folder with prompt "Select output folder"`,
		"-e", `POSIX path of selectedFolder`,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(string(out)))
		if strings.Contains(msg, "user canceled") || strings.Contains(msg, "cancelled") {
			return "", errFolderSelectionCancelled
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errFolderSelectionCancelled
	}
	return filepath.Clean(path), nil
}

func selectOutputDirLinux() (string, error) {
	type pickerCmd struct {
		name string
		args []string
	}
	candidates := []pickerCmd{
		{name: "zenity", args: []string{"--file-selection", "--directory", "--title=Select output folder"}},
		{name: "kdialog", args: []string{"--getexistingdirectory", ".", "--title", "Select output folder"}},
	}
	for _, c := range candidates {
		cmd := exec.Command(c.name, c.args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", errFolderSelectionCancelled
			}
			continue
		}
		path := strings.TrimSpace(string(out))
		if path == "" {
			return "", errFolderSelectionCancelled
		}
		return filepath.Clean(path), nil
	}
	return "", errors.New("no supported folder picker found (install zenity or kdialog)")
}

// stop closes the Cancel channel at most once, so concurrent stop requests
// cannot panic with "close of closed channel".
func (j *ScanJob) stop() {
	j.cancelOnce.Do(func() { close(j.Cancel) })
}

const jobTTL = 10 * time.Minute

// maxScanCount caps the requested endpoint/IP count so a huge value can't drive
// the generators into multi-GB allocations (clean scan) or excessive work. The
// WARP generator is additionally bounded by its finite address pool.
const maxScanCount = 100000
const maxEndpointConcurrency = 2048
const maxCleanPhase1Probes = 4096
const maxCleanPhase2Probes = 256
const maxOutCount = 10000
const maxReplacerOutputs = 50000
const csrfCookieName = "cfscanner_token"
const csrfHeaderName = "X-CSRF-Token"

var (
	scanJobs   = map[string]*ScanJob{}
	scanJobsMu sync.Mutex
	jobCounter int

	cleanJobs       = map[string]*CleanIPJob{}
	cleanJobsMu     sync.Mutex
	cleanJobCounter int

	// warpSocksPortBase hands out non-overlapping SOCKS port windows for the WARP
	// noise-fallback batches. Process-global (not per-job) so two WARP noise scans
	// running at once can't allocate the same window.
	warpSocksPortBase atomic.Int32
)

func scheduleScanJobCleanup(id string) {
	time.AfterFunc(jobTTL, func() {
		scanJobsMu.Lock()
		delete(scanJobs, id)
		scanJobsMu.Unlock()
	})
}

func scheduleCleanJobCleanup(id string) {
	time.AfterFunc(jobTTL, func() {
		cleanJobsMu.Lock()
		delete(cleanJobs, id)
		cleanJobsMu.Unlock()
	})
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

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func newCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isLoopbackHost reports whether an HTTP Host header targets this loopback-only
// server. It rejects any other hostname so a malicious page that rebinds its own
// DNS name to 127.0.0.1 can't reach the API (the browser still sends the
// attacker's hostname in Host, never "127.0.0.1"). An empty Host is allowed: the
// rebinding vector is a browser, which always sends one.
func isLoopbackHost(hostHeader string) bool {
	if hostHeader == "" {
		return true
	}
	host := hostHeader
	if h, _, err := net.SplitHostPort(hostHeader); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func csrfMiddleware(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}

		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			http.SetCookie(w, &http.Cookie{
				Name:     csrfCookieName,
				Value:    token,
				Path:     "/",
				SameSite: http.SameSiteStrictMode,
			})
		}

		needsToken := strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet
		if r.URL.Path == "/api/select-output-dir" || r.URL.Path == "/api/update-check" {
			needsToken = true
		}
		if !needsToken {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(csrfCookieName)
		if err != nil || cookie.Value == "" || cookie.Value != token || r.Header.Get(csrfHeaderName) != token {
			jsonError(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func startServer(xrayPath string) (*http.Server, int, error) {
	sub, err := fs.Sub(uiFS, "ui/dist")
	if err != nil {
		return nil, 0, fmt.Errorf("embed ui/dist: %w", err)
	}
	distFS = sub

	token, err := newCSRFToken()
	if err != nil {
		return nil, 0, fmt.Errorf("csrf token: %w", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.Handle("GET /assets/", http.FileServerFS(distFS))
	mux.HandleFunc("POST /api/scan", handleScanStart(xrayPath))
	mux.HandleFunc("GET /api/status/{id}", handleScanStatus)
	mux.HandleFunc("GET /api/scan-events/{id}", handleScanEvents)
	mux.HandleFunc("GET /api/results/{id}", handleScanResults)
	mux.HandleFunc("POST /api/stop/{id}", handleScanStop)
	mux.HandleFunc("POST /api/apply-endpoint", handleApplyEndpoint)
	mux.HandleFunc("GET /api/select-output-dir", handleSelectOutputDir)
	mux.HandleFunc("POST /api/clean-scan", handleCleanScanStart(xrayPath))
	mux.HandleFunc("GET /api/clean-status/{id}", handleCleanScanStatus)
	mux.HandleFunc("GET /api/clean-events/{id}", handleCleanScanEvents)
	mux.HandleFunc("GET /api/clean-results/{id}", handleCleanScanResults)
	mux.HandleFunc("POST /api/clean-stop/{id}", handleCleanScanStop)
	mux.HandleFunc("POST /api/clean-export", handleCleanExport)
	mux.HandleFunc("POST /api/replacer/fetch", handleReplacerFetch)
	mux.HandleFunc("POST /api/replacer/parse", handleReplacerParse)
	mux.HandleFunc("POST /api/replacer/apply", handleReplacerApply)
	mux.HandleFunc("GET /api/version", handleVersion)
	mux.HandleFunc("GET /api/update-check", handleUpdateCheck)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler:           csrfMiddleware(mux, token),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()
	return srv, listener.Addr().(*net.TCPAddr).Port, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// Vite emits external module scripts, so script-src no longer needs
	// 'unsafe-inline'. style-src keeps it for Svelte's dynamic style= bindings
	// and inline styles; connect-src 'self' covers fetch/SSE to the API.
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
	data, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		http.Error(w, "UI unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
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

		scanJobsMu.Lock()
		jobCounter++
		jobID := fmt.Sprintf("job_%d", jobCounter)
		scanJobsMu.Unlock()

		job := &ScanJob{
			ID:          jobID,
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
	results := append([]ScanResult(nil), job.Results...)
	job.mu.Unlock()

	sortScanResults(results)

	job.mu.Lock()
	job.Results = results
	job.Progress = len(results)
	job.mu.Unlock()
}

// runScanNoiseBatched validates WARP endpoints through pooled xray processes (one
// process per batch) instead of one process per endpoint. It is used only for the
// noise/AmneziaWG fallback; the native-handshake and TCP-only paths are
// process-free and never reach here. Honors stop-after and cancellation, retries
// each batch's failures once, and keeps partial results on cancel.
func runScanNoiseBatched(ctx context.Context, job *ScanJob, scanner *Scanner) {
	const batchSize = 16
	concurrentBatches := (scanner.Concurrency + batchSize - 1) / batchSize
	if concurrentBatches < 1 {
		concurrentBatches = 1
	}

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

	var batches [][]string
	for i := 0; i < len(job.Endpoints); i += batchSize {
		end := i + batchSize
		if end > len(job.Endpoints) {
			end = len(job.Endpoints)
		}
		batches = append(batches, job.Endpoints[i:end])
	}

	sem := make(chan struct{}, concurrentBatches)
	var wg sync.WaitGroup

	for _, b := range batches {
		select {
		case <-ctx.Done():
			goto wait
		default:
		}

		wg.Add(1)
		go func(batch []string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			res := scanner.scanBatchNoise(ctx, batch, allocPortBase())

			// Retry the failures once in a fresh batch — mirrors the old 2-attempt
			// behavior without paying 2× on endpoints that pass first try. Skip
			// when the whole batch failed (systemic: broken xray / wrong config /
			// every endpoint dead) since a second spawn only doubles the cost for
			// no benefit, and skip once the job is cancelled.
			var retryIdx []int
			var retryEps []string
			for i, r := range res {
				if !r.Success {
					retryIdx = append(retryIdx, i)
					retryEps = append(retryEps, batch[i])
				}
			}
			partialFailure := len(retryEps) > 0 && len(retryEps) < len(batch)
			if partialFailure {
				select {
				case <-ctx.Done():
				default:
					rres := scanner.scanBatchNoise(ctx, retryEps, allocPortBase())
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
		}(b)
	}

wait:
	wg.Wait()

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

// streamSSE drives a Server-Sent Events response from periodic snapshots of a
// job. It calls snapshot() immediately, then every 250ms, emitting a `data:`
// frame only when the JSON changes, and returns once snapshot reports done or
// the client disconnects. This is server-pushed polling: the scan goroutines
// stay untouched, but the client holds one connection instead of hammering the
// status endpoint a few times a second. Results stay on their own poll.
func streamSSE(w http.ResponseWriter, r *http.Request, snapshot func() (map[string]interface{}, bool)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming unsupported", 500)
		return
	}
	// The server-wide WriteTimeout (set in startServer) is a deadline on the
	// whole response, which is correct for normal JSON handlers but fatal for an
	// SSE stream that legitimately stays open for the entire scan (a noise scan
	// can run minutes). Once it fires mid-stream the connection is torn down and
	// the browser logs ERR_INCOMPLETE_CHUNKED_ENCODING. Clear the deadline for
	// this one response so the stream lives as long as the scan does; the
	// ctx.Done() select below still tears it down when the client disconnects.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering if any

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	ctx := r.Context()

	var last string
	for {
		data, done := snapshot()
		b, _ := json.Marshal(data)
		if s := string(b); s != last {
			last = s
			fmt.Fprintf(w, "data: %s\n\n", s)
			flusher.Flush()
		}
		if done {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
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

	type failEntry struct {
		Endpoint string `json:"endpoint"`
		Error    string `json:"error"`
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
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		jsonError(w, fmt.Sprintf("invalid endpoint format (expected host:port): %v", err), 400)
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		jsonError(w, fmt.Sprintf("cannot get exe path: %v", err), 500)
		return
	}
	exeDir := filepath.Dir(exePath)

	outputDir := r.FormValue("output_dir")
	if outputDir == "" {
		outputDir = exeDir
	}
	outputDir = filepath.Clean(outputDir)
	if !filepath.IsAbs(outputDir) && (strings.HasPrefix(outputDir, "/") || strings.HasPrefix(outputDir, `\\`)) {
		jsonError(w, "output_dir must be relative to app directory or an absolute path inside it", 400)
		return
	}
	if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(exeDir, outputDir)
	}
	rel, err := filepath.Rel(exeDir, outputDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		jsonError(w, "output_dir must stay inside the app directory", 400)
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

type cleanScanRequest struct {
	VLESSURL        string `json:"vless_url"`
	Count           int    `json:"count"`
	IPv4            bool   `json:"ipv4"`
	IPv6            bool   `json:"ipv6"`
	Phase2Count     int    `json:"phase2_count"`
	OnePhase        bool   `json:"one_phase"`
	NearbyScan      bool   `json:"nearby_scan"`
	NearbyCount     int    `json:"nearby_count"`
	Phase1Probes    int    `json:"phase1_probes"`
	Phase2Probes    int    `json:"phase2_probes"`
	TimeoutMs       int    `json:"timeout_ms"`
	Phase2TimeoutMs int    `json:"phase2_timeout_ms"`
	Ports           []int  `json:"ports"`
	CustomRanges    string `json:"custom_ranges"`
	StopAfter       int    `json:"stop_after"`
}

func handleCleanScanStart(xrayPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req cleanScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}

		if !req.OnePhase && req.VLESSURL == "" {
			jsonError(w, "vless_url required", 400)
			return
		}

		var cfg *ProxyConfig
		if req.VLESSURL != "" {
			var err error
			cfg, err = ParseProxyURL(req.VLESSURL)
			if err != nil {
				jsonError(w, fmt.Sprintf("parse url: %v", err), 400)
				return
			}
		}

		if req.Count <= 0 {
			req.Count = 1000
		}
		if req.Count > maxScanCount {
			req.Count = maxScanCount
		}
		if req.Phase2Count <= 0 {
			req.Phase2Count = 30
		}
		req.Phase2Count = clampInt(req.Phase2Count, 1, maxOutCount)
		req.Phase1Probes = clampInt(req.Phase1Probes, 0, maxCleanPhase1Probes)
		req.Phase2Probes = clampInt(req.Phase2Probes, 0, maxCleanPhase2Probes)
		if req.StopAfter < 0 {
			req.StopAfter = 0
		}
		if req.StopAfter > req.Count {
			req.StopAfter = req.Count
		}
		// 0 means "use the default"; otherwise clamp each to a sane window.
		if req.TimeoutMs > 0 {
			if req.TimeoutMs < 100 {
				req.TimeoutMs = 100
			}
			if req.TimeoutMs > 60000 {
				req.TimeoutMs = 60000
			}
		}
		if req.Phase2TimeoutMs > 0 {
			if req.Phase2TimeoutMs < 100 {
				req.Phase2TimeoutMs = 100
			}
			if req.Phase2TimeoutMs > 60000 {
				req.Phase2TimeoutMs = 60000
			}
		}
		if !req.IPv4 && !req.IPv6 {
			req.IPv4 = true
		}

		// Resolve scan ports — validate and default to 443
		var scanPorts []int
		for _, p := range req.Ports {
			if p >= 1 && p <= 65535 {
				scanPorts = append(scanPorts, p)
			}
		}
		if len(scanPorts) == 0 {
			scanPorts = []int{443}
		}

		gen := NewCleanIPGenerator()
		var endpoints []string
		if strings.TrimSpace(req.CustomRanges) != "" {
			ranges, bad := ParseIPRanges(req.CustomRanges)
			if len(bad) > 0 {
				jsonError(w, fmt.Sprintf("%d invalid custom range line(s), first: %s", len(bad), bad[0]), 400)
				return
			}
			if len(ranges) == 0 {
				jsonError(w, "no valid IP ranges found in custom input", 400)
				return
			}
			endpoints = GenerateFromRanges(ranges, req.Count, scanPorts, newRangeRNG())
		} else {
			endpoints = gen.GenerateIPs(req.Count, req.IPv4, req.IPv6, scanPorts)
		}

		cleanJobsMu.Lock()
		cleanJobCounter++
		jobID := fmt.Sprintf("clean_%d", cleanJobCounter)
		cleanJobsMu.Unlock()

		job := &CleanIPJob{
			ID:              jobID,
			Status:          "pending",
			Total:           len(endpoints),
			Config:          cfg,
			Endpoints:       endpoints,
			Phase2Count:     req.Phase2Count,
			SkipPhase2:      req.OnePhase,
			NearbyScan:      req.NearbyScan,
			NearbyCount:     req.NearbyCount,
			Phase1Probes:    req.Phase1Probes,
			Phase2Probes:    req.Phase2Probes,
			TimeoutMs:       req.TimeoutMs,
			Phase2TimeoutMs: req.Phase2TimeoutMs,
			StopAfter:       req.StopAfter,
			ScanPorts:       scanPorts,
			Cancel:          make(chan struct{}),
		}

		cleanJobsMu.Lock()
		cleanJobs[jobID] = job
		cleanJobsMu.Unlock()

		go runCleanScan(job, xrayPath)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": jobID, "total": fmt.Sprintf("%d", job.Total)})
	}
}

func handleCleanScanStop(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	cleanJobsMu.Lock()
	job, ok := cleanJobs[jobID]
	cleanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	job.mu.Lock()
	status := job.Status
	job.mu.Unlock()

	// "pending" covers the brief window between job creation and runCleanScan
	// flipping to running-phase1 — a stop clicked there must still take effect.
	if status != "pending" && status != "running-phase1" && status != "running-phase2" {
		jsonError(w, "scan not running", 400)
		return
	}

	job.stop()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func handleCleanScanStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	cleanJobsMu.Lock()
	job, ok := cleanJobs[jobID]
	cleanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	job.mu.Lock()
	status := job.Status
	phase1Progress := job.Phase1Progress
	phase1Total := job.Phase1Total
	phase2Progress := job.Phase2Progress
	phase2Total := job.Phase2Total
	job.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          status,
		"phase1_progress": phase1Progress,
		"phase1_total":    phase1Total,
		"phase2_progress": phase2Progress,
		"phase2_total":    phase2Total,
	})
}

func handleCleanScanEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	cleanJobsMu.Lock()
	job, ok := cleanJobs[jobID]
	cleanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	streamSSE(w, r, func() (map[string]interface{}, bool) {
		job.mu.Lock()
		status := job.Status
		phase1Progress := job.Phase1Progress
		phase1Total := job.Phase1Total
		phase2Progress := job.Phase2Progress
		phase2Total := job.Phase2Total
		job.mu.Unlock()
		return map[string]interface{}{
			"status":          status,
			"phase1_progress": phase1Progress,
			"phase1_total":    phase1Total,
			"phase2_progress": phase2Progress,
			"phase2_total":    phase2Total,
		}, status == "done" || status == "cancelled"
	})
}

func handleCleanScanResults(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	cleanJobsMu.Lock()
	job, ok := cleanJobs[jobID]
	cleanJobsMu.Unlock()

	if !ok {
		jsonError(w, "not found", 404)
		return
	}

	job.mu.Lock()
	status := job.Status
	phase1Results := append([]CleanIPResult(nil), job.Phase1Results...)
	phase2Results := append([]CleanIPResult(nil), job.Phase2Results...)
	nearbyPhase1Results := append([]CleanIPResult(nil), job.NearbyPhase1Results...)
	nearbyPhase2Results := append([]CleanIPResult(nil), job.NearbyPhase2Results...)
	phase1Progress := job.Phase1Progress
	phase1Total := job.Phase1Total
	skipPhase2 := job.SkipPhase2
	job.mu.Unlock()

	showPhase2 := (status == "done" || status == "cancelled" || status == "running-phase2") && !skipPhase2

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
		H3       bool    `json:"h3"`
		Colo     string  `json:"colo,omitempty"`
		Loc      string  `json:"loc,omitempty"`
	}

	cleanEntry := func(r CleanIPResult) resultEntry {
		return resultEntry{
			Endpoint: r.Endpoint,
			Latency:  r.Latency.Round(time.Millisecond).String(),
			Success:  r.Success,
			Error:    r.Error,
			Attempts: r.Attempts,
			Passes:   r.Passes,
			Best:     r.Best.Round(time.Millisecond).String(),
			Jitter:   r.Jitter.Round(time.Millisecond).String(),
			Loss:     r.Loss,
			Score:    r.Score,
			H3:       r.H3,
			Colo:     r.Colo,
			Loc:      r.Loc,
		}
	}

	if showPhase2 {
		entries := make([]resultEntry, 0, len(phase2Results))
		for _, r := range phase2Results {
			if !r.Success {
				continue
			}
			entries = append(entries, cleanEntry(r))
		}
		nearbyEntries := make([]resultEntry, 0, len(nearbyPhase2Results))
		for _, r := range nearbyPhase2Results {
			if !r.Success {
				continue
			}
			nearbyEntries = append(nearbyEntries, cleanEntry(r))
		}
		phase2Failures := len(phase2Results) - len(entries)

		// Surface WHY validation failed. Without this, an all-failed Phase 2 is a
		// silent dead end (broken xray, wrong Host, too-tight timeout all look the
		// same). We return a bounded sample of endpoint+error plus an aggregated
		// reason->count summary so the UI can explain the outcome.
		type failEntry struct {
			Endpoint string `json:"endpoint"`
			Error    string `json:"error"`
		}
		failures := make([]failEntry, 0)
		reasons := map[string]int{}
		for _, r := range phase2Results {
			if r.Success {
				continue
			}
			reason := r.Error
			if reason == "" {
				reason = "unknown"
			}
			reasons[summarizeFailure(reason)]++
			if len(failures) < 40 {
				failures = append(failures, failEntry{Endpoint: r.Endpoint, Error: r.Error})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries":         entries,
			"total":           len(entries),
			"scanned":         len(phase2Results),
			"phase2_failures": phase2Failures,
			"failures":        failures,
			"fail_reasons":    reasons,
			"status":          status,
			"phase":           "phase2",
			"phase1":          phase1Progress,
			"nearby_entries":  nearbyEntries,
		})
		return
	}

	entries := make([]resultEntry, 0, len(phase1Results))
	for _, r := range phase1Results {
		if !r.Success {
			continue
		}
		entries = append(entries, cleanEntry(r))
	}
	nearbyEntries := make([]resultEntry, 0, len(nearbyPhase1Results))
	for _, r := range nearbyPhase1Results {
		if !r.Success {
			continue
		}
		nearbyEntries = append(nearbyEntries, cleanEntry(r))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries":        entries,
		"total":          len(entries),
		"scanned":        phase1Progress,
		"phase1_total":   phase1Total,
		"status":         status,
		"phase":          "phase1",
		"phase1":         phase1Progress,
		"nearby_entries": nearbyEntries,
	})
}

type cleanExportRequest struct {
	VLESSURL  string   `json:"vless_url"`
	Endpoints []string `json:"endpoints"`
}

func handleCleanExport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req cleanExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if req.VLESSURL == "" || len(req.Endpoints) == 0 {
		jsonError(w, "vless_url and endpoints required", 400)
		return
	}

	cfg, err := ParseProxyURL(req.VLESSURL)
	if err != nil {
		jsonError(w, fmt.Sprintf("parse url: %v", err), 400)
		return
	}

	urls := cfg.GenerateExport(req.Endpoints)

	content := "# Clean IP VLESS Configs\n"
	content += "# Generated by Cloudflare Scanner\n"
	content += fmt.Sprintf("# Original: %s\n", req.VLESSURL)
	content += fmt.Sprintf("# Working IPs: %d\n\n", len(urls))
	for _, u := range urls {
		content += u + "\n"
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=clean_ips_vless.txt")
	w.Write([]byte(content))
}

type replacerFetchRequest struct {
	URL string `json:"url"`
}

type replacerConfigEntry struct {
	Fingerprint   string `json:"fingerprint"`
	Protocol      string `json:"protocol"`
	UUID          string `json:"uuid"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Encryption    string `json:"encryption"`
	Security      string `json:"security"`
	SNI           string `json:"sni"`
	FingerprintFP string `json:"fingerprint_fp"`
	Network       string `json:"network"`
	Host          string `json:"host"`
	Path          string `json:"path"`
	PacketEnc     string `json:"packet_enc"`
	Remark        string `json:"remark"`
	Flow          string `json:"flow,omitempty"`
	PublicKey     string `json:"pbk,omitempty"`
	ShortId       string `json:"sid,omitempty"`
	SpiderX       string `json:"spx,omitempty"`
	AllowInsecure bool   `json:"allow_insecure,omitempty"`
	ALPN          string `json:"alpn,omitempty"`
	HeaderType    string `json:"header_type,omitempty"`
	Mode          string `json:"mode,omitempty"`
	ServiceName   string `json:"service_name,omitempty"`
	MaxEarlyData  int    `json:"max_early_data,omitempty"`
	EarlyDataHdr  string `json:"early_data_header,omitempty"`
}

// proxyConfigToEntry / toProxyConfig are the single source of truth for the
// ProxyConfig <-> replacerConfigEntry mapping used by the replacer handlers.
func proxyConfigToEntry(c *ProxyConfig) replacerConfigEntry {
	return replacerConfigEntry{
		Fingerprint:   ConfigFingerprint(c),
		Protocol:      c.Protocol,
		UUID:          c.UUID,
		Address:       c.Address,
		Port:          c.Port,
		Encryption:    c.Encryption,
		Security:      c.Security,
		SNI:           c.SNI,
		FingerprintFP: c.Fingerprint,
		Network:       c.Network,
		Host:          c.Host,
		Path:          c.Path,
		PacketEnc:     c.PacketEncoding,
		Remark:        c.Remark,
		Flow:          c.Flow,
		PublicKey:     c.PublicKey,
		ShortId:       c.ShortId,
		SpiderX:       c.SpiderX,
		AllowInsecure: c.AllowInsecure,
		ALPN:          c.ALPN,
		HeaderType:    c.HeaderType,
		Mode:          c.Mode,
		ServiceName:   c.ServiceName,
		MaxEarlyData:  c.MaxEarlyData,
		EarlyDataHdr:  c.EarlyDataHeaderName,
	}
}

func (e replacerConfigEntry) toProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		Protocol:            e.Protocol,
		UUID:                e.UUID,
		Address:             e.Address,
		Port:                e.Port,
		Encryption:          e.Encryption,
		Security:            e.Security,
		SNI:                 e.SNI,
		Fingerprint:         e.FingerprintFP,
		Network:             e.Network,
		Host:                e.Host,
		Path:                e.Path,
		PacketEncoding:      e.PacketEnc,
		Remark:              e.Remark,
		Flow:                e.Flow,
		PublicKey:           e.PublicKey,
		ShortId:             e.ShortId,
		SpiderX:             e.SpiderX,
		AllowInsecure:       e.AllowInsecure,
		ALPN:                e.ALPN,
		HeaderType:          e.HeaderType,
		Mode:                e.Mode,
		ServiceName:         e.ServiceName,
		MaxEarlyData:        e.MaxEarlyData,
		EarlyDataHeaderName: e.EarlyDataHdr,
	}
}

func handleReplacerFetch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req replacerFetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if req.URL == "" {
		jsonError(w, "url required", 400)
		return
	}

	content, err := FetchSubscription(req.URL)
	if err != nil {
		jsonError(w, fmt.Sprintf("fetch: %v", err), 400)
		return
	}

	configs, err := ParseSubscription(content)
	if err != nil {
		jsonError(w, fmt.Sprintf("parse: %v", err), 400)
		return
	}

	if len(configs) == 0 {
		jsonError(w, "no valid configs found in subscription", 400)
		return
	}

	unique := DeduplicateConfigs(configs)

	entries := make([]replacerConfigEntry, 0, len(unique))
	for _, c := range unique {
		entries = append(entries, proxyConfigToEntry(c))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": entries,
		"total":   len(configs),
		"unique":  len(unique),
	})
}

type replacerParseRequest struct {
	Raw string `json:"raw"`
}

func handleReplacerParse(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req replacerParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if req.Raw == "" {
		jsonError(w, "raw text required", 400)
		return
	}

	configs := ParseRawConfigs(req.Raw)

	if len(configs) == 0 {
		jsonError(w, "no valid configs found in text", 400)
		return
	}

	unique := DeduplicateConfigs(configs)

	entries := make([]replacerConfigEntry, 0, len(unique))
	for _, c := range unique {
		entries = append(entries, proxyConfigToEntry(c))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"configs": entries,
		"total":   len(configs),
		"unique":  len(unique),
	})
}

type replacerApplyRequest struct {
	Configs      []replacerConfigEntry `json:"configs"`
	Endpoints    []string              `json:"endpoints"`
	NameTemplate string                `json:"name_template"`
}

func handleReplacerApply(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req replacerApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	if len(req.Configs) == 0 {
		jsonError(w, "no configs provided", 400)
		return
	}
	if len(req.Endpoints) == 0 {
		jsonError(w, "no endpoints provided", 400)
		return
	}

	if len(req.Configs)*len(req.Endpoints) > maxReplacerOutputs {
		jsonError(w, fmt.Sprintf("too many outputs requested (max %d)", maxReplacerOutputs), 400)
		return
	}

	configs := make([]*ProxyConfig, 0, len(req.Configs))
	for _, e := range req.Configs {
		cfg := e.toProxyConfig()
		if cfg.Protocol != "vless" && cfg.Protocol != "trojan" && cfg.Protocol != "vmess" {
			jsonError(w, "unsupported config protocol", 400)
			return
		}
		if cfg.UUID == "" {
			jsonError(w, "config UUID/password required", 400)
			return
		}
		configs = append(configs, cfg)
	}

	endpoints := make([]string, 0, len(req.Endpoints))
	for _, ep := range req.Endpoints {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}
		host, portStr, err := net.SplitHostPort(ep)
		port, perr := strconv.Atoi(portStr)
		if err != nil || perr != nil || host == "" || port < 1 || port > 65535 {
			jsonError(w, fmt.Sprintf("invalid endpoint: %s", ep), 400)
			return
		}
		endpoints = append(endpoints, ep)
	}
	if len(endpoints) == 0 {
		jsonError(w, "no valid endpoints provided", 400)
		return
	}

	urls := GenerateReplacedConfigsNamed(configs, endpoints, req.NameTemplate)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"urls":         urls,
		"count":        len(urls),
		"subscription": base64.StdEncoding.EncodeToString([]byte(strings.Join(urls, "\n"))),
	})
}
