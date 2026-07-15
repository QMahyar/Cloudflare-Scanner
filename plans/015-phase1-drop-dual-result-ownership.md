# Plan 015: Drop dual Phase-1 result ownership and reduce job.mu churn

> **Executor instructions**: Step by step; careful concurrency; STOP if unsure.
> **Drift check**: `git diff --stat 380c55e..HEAD -- cleanip_measure.go cleanip.go`

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED (live progress UX / cancel races)
- **Depends on**: none (nice-after 010)
- **Category**: perf
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

`runCleanPhase1TCP` appends every success to a **local** `results` slice **and**
to `job.Phase1Results` under `job.mu` on every hit. After the function returns,
`runCleanScan` assigns `job.Phase1Results = phase1Results` again (sorted local
copy). Mid-scan the job holds an unsorted growing slice; every success pays a
mutex. For dense ranges this is pure overhead.

## Current state

`cleanip_measure.go` (~146–156):

```go
mu.Lock()
results = append(results, result)
n := len(results)
mu.Unlock()

if job != nil {
 job.mu.Lock()
 job.Phase1Results = append(job.Phase1Results, result)
 job.Phase1Progress++
 job.mu.Unlock()
}
```

`cleanip.go` after phase1:

```go
job.mu.Lock()
job.Phase1Results = phase1Results
job.mu.Unlock()
```

Status/results handlers read `job.Phase1Results` and `Phase1Progress` under lock.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| Vet | `go vet ./...` | clean |

## Scope

**In scope**:

- `cleanip_measure.go` (`runCleanPhase1TCP`)
- `cleanip.go` (only if progress publishing needs a tweak)
- tests only if pure helpers extracted

**Out of scope**: Phase-2 batching, frontend poll cadence, changing result JSON shape.

## Git workflow

- Branch: `advisor/015-phase1-drop-dual-result-ownership`
- Commit: `perf: publish clean phase1 progress without dual result slices`

## Steps

### Step 1: Stop appending full results under job.mu every hit

Keep local `results` as the sole owner of success rows during the dial loop.

Under `job != nil`, on each **tested** outcome (success or failed-but-tested),
only update progress counters, e.g.:

```go
if job != nil {
 job.mu.Lock()
 job.Phase1Progress++
 // optional: do NOT touch Phase1Results here
 job.mu.Unlock()
}
```

Match existing semantics for cancel: cancelled probes that never tested should
not increment progress (already the case in current code for cancel path).

### Step 2: Publish partial results for live UI (choose one approach)

Live UI expects growing phase1 successes mid-scan. Pick **one**:

**Preferred (simple)**: Periodically (e.g. every N successes or every 50ms via
last-publish timestamp) copy `results` under `mu` into `job.Phase1Results`
under `job.mu`. Or publish every 32 successes:

```go
if job != nil && (n%32 == 0 || /* stop */) {
 snapshot := append([]CleanIPResult(nil), results...)
 // sort optional for mid-scan; final sort still happens
 job.mu.Lock()
 job.Phase1Results = snapshot
 job.Phase1Progress = /* keep consistent with tested count if that's what status uses */
 job.mu.Unlock()
}
```

Read status handler fields first (`handleCleanScanStatus` / results) so
`Phase1Progress` meaning stays "tested" or "successes" consistently with
today. Prefer **not** changing the JSON field meanings.

After `wg.Wait()`, existing sort + `job.Phase1Results = phase1Results` in
`runCleanScan` remains the terminal assignment — keep it.

### Step 3: Ensure cancel/stop-after still correct

- stopAfter uses local `n` / `stopNow` — keep.
- job cancel still cancels phaseCtx — keep.
- No late append to job after cancel that re-grows results after terminal
  snapshot (mirror existing guards).

**Verify**: `go test ./...` pass; manually reason cancel path.

## Done criteria

- [ ] No per-success dual append of full `CleanIPResult` to both slices
- [ ] Terminal `job.Phase1Results` still sorted successes as today
- [ ] Progress still moves during scan (throttled publish OK)
- [ ] Tests/vet green

## STOP conditions

- Cannot preserve live progress without large redesign — report and keep
  counter-only progress if results endpoint only matters at end (check UI:
  IpScanner refetches results on status frames; empty mid-scan entries may be
  OK if progress bar uses status fields).
- Race detector needed and unavailable — still ship careful locking.

## Maintenance notes

- Reviewer: watch for unlocked read of `results` while workers append — always
  snapshot under `mu`.
- Nearby phase1 calls `runCleanPhase1TCP` with `job == nil` — must keep working.
