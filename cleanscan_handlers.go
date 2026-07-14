package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type cleanScanRequest struct {
	// ConfigURL is the proxy template (vless/trojan/vmess) whose IP:port each
	// validated clean IP is swapped into. VLESSURL is the legacy field name, still
	// accepted for back-compat; ConfigURL wins when both are set.
	ConfigURL       string `json:"config_url"`
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

		// config_url is the modern field; vless_url is accepted for back-compat.
		configURL := req.ConfigURL
		if configURL == "" {
			configURL = req.VLESSURL
		}

		if !req.OnePhase && configURL == "" {
			jsonError(w, "config_url required", 400)
			return
		}

		var cfg *ProxyConfig
		if configURL != "" {
			var err error
			cfg, err = ParseProxyURL(configURL)
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

		// Resolve scan ports — validate, dedupe, and cap. The port count multiplies
		// the IP count into the endpoint slice, so an unbounded/duplicate-laden list
		// could blow up allocation (this is the one clean-scan input left unclamped).
		scanPorts := sanitizeScanPorts(req.Ports)

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
	ConfigURL string   `json:"config_url"` // modern field
	VLESSURL  string   `json:"vless_url"`  // legacy alias, still accepted
	Endpoints []string `json:"endpoints"`
}

// exportFilename derives the download filename from the config's protocol so
// trojan/vmess users don't get a misleading "vless" filename. Falls back to a
// protocol-neutral name when the protocol is empty/unknown.
func exportFilename(prefix, protocol string) string {
	switch protocol {
	case "vless", "trojan", "vmess":
		return fmt.Sprintf("%s_%s.txt", prefix, protocol)
	default:
		return prefix + ".txt"
	}
}

func handleCleanExport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req cleanExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	configURL := req.ConfigURL
	if configURL == "" {
		configURL = req.VLESSURL
	}
	if configURL == "" || len(req.Endpoints) == 0 {
		jsonError(w, "config_url and endpoints required", 400)
		return
	}

	cfg, err := ParseProxyURL(configURL)
	if err != nil {
		jsonError(w, fmt.Sprintf("parse url: %v", err), 400)
		return
	}

	urls := cfg.GenerateExport(req.Endpoints)

	proto := cfg.Protocol
	label := proto
	if label == "" {
		label = "proxy"
	}
	content := fmt.Sprintf("# Clean IP %s Configs\n", strings.ToUpper(label))
	content += "# Generated by Cloudflare Scanner\n"
	content += fmt.Sprintf("# Original: %s\n", configURL)
	content += fmt.Sprintf("# Working IPs: %d\n\n", len(urls))
	for _, u := range urls {
		content += u + "\n"
	}

	filename := exportFilename("clean_ips", proto)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Write([]byte(content))
}
