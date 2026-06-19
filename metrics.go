package main

import (
	"sort"
	"time"
)

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]time.Duration(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	mid := len(copyValues) / 2
	if len(copyValues)%2 == 1 {
		return copyValues[mid]
	}
	return (copyValues[mid-1] + copyValues[mid]) / 2
}

func bestDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	best := values[0]
	for _, v := range values[1:] {
		if v < best {
			best = v
		}
	}
	return best
}

func jitterDuration(values []time.Duration) time.Duration {
	if len(values) < 2 {
		return 0
	}
	copyValues := append([]time.Duration(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	return copyValues[len(copyValues)-1] - copyValues[0]
}

func sortScanResults(results []ScanResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Success != results[j].Success {
			return results[i].Success
		}
		return results[i].Latency < results[j].Latency
	})
}

func sortCleanIPResults(results []CleanIPResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Success != results[j].Success {
			return results[i].Success
		}
		return results[i].Latency < results[j].Latency
	})
}

// lossPercent is the fraction of probe attempts that did not respond, as a
// 0–100 percentage. attempts <= 0 (no probes run) reports 0 — "unknown", not
// "perfect" — so it never inflates a score for an endpoint we never measured.
func lossPercent(passes, attempts int) float64 {
	if attempts <= 0 {
		return 0
	}
	if passes < 0 {
		passes = 0
	}
	if passes > attempts {
		passes = attempts
	}
	return 100 * float64(attempts-passes) / float64(attempts)
}

// qualityScore folds latency, jitter, and packet loss into a single 0–100 rank
// (higher is better) so results can be ordered by overall quality, not latency
// alone — the way a best-in-class scanner picks an IP. Weights: latency 50,
// jitter 15, loss 35. Each term degrades to 0 past a "clearly bad" ceiling
// (400ms latency, 150ms jitter, 100% loss). A speed term can be added later
// (the user deferred the throughput test) by reweighting without touching callers.
//
// median <= 0 means we have no usable measurement → 0 (unknown), never a free 100.
func qualityScore(median, jitter time.Duration, lossPct float64) int {
	if median <= 0 {
		return 0
	}
	medMs := float64(median) / float64(time.Millisecond)
	jitMs := float64(jitter) / float64(time.Millisecond)
	if jitMs < 0 {
		jitMs = 0
	}
	if lossPct < 0 {
		lossPct = 0
	}
	if lossPct > 100 {
		lossPct = 100
	}

	latScore := 50 * (1 - clampFloat(medMs/400, 0, 1))
	jitScore := 15 * (1 - clampFloat(jitMs/150, 0, 1))
	lossScore := 35 * (1 - lossPct/100)

	return clampInt(int(latScore+jitScore+lossScore+0.5), 0, 100)
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
