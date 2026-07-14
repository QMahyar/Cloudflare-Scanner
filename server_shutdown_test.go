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
