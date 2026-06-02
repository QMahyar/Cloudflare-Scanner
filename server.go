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

	cleanJobs   = map[string]*CleanIPJob{}
	cleanJobsMu sync.Mutex
	cleanJobCounter int
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

type cleanScanRequest struct {
	VLESSURL    string `json:"vless_url"`
	Count       int    `json:"count"`
	IPv4        bool   `json:"ipv4"`
	IPv6        bool   `json:"ipv6"`
	Phase2Count int    `json:"phase2_count"`
	OnePhase    bool   `json:"one_phase"`
}

func handleCleanScanStart(xrayPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonError(w, "POST required", 405)
			return
		}

		var req cleanScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, err.Error(), 400)
			return
		}

		if !req.OnePhase && req.VLESSURL == "" {
			jsonError(w, "vless_url required", 400)
			return
		}

		port := 443
		var cfg *ProxyConfig
		if req.VLESSURL != "" {
			var err error
			cfg, err = ParseProxyURL(req.VLESSURL)
			if err != nil {
				jsonError(w, fmt.Sprintf("parse url: %v", err), 400)
				return
			}
			port = cfg.Port
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

		gen := NewCleanIPGenerator()
		endpoints := gen.GenerateIPs(req.Count, req.IPv4, req.IPv6, port)

		cleanJobCounter++
		jobID := fmt.Sprintf("clean_%d", cleanJobCounter)

		job := &CleanIPJob{
			ID:          jobID,
			Status:      "pending",
			Total:       len(endpoints),
			Config:      cfg,
			Endpoints:   endpoints,
			Phase2Count: req.Phase2Count,
			SkipPhase2:  req.OnePhase,
			Cancel:      make(chan struct{}),
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

	close(job.Cancel)

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
	phase1Results := job.Phase1Results
	phase2Results := job.Phase2Results
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
	}

	if showPhase2 {
		entries := make([]resultEntry, 0, len(phase2Results))
		for _, r := range phase2Results {
			if !r.Success {
				continue
			}
			entries = append(entries, resultEntry{
				Endpoint: r.Endpoint,
				Latency:  r.Latency.Round(time.Millisecond).String(),
				Success:  true,
			})
		}
		raw := make([]resultEntry, 0)
		for _, r := range phase2Results {
			if r.Success {
				raw = append(raw, resultEntry{Endpoint: r.Endpoint, Latency: r.Latency.Round(time.Millisecond).String(), Success: true})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": entries,
			"raw":     raw,
			"total":   len(entries),
			"scanned": len(phase2Results),
			"status":  status,
			"phase":   "phase2",
			"phase1":  phase1Progress,
		})
		return
	}

	entries := make([]resultEntry, 0, len(phase1Results))
	for _, r := range phase1Results {
		if !r.Success {
			continue
		}
		entries = append(entries, resultEntry{
			Endpoint: r.Endpoint,
			Latency:  r.Latency.Round(time.Millisecond).String(),
			Success:  true,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries":     entries,
		"total":       len(entries),
		"scanned":     phase1Progress,
		"phase1_total": phase1Total,
		"status":      status,
		"phase":       "phase1",
		"phase1":      phase1Progress,
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
	Fingerprint string `json:"fingerprint"`
	Protocol    string `json:"protocol"`
	UUID        string `json:"uuid"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	Encryption  string `json:"encryption"`
	Security    string `json:"security"`
	SNI         string `json:"sni"`
	FingerprintFP string `json:"fingerprint_fp"`
	Network     string `json:"network"`
	Host        string `json:"host"`
	Path        string `json:"path"`
	PacketEnc   string `json:"packet_enc"`
	Remark      string `json:"remark"`
}

func handleReplacerFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}

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
		entries = append(entries, replacerConfigEntry{
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
		})
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
		entries = append(entries, replacerConfigEntry{
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
		})
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
		configs = append(configs, &ProxyConfig{
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
		})
	}

	urls := GenerateReplacedConfigs(configs, req.Endpoints)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"urls":  urls,
		"count": len(urls),
	})
}
