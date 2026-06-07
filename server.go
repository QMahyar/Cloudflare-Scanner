package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed ui
var uiFS embed.FS

type ScanJob struct {
	ID          string
	Status      string
	Progress    int
	Total       int
	Results     []ScanResult
	Config      *WarpConfig
	Endpoints   []string
	Noise       NoiseConfig
	OutCount    int
	Concurrency int
	Attempts    int
	Cancel      chan struct{}
	cancelOnce  sync.Once
	mu          sync.Mutex
}

var errFolderSelectionCancelled = errors.New("folder selection cancelled")

func handleSelectOutputDir(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonError(w, "GET required", 405)
		return
	}

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
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
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

var (
	scanJobs   = map[string]*ScanJob{}
	scanJobsMu sync.Mutex
	jobCounter int

	cleanJobs       = map[string]*CleanIPJob{}
	cleanJobsMu     sync.Mutex
	cleanJobCounter int
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
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func startServer(xrayPath string) (int, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/scan", handleScanStart(xrayPath))
	mux.HandleFunc("/api/status/", handleScanStatus)
	mux.HandleFunc("/api/results/", handleScanResults)
	mux.HandleFunc("/api/stop/", handleScanStop)
	mux.HandleFunc("/api/apply-endpoint", handleApplyEndpoint)
	mux.HandleFunc("/api/select-output-dir", handleSelectOutputDir)
	mux.HandleFunc("/api/clean-scan", handleCleanScanStart(xrayPath))
	mux.HandleFunc("/api/clean-status/", handleCleanScanStatus)
	mux.HandleFunc("/api/clean-results/", handleCleanScanResults)
	mux.HandleFunc("/api/clean-stop/", handleCleanScanStop)
	mux.HandleFunc("/api/clean-export", handleCleanExport)
	mux.HandleFunc("/api/replacer/fetch", handleReplacerFetch)
	mux.HandleFunc("/api/replacer/parse", handleReplacerParse)
	mux.HandleFunc("/api/replacer/apply", handleReplacerApply)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler:           mux,
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
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:")
	data, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "UI unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleScanStart(xrayPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonError(w, "POST required", 405)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			jsonError(w, err.Error(), 400)
			return
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
			io.Copy(tmpFile, file)
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
		if req.OutCount <= 0 {
			req.OutCount = 10
		}
		if req.Attempts <= 0 {
			req.Attempts = 2
		}
		if req.Attempts > 5 {
			req.Attempts = 5
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

		gen := NewEndpointGenerator()
		endpoints := gen.Generate(req.Count, req.IPv4, req.IPv6)

		scanJobsMu.Lock()
		jobCounter++
		jobID := fmt.Sprintf("job_%d", jobCounter)
		scanJobsMu.Unlock()

		exePath, _ := os.Executable()
		workDir := filepath.Dir(exePath)

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
			Cancel:      make(chan struct{}),
		}

		scanJobsMu.Lock()
		scanJobs[jobID] = job
		scanJobsMu.Unlock()

		go runScan(job, xrayPath, workDir)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": jobID, "total": fmt.Sprintf("%d", job.Total)})
	}
}

func runScan(job *ScanJob, xrayPath, workDir string) {
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

	scanner := NewScanner(job.Config, job.Noise, xrayPath, workDir)
	if job.Config == nil {
		scanner.TCPOnly = true
	}
	if job.Concurrency > 0 {
		scanner.Concurrency = job.Concurrency
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, scanner.Concurrency)

	for i, ep := range job.Endpoints {
		select {
		case <-ctx.Done():
			job.mu.Lock()
			job.Status = "cancelled"
			job.mu.Unlock()
			wg.Wait()
			return
		default:
		}

		wg.Add(1)
		go func(endpoint string, idx int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			result := scanner.testEndpointAttempts(ctx, endpoint, job.Attempts)

			job.mu.Lock()
			job.Results = append(job.Results, result)
			job.Progress = len(job.Results)
			_ = idx
			job.mu.Unlock()
		}(ep, i)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-ctx.Done():
		wg.Wait()
		job.mu.Lock()
		job.Status = "cancelled"
		job.Progress = len(job.Results)
		job.mu.Unlock()
		return
	}

	job.mu.Lock()
	results := append([]ScanResult(nil), job.Results...)
	job.mu.Unlock()
	sortScanResults(results)

	job.mu.Lock()
	job.Status = "done"
	job.Results = results
	job.mu.Unlock()
}

