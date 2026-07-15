package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActiveJobCount(t *testing.T) {
	cases := []struct {
		name     string
		statuses []string
		want     int
	}{
		{"empty", nil, 0},
		{"all terminal", []string{"done", "cancelled"}, 0},
		{"running counts", []string{"running", "done"}, 1},
		{"empty status counts", []string{""}, 1},
		{"clean active statuses", []string{"pending", "running-phase1", "running-phase2", "done"}, 3},
		{"mixed", []string{"running", "cancelled", "pending", "done"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := activeJobCount(tc.statuses); got != tc.want {
				t.Fatalf("activeJobCount(%v)=%d want %d", tc.statuses, got, tc.want)
			}
		})
	}
}

func TestIsActiveJobStatus(t *testing.T) {
	if isActiveJobStatus("done") || isActiveJobStatus("cancelled") {
		t.Fatal("terminal statuses must not be active")
	}
	for _, s := range []string{"", "running", "pending", "running-phase1", "running-phase2"} {
		if !isActiveJobStatus(s) {
			t.Fatalf("%q should be active", s)
		}
	}
}

func TestHandleScanStartRejectsOverCap(t *testing.T) {
	// Isolate global maps for this test.
	scanJobsMu.Lock()
	prev := scanJobs
	scanJobs = map[string]*ScanJob{
		"job_hold_1": {ID: "job_hold_1", Status: "running", Cancel: make(chan struct{})},
		"job_hold_2": {ID: "job_hold_2", Status: "running", Cancel: make(chan struct{})},
	}
	scanJobsMu.Unlock()
	t.Cleanup(func() {
		scanJobsMu.Lock()
		scanJobs = prev
		scanJobsMu.Unlock()
	})

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("params", `{"count":1,"ipv4":true}`); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scan", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	handleScanStart("xray")(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s want 429", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "too many concurrent scans") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestHandleScanStartIgnoresTerminalJobsForCap(t *testing.T) {
	scanJobsMu.Lock()
	prev := scanJobs
	scanJobs = map[string]*ScanJob{
		"job_done":   {ID: "job_done", Status: "done", Cancel: make(chan struct{})},
		"job_cancel": {ID: "job_cancel", Status: "cancelled", Cancel: make(chan struct{})},
	}
	scanJobsMu.Unlock()
	t.Cleanup(func() {
		scanJobsMu.Lock()
		// Drop any job created by this test so runScan can't race after restore.
		for id, j := range scanJobs {
			if id != "job_done" && id != "job_cancel" {
				j.stop()
				delete(scanJobs, id)
			}
		}
		scanJobs = prev
		scanJobsMu.Unlock()
	})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("params", `{"count":1,"ipv4":true}`); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scan", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	handleScanStart("xray")(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s want 200 (terminal jobs must not count)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] == "" {
		t.Fatalf("expected job id in response: %v", resp)
	}
	// Stop the just-started job so it doesn't keep work running in the package.
	scanJobsMu.Lock()
	if j, ok := scanJobs[resp["id"]]; ok {
		j.stop()
	}
	scanJobsMu.Unlock()
}

func TestHandleCleanScanStartRejectsOverCap(t *testing.T) {
	cleanJobsMu.Lock()
	prev := cleanJobs
	cleanJobs = map[string]*CleanIPJob{
		"clean_hold_1": {ID: "clean_hold_1", Status: "pending", Cancel: make(chan struct{})},
		"clean_hold_2": {ID: "clean_hold_2", Status: "running-phase1", Cancel: make(chan struct{})},
	}
	cleanJobsMu.Unlock()
	t.Cleanup(func() {
		cleanJobsMu.Lock()
		cleanJobs = prev
		cleanJobsMu.Unlock()
	})

	payload := `{"one_phase":true,"count":1,"ipv4":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/clean-scan", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleCleanScanStart("xray")(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s want 429", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "too many concurrent scans") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestScanAndCleanCapsAreIndependent(t *testing.T) {
	// Fill clean jobs to the cap; scan start must still succeed.
	cleanJobsMu.Lock()
	prevClean := cleanJobs
	cleanJobs = map[string]*CleanIPJob{
		"clean_a": {ID: "clean_a", Status: "running-phase2", Cancel: make(chan struct{})},
		"clean_b": {ID: "clean_b", Status: "pending", Cancel: make(chan struct{})},
	}
	cleanJobsMu.Unlock()

	scanJobsMu.Lock()
	prevScan := scanJobs
	scanJobs = map[string]*ScanJob{}
	scanJobsMu.Unlock()

	t.Cleanup(func() {
		scanJobsMu.Lock()
		for id, j := range scanJobs {
			j.stop()
			delete(scanJobs, id)
		}
		scanJobs = prevScan
		scanJobsMu.Unlock()
		cleanJobsMu.Lock()
		cleanJobs = prevClean
		cleanJobsMu.Unlock()
	})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("params", `{"count":1,"ipv4":true}`); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/scan", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	handleScanStart("xray")(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("scan blocked by clean cap: status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["id"] != "" {
		scanJobsMu.Lock()
		if j, ok := scanJobs[resp["id"]]; ok {
			j.stop()
		}
		scanJobsMu.Unlock()
	}
}
