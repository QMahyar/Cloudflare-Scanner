package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleCleanScanResultsReturnsBothPhases(t *testing.T) {
	job := &CleanIPJob{
		ID:            "both-phases",
		Status:        "running-phase2",
		Phase1Results: []CleanIPResult{{Endpoint: "1.1.1.1:443", Success: true, Latency: 20 * time.Millisecond}},
		Phase2Results: []CleanIPResult{{Endpoint: "2.2.2.2:443", Success: true, Latency: 30 * time.Millisecond}},
		Phase2Total:   1,
		Cancel:        make(chan struct{}),
	}

	cleanJobsMu.Lock()
	previous := cleanJobs
	cleanJobs = map[string]*CleanIPJob{job.ID: job}
	cleanJobsMu.Unlock()
	t.Cleanup(func() {
		cleanJobsMu.Lock()
		cleanJobs = previous
		cleanJobsMu.Unlock()
	})

	req := httptest.NewRequest(http.MethodGet, "/api/clean-results/"+job.ID, nil)
	req.SetPathValue("id", job.ID)
	rr := httptest.NewRecorder()
	handleCleanScanResults(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	var response struct {
		Entries []struct {
			Endpoint string `json:"endpoint"`
		} `json:"entries"`
		Phase1Entries []struct {
			Endpoint string `json:"endpoint"`
		} `json:"phase1_entries"`
		Phase2Entries []struct {
			Endpoint string `json:"endpoint"`
		} `json:"phase2_entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Phase1Entries) != 1 || response.Phase1Entries[0].Endpoint != "1.1.1.1:443" {
		t.Fatalf("phase1_entries=%+v", response.Phase1Entries)
	}
	if len(response.Phase2Entries) != 1 || response.Phase2Entries[0].Endpoint != "2.2.2.2:443" {
		t.Fatalf("phase2_entries=%+v", response.Phase2Entries)
	}
	if len(response.Entries) != 1 || response.Entries[0].Endpoint != "2.2.2.2:443" {
		t.Fatalf("entries=%+v", response.Entries)
	}
}
