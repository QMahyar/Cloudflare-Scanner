package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed ui
var uiFS embed.FS

type ScanJob struct {
	ID        string
	Status    string
	Progress  int
	Total     int
	Results   []ScanResult
	Config    *WarpConfig
	Endpoints []string
	Noise     NoiseConfig
	OutCount  int
	Cancel    chan struct{}
	mu        sync.Mutex
}

var (
	scanJobs   = map[string]*ScanJob{}
	scanJobsMu sync.Mutex
	jobCounter int
)

type scanRequest struct {
	Noise     bool   `json:"noise"`
	IPv4      bool   `json:"ipv4"`
	IPv6      bool   `json:"ipv6"`
	Count     int    `json:"count"`
	OutCount  int    `json:"outCount"`
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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}

	go http.Serve(listener, mux)
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, _ := uiFS.ReadFile("ui/index.html")
	w.Write(data)
}

func handleScanStart(xrayPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonError(w, "POST required", 405)
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}

		file, _, err := r.FormFile("config")
		if err != nil {
			jsonError(w, "config file required", 400)
			return
		}
		defer file.Close()

		tmpFile, _ := os.CreateTemp("", "warp-*.conf")
		defer os.Remove(tmpFile.Name())
		io.Copy(tmpFile, file)
		tmpFile.Close()

		cfg, err := ParseWarpConfig(tmpFile.Name())
		if err != nil {
			jsonError(w, fmt.Sprintf("%v", err), 400)
			return
		}

		var req scanRequest
		jsonStr := r.FormValue("params")
		if jsonStr != "" {
			json.Unmarshal([]byte(jsonStr), &req)
		}

		if req.Count <= 0 {
			req.Count = 100
		}
		if req.OutCount <= 0 {
			req.OutCount = 10
		}
		if !req.IPv4 && !req.IPv6 {
			req.IPv4 = true
		}

		noise := NoiseConfig{}
		if req.Noise {
			noise = DefaultNoise()
		}

		gen := NewEndpointGenerator()
		endpoints := gen.Generate(req.Count, req.IPv4, req.IPv6)

		jobCounter++
		jobID := fmt.Sprintf("job_%d", jobCounter)

		exePath, _ := os.Executable()
		workDir := filepath.Dir(exePath)

		job := &ScanJob{
			ID:        jobID,
			Status:    "running",
			Total:     len(endpoints),
			Config:    cfg,
			Endpoints: endpoints,
			Noise:     noise,
			OutCount:  req.OutCount,
			Cancel:    make(chan struct{}),
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
	scanner := NewScanner(job.Config, job.Noise, xrayPath, workDir)

	var mu sync.Mutex
	var results []ScanResult
	var wg sync.WaitGroup
	sem := make(chan struct{}, scanner.Concurrency)

	for i, ep := range job.Endpoints {
		select {
		case <-job.Cancel:
			job.mu.Lock()
			job.Status = "cancelled"
			job.mu.Unlock()
			return
		default:
		}

		wg.Add(1)
		go func(endpoint string, idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := scanner.testEndpoint(endpoint)

			mu.Lock()
			results = append(results, result)
			progress := len(results)
			mu.Unlock()

			job.mu.Lock()
			job.Results = results
			job.Progress = progress
			_ = idx
			job.mu.Unlock()
		}(ep, i)
	}

	wg.Wait()

	select {
	case <-job.Cancel:
		job.mu.Lock()
		job.Status = "cancelled"
		job.mu.Unlock()
		return
	default:
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

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

	close(job.Cancel)

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
	results := job.Results
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
		})
		if len(entries) >= showN {
			break
		}
	}

	type rawEntry struct {
		Endpoint string `json:"endpoint"`
		Latency  string `json:"latency"`
	}

	raw := make([]rawEntry, 0)
	for _, r := range results {
		if r.Success {
			raw = append(raw, rawEntry{Endpoint: r.Endpoint, Latency: r.Latency.Round(time.Millisecond).String()})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"raw":     raw,
		"total":   len(entries),
		"scanned": len(results),
		"status":  status,
	})
}

func handleApplyEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}

	if err := r.ParseMultipartForm(10 << 22); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	endpoint := r.FormValue("endpoint")
	if endpoint == "" {
		jsonError(w, "endpoint required", 400)
		return
	}

	outputDir := r.FormValue("output_dir")
	if outputDir == "" {
		exePath, err := os.Executable()
		if err != nil {
			jsonError(w, fmt.Sprintf("cannot get exe path: %v", err), 500)
			return
		}
		outputDir = filepath.Dir(exePath)
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
		Name     string `json:"name"`
		Path     string `json:"path,omitempty"`
		Content  string `json:"content,omitempty"`
		Error    string `json:"error,omitempty"`
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

		outPath := filepath.Join(outputDir, fh.Filename)
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
