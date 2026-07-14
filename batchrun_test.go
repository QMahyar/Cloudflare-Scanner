package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeResult is a minimal result type for exercising runBatches without xray.
type fakeResult struct {
	ep       string
	ok       bool
	attempts int
}

// TestRunBatchesCoversAllEndpoints: every endpoint appears in onBatch output
// exactly once across all batches.
func TestRunBatchesCoversAllEndpoints(t *testing.T) {
	eps := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		eps = append(eps, string(rune('a'+i%26))+string(rune('0'+i/26)))
	}
	var mu sync.Mutex
	seen := map[string]int{}
	var port int32
	runBatches[fakeResult](
		context.Background(), make(chan struct{}), eps, 16, 4,
		func() int { return int(atomic.AddInt32(&port, 1)) },
		func(batch []string, basePort int) []fakeResult {
			out := make([]fakeResult, len(batch))
			for i, ep := range batch {
				out[i] = fakeResult{ep: ep, ok: true}
			}
			return out
		},
		func(r fakeResult) bool { return r.ok },
		func(r *fakeResult) { r.attempts = 2 },
		func(res []fakeResult) {
			mu.Lock()
			for _, r := range res {
				seen[r.ep]++
			}
			mu.Unlock()
		},
	)
	if len(seen) != len(eps) {
		t.Fatalf("saw %d distinct endpoints, want %d", len(seen), len(eps))
	}
	for ep, n := range seen {
		if n != 1 {
			t.Errorf("endpoint %s seen %d times, want 1", ep, n)
		}
	}
}

// TestRunBatchesRetriesPartialFailureOnce: a batch with some (not all) failures
// re-validates exactly the failures, once; a wholly-failed batch is NOT retried.
func TestRunBatchesRetryPolicy(t *testing.T) {
	// One batch of 4: "good" passes, "bad-retryable" fails first then passes on
	// retry, "bad-permanent" always fails. A separate all-fail batch must never retry.
	eps := []string{"good", "bad-retryable", "bad-permanent", "good2"}

	var validateCalls int32
	// track how many times each endpoint is validated
	var callMu sync.Mutex
	calls := map[string]int{}

	firstSeen := map[string]bool{}
	var seenMu sync.Mutex

	runBatches[fakeResult](
		context.Background(), make(chan struct{}), eps, 16, 1,
		func() int { return 1 },
		func(batch []string, basePort int) []fakeResult {
			atomic.AddInt32(&validateCalls, 1)
			out := make([]fakeResult, len(batch))
			for i, ep := range batch {
				callMu.Lock()
				calls[ep]++
				callMu.Unlock()
				ok := true
				switch ep {
				case "bad-permanent":
					ok = false
				case "bad-retryable":
					// fail on first validate, pass on retry
					seenMu.Lock()
					if !firstSeen[ep] {
						firstSeen[ep] = true
						ok = false
					}
					seenMu.Unlock()
				}
				out[i] = fakeResult{ep: ep, ok: ok}
			}
			return out
		},
		func(r fakeResult) bool { return r.ok },
		func(r *fakeResult) { r.attempts = 2 },
		func(res []fakeResult) {},
	)

	// good/good2 validated once; the two failures trigger one retry batch, so
	// bad-retryable and bad-permanent are each validated twice.
	if calls["good"] != 1 || calls["good2"] != 1 {
		t.Errorf("passing endpoints should validate once: %v", calls)
	}
	if calls["bad-retryable"] != 2 || calls["bad-permanent"] != 2 {
		t.Errorf("partial failures should validate twice (one retry): %v", calls)
	}
	// main batch (1) + one retry batch (1) = 2 validate calls.
	if got := atomic.LoadInt32(&validateCalls); got != 2 {
		t.Errorf("expected 2 validate calls (main + one retry), got %d", got)
	}
}

// TestRunBatchesNoRetryOnTotalFailure: when every endpoint in a batch fails, no
// retry batch is spawned (systemic failure).
func TestRunBatchesNoRetryOnTotalFailure(t *testing.T) {
	eps := []string{"x", "y", "z"}
	var validateCalls int32
	runBatches[fakeResult](
		context.Background(), make(chan struct{}), eps, 16, 1,
		func() int { return 1 },
		func(batch []string, basePort int) []fakeResult {
			atomic.AddInt32(&validateCalls, 1)
			out := make([]fakeResult, len(batch))
			for i, ep := range batch {
				out[i] = fakeResult{ep: ep, ok: false}
			}
			return out
		},
		func(r fakeResult) bool { return r.ok },
		func(r *fakeResult) { r.attempts = 2 },
		func(res []fakeResult) {},
	)
	if got := atomic.LoadInt32(&validateCalls); got != 1 {
		t.Errorf("total failure must not retry: expected 1 validate call, got %d", got)
	}
}

// TestRunBatchesCancelShortCircuits: a pre-cancelled channel means (near) no work
// and the cancelled flag is returned.
func TestRunBatchesCancelShortCircuits(t *testing.T) {
	eps := make([]string, 100)
	for i := range eps {
		eps[i] = "e"
	}
	cancel := make(chan struct{})
	close(cancel) // cancelled before we start
	var validateCalls int32
	cancelled := runBatches[fakeResult](
		context.Background(), cancel, eps, 16, 4,
		func() int { return 1 },
		func(batch []string, basePort int) []fakeResult {
			atomic.AddInt32(&validateCalls, 1)
			out := make([]fakeResult, len(batch))
			return out
		},
		func(r fakeResult) bool { return r.ok },
		func(r *fakeResult) {},
		func(res []fakeResult) {},
	)
	if !cancelled {
		t.Error("expected cancelled=true when cancel channel is closed")
	}
	if got := atomic.LoadInt32(&validateCalls); got != 0 {
		t.Errorf("cancelled run should do no validate work, got %d calls", got)
	}
}
