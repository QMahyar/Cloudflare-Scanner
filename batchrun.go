package main

import (
	"context"
	"sync"
	"sync/atomic"
)

// runBatches is the shared pooled-batch orchestrator for the two xray scan paths
// (WARP noise fallback and clean-IP Phase 2). It splits endpoints into batches of
// batchSize, runs up to concurrentBatches validate() calls at once, retries every
// failed endpoint ONCE in a fresh follow-up batch (including all-failed batches —
// a cold xray start, TLS handshake race, or transient edge 403 often wipes a whole
// first attempt, and skipping the retry was the dominant "Phase 2 returns nothing"
// failure mode on otherwise-good networks), marks retried results via markRetried,
// and hands each completed batch to onBatch. It stops launching new batches once
// ctx or cancel fires, and keeps partial results (onBatch decides what to do with
// a late batch). Returns true if it was cancelled mid-run.
//
// allocPort returns a fresh non-overlapping SOCKS base port per validate call;
// the caller owns the port-band math. validate runs one batch against its base
// port. isSuccess reports whether a result passed (drives the retry set). onBatch
// receives the finished batch's results (aligned to its endpoints) and owns all
// normalization, scoring, progress, and locking.
func runBatches[T any](
	ctx context.Context,
	cancel <-chan struct{},
	endpoints []string,
	batchSize, concurrentBatches int,
	allocPort func() int,
	validate func(batch []string, basePort int) []T,
	isSuccess func(T) bool,
	markRetried func(*T),
	onBatch func([]T),
) bool {
	var batches [][]string
	for i := 0; i < len(endpoints); i += batchSize {
		end := i + batchSize
		if end > len(endpoints) {
			end = len(endpoints)
		}
		batches = append(batches, endpoints[i:end])
	}

	sem := make(chan struct{}, concurrentBatches)
	var wg sync.WaitGroup
	var cancelled atomic.Bool

	for _, b := range batches {
		select {
		case <-ctx.Done():
			cancelled.Store(true)
		case <-cancel:
			cancelled.Store(true)
		default:
		}
		if cancelled.Load() {
			break
		}

		wg.Add(1)
		go func(batch []string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			res := validate(batch, allocPort())
			// Defensive: every validate() must return one result per endpoint. A short
			// slice would panic on batch[i] / retryIdx[j]; a long one would drop the
			// tail. Pad / trim so the rest of the pipeline can assume alignment.
			if len(res) != len(batch) {
				fixed := make([]T, len(batch))
				n := copy(fixed, res)
				_ = n
				res = fixed
			}

			var retryIdx []int
			var retryEps []string
			for i, r := range res {
				if !isSuccess(r) {
					retryIdx = append(retryIdx, i)
					retryEps = append(retryEps, batch[i])
				}
			}
			// Always retry failures once, even when the whole batch failed.
			// Total-failure skip used to avoid a second spawn on "broken xray /
			// dead config", but it also discarded recoverable cold-start and
			// transient edge failures — and a single-endpoint batch always
			// looked "total", so it never retried at all.
			if len(retryEps) > 0 {
				select {
				case <-ctx.Done():
				default:
					rres := validate(retryEps, allocPort())
					limit := len(rres)
					if limit > len(retryIdx) {
						limit = len(retryIdx)
					}
					for j := 0; j < limit; j++ {
						rr := rres[j]
						if isSuccess(rr) {
							markRetried(&rr)
							res[retryIdx[j]] = rr
						} else {
							markRetried(&res[retryIdx[j]])
						}
					}
					// Retries that never came back still count as a second attempt.
					for j := limit; j < len(retryIdx); j++ {
						markRetried(&res[retryIdx[j]])
					}
				}
			}

			onBatch(res)
		}(b)
	}

	wg.Wait()
	return cancelled.Load()
}