func handleScanStop(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Path[len("/api/stop/"):]

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
	jobID := r.URL.Path[len("/api/status/"):]

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

func handleScanResults(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Path[len("/api/results/"):]

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
		Endpoint string `json:"endpoint"`
		Latency  string `json:"latency"`
		Success  bool   `json:"success"`
		Error    string `json:"error,omitempty"`
		Attempts int    `json:"attempts,omitempty"`
		Passes   int    `json:"passes,omitempty"`
		Best     string `json:"best,omitempty"`
		Jitter   string `json:"jitter,omitempty"`
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
		})
		if len(entries) >= showN {
			break
		}
	}

	type rawEntry struct {
		Endpoint string `json:"endpoint"`
		Latency  string `json:"latency"`
		Attempts int    `json:"attempts,omitempty"`
		Passes   int    `json:"passes,omitempty"`
		Best     string `json:"best,omitempty"`
		Jitter   string `json:"jitter,omitempty"`
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
			})
		}
	}

	type failEntry struct {
		Endpoint string `json:"endpoint"`
		Error    string `json:"error"`
	}
	failures := make([]failEntry, 0)
	for _, r := range results {
		if !r.Success {
			failures = append(failures, failEntry{Endpoint: r.Endpoint, Error: r.Error})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries":      entries,
		"raw":          raw,
		"failures":     failures,
		"failed_count": len(failures),
		"total":        len(entries),
		"scanned":      len(results),
		"status":       status,
	})
}

func handleApplyEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<22)
	if err := r.ParseMultipartForm(10 << 22); err != nil {
		jsonError(w, err.Error(), 400)
		return
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
	VLESSURL     string `json:"vless_url"`
	Count        int    `json:"count"`
	IPv4         bool   `json:"ipv4"`
	IPv6         bool   `json:"ipv6"`
	Phase2Count  int    `json:"phase2_count"`
	OnePhase     bool   `json:"one_phase"`
	NearbyScan   bool   `json:"nearby_scan"`
	NearbyCount  int    `json:"nearby_count"`
	Phase1Probes int    `json:"phase1_probes"`
	Phase2Probes int    `json:"phase2_probes"`
	Ports        []int  `json:"ports"`
	CustomRanges string `json:"custom_ranges"`
}

func handleCleanScanStart(xrayPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonError(w, "POST required", 405)
			return
		}

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
		if req.Phase2Count <= 0 {
			req.Phase2Count = 30
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
			ranges, _ := ParseIPRanges(req.CustomRanges)
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
			ID:           jobID,
			Status:       "pending",
			Total:        len(endpoints),
			Config:       cfg,
			Endpoints:    endpoints,
			Phase2Count:  req.Phase2Count,
			SkipPhase2:   req.OnePhase,
			NearbyScan:   req.NearbyScan,
			NearbyCount:  req.NearbyCount,
			Phase1Probes: req.Phase1Probes,
			Phase2Probes: req.Phase2Probes,
			ScanPorts:    scanPorts,
			Cancel:       make(chan struct{}),
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
	jobID := r.URL.Path[len("/api/clean-stop/"):]

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

	if status != "running-phase1" && status != "running-phase2" {
		jsonError(w, "scan not running", 400)
		return
	}

	job.stop()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func handleCleanScanStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Path[len("/api/clean-status/"):]

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

func handleCleanScanResults(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Path[len("/api/clean-results/"):]

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
		Endpoint string `json:"endpoint"`
		Latency  string `json:"latency"`
		Success  bool   `json:"success"`
		Error    string `json:"error,omitempty"`
		Attempts int    `json:"attempts,omitempty"`
		Passes   int    `json:"passes,omitempty"`
		Best     string `json:"best,omitempty"`
		Jitter   string `json:"jitter,omitempty"`
		Colo     string `json:"colo,omitempty"`
		Loc      string `json:"loc,omitempty"`
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
		raw := make([]resultEntry, 0)
		for _, r := range phase2Results {
			if r.Success {
				raw = append(raw, cleanEntry(r))
			}
		}
		nearbyEntries := make([]resultEntry, 0, len(nearbyPhase2Results))
		for _, r := range nearbyPhase2Results {
			if !r.Success {
				continue
			}
			nearbyEntries = append(nearbyEntries, cleanEntry(r))
		}
		phase2Failures := len(phase2Results) - len(entries)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries":         entries,
			"raw":             raw,
			"total":           len(entries),
			"scanned":         len(phase2Results),
			"phase2_failures": phase2Failures,
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
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}

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
	}
}

func (e replacerConfigEntry) toProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		Protocol:       e.Protocol,
		UUID:           e.UUID,
		Address:        e.Address,
		Port:           e.Port,
		Encryption:     e.Encryption,
		Security:       e.Security,
		SNI:            e.SNI,
		Fingerprint:    e.FingerprintFP,
		Network:        e.Network,
		Host:           e.Host,
		Path:           e.Path,
		PacketEncoding: e.PacketEnc,
		Remark:         e.Remark,
		Flow:           e.Flow,
		PublicKey:      e.PublicKey,
		ShortId:        e.ShortId,
		SpiderX:        e.SpiderX,
		AllowInsecure:  e.AllowInsecure,
		ALPN:           e.ALPN,
		HeaderType:     e.HeaderType,
		Mode:           e.Mode,
		ServiceName:    e.ServiceName,
	}
}

func handleReplacerFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}

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
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}

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
	Configs   []replacerConfigEntry `json:"configs"`
	Endpoints []string              `json:"endpoints"`
}

func handleReplacerApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}

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

	configs := make([]*ProxyConfig, 0, len(req.Configs))
	for _, e := range req.Configs {
		configs = append(configs, e.toProxyConfig())
	}

	urls := GenerateReplacedConfigs(configs, req.Endpoints)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"urls":  urls,
		"count": len(urls),
	})
}
