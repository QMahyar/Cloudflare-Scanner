package main

import "testing"

// job.stop() must be safe to call repeatedly (concurrent /api/stop + shutdown):
// a second close of the Cancel channel would panic without the sync.Once guard.
func TestScanJobStopIdempotent(t *testing.T) {
	j := &ScanJob{Cancel: make(chan struct{})}
	j.stop()
	j.stop() // must not panic
	select {
	case <-j.Cancel:
	default:
		t.Fatal("Cancel channel should be closed after stop()")
	}
}

// stopAllJobs must be safe to call with empty job maps (the common shutdown case
// when no scan is running).
func TestStopAllJobsEmpty(t *testing.T) {
	scanJobsMu.Lock()
	scanJobs = map[string]*ScanJob{}
	scanJobsMu.Unlock()
	cleanJobsMu.Lock()
	cleanJobs = map[string]*CleanIPJob{}
	cleanJobsMu.Unlock()
	stopAllJobs() // must not panic
}

func TestReleaseScanJobInputsNilsEndpoints(t *testing.T) {
	job := &ScanJob{
		Endpoints: []string{"1.2.3.4:2408", "5.6.7.8:2408"},
		Config:    &WarpConfig{PrivateKey: "secret-private-key"},
		Results:   []ScanResult{{Endpoint: "1.2.3.4:2408", Success: true}},
	}
	releaseScanJobInputs(job)
	if job.Endpoints != nil {
		t.Fatalf("Endpoints should be nil after release, got %v", job.Endpoints)
	}
	if job.Config != nil {
		t.Fatalf("Config should be nil after release, got %+v", job.Config)
	}
	if len(job.Results) != 1 {
		t.Fatalf("Results should be preserved, got %d", len(job.Results))
	}
}

func TestReleaseCleanJobInputsNilsEndpoints(t *testing.T) {
	job := &CleanIPJob{
		Endpoints:     []string{"1.2.3.4:443"},
		Config:        &ProxyConfig{UUID: "secret-uuid", PublicKey: "secret-public-key"},
		Phase1Results: []CleanIPResult{{Endpoint: "1.2.3.4:443", Success: true}},
	}
	releaseCleanJobInputs(job)
	if job.Endpoints != nil {
		t.Fatalf("Endpoints should be nil after release, got %v", job.Endpoints)
	}
	if job.Config != nil {
		t.Fatalf("Config should be nil after release, got %+v", job.Config)
	}
	if len(job.Phase1Results) != 1 {
		t.Fatalf("Phase1Results should be preserved, got %d", len(job.Phase1Results))
	}
}
