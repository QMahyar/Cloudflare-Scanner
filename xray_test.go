package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain doubles as the fake xray for TestBatchProbeDetectsEarlyExit: when the
// test binary is re-executed with GO_FAKE_XRAY=1 in its env (which BatchProbe's
// exec.Command inherits from the test process), it prints a config error to
// stderr and exits immediately instead of running tests.
func TestMain(m *testing.M) {
	if os.Getenv("GO_FAKE_XRAY") == "1" {
		fmt.Fprintln(os.Stderr, "failed to parse config: invalid outbound")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestBatchProbeDetectsEarlyExit verifies BatchProbe notices a process that dies
// right after spawn and surfaces the real error from stderr/log instead of
// polling the dead port for the full startup budget. Regression for the dead
// cmd.ProcessState check: ProcessState is only populated by cmd.Wait(), which
// used to live in the deferred cleanup AFTER the poll loop, so a fake xray that
// exited immediately used to burn the whole ~6.2s budget per batch.
func TestBatchProbeDetectsEarlyExit(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GO_FAKE_XRAY", "1")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	results := BatchProbe(ctx, os.Args[0], configPath, []string{"1.2.3.4:443"}, 1, 5*time.Second,
		func(ctx context.Context, endpoint string, socksPort int) probeResult {
			return probeResult{Endpoint: endpoint, Error: "not reached"}
		})
	elapsed := time.Since(start)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Error, "startup timeout") {
		t.Fatalf("expected a startup-timeout error, got %q", results[0].Error)
	}
	if !strings.Contains(results[0].Error, "failed to parse config") {
		t.Errorf("expected the stderr cause in the error, got %q", results[0].Error)
	}
	// The startup budget for a 1-endpoint batch is ~6.2s. Early-exit detection
	// should fire within a couple of dials/ticker ticks (~100ms), not the full
	// budget — the old dead-code check took 6.2s to report the same error.
	if elapsed > 2*time.Second {
		t.Errorf("early-exit detection took %v — the process exited immediately; expected well under the 6.2s startup budget", elapsed)
	}
}
