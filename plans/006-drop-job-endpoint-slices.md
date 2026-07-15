# Plan 006: Drop retained endpoint slices after jobs finish

> **Executor instructions**: Follow step by step. Reviewer maintains index.
>
> **Drift check**: `git diff --stat 5945765..HEAD -- scan_handlers.go cleanip.go`

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: MED
- **Depends on**: none (coordinate mentally with 003; do not conflict on job structs)
- **Category**: perf
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

Finished jobs stay in `scanJobs` / `cleanJobs` for `jobTTL` (10 minutes) so the
UI can poll results. Each job still holds `Endpoints []string` (up to 100k) even
though no code path after completion re-reads that slice — only `Results` /
`Phase*Results` are served. Dropping `Endpoints` (and any other pure-input
slices no longer needed) at terminal status cuts RSS for large scans.

## Current state

- `ScanJob` has `Endpoints []string` used during `runScan` / `runScanNoiseBatched`.
- After workers finish, status set to `done`/`cancelled` and `Results` sorted —
  `Endpoints` never nilled (`scan_handlers.go` end of `runScan` and
  `runScanNoiseBatched`).
- `CleanIPJob.Endpoints` used in `runCleanScan` phase 1; after phase 1 (or at
  latest when status becomes `done`/`cancelled`), safe to nil.
- Handlers for status/results only copy Results / Phase results / progress fields
  (`handleScanResults`, `handleCleanScanResults`).

## Design

At every terminal transition (`done` or `cancelled`), under the job mutex:

```go
job.Endpoints = nil
```

For clean jobs, also consider nilling only after phase 1 no longer needs them —
simplest correct approach: nil when setting final status (done/cancelled), not
earlier mid-phase2 if any path still references `job.Endpoints` (grep first).

Do **not** nil `Results` / `Phase1Results` / `Phase2Results` / nearby results —
UI needs them until TTL.

Optional: nil `Config` pointer only if nothing in results handlers needs it —
**grep first**; if export uses stored config from job, keep it. Prefer only
`Endpoints`.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Grep safety | `rg -n "job\\.Endpoints|\\.Endpoints" --type go -g '!*_test.go'` | only generation + run loops |
| Tests | `go test ./... -count=1` | pass |
| Vet | `go vet ./...` | exit 0 |

## Scope

**In scope**:

- `scan_handlers.go` (terminal paths in `runScan`, `runScanNoiseBatched`)
- `cleanip.go` (all places that set status to done/cancelled)

**Out of scope**:

- Changing jobTTL
- Compacting Results
- Frontend

## Git workflow

- Branch: `advisor/006-drop-job-endpoint-slices`
- Commit: `perf: drop endpoint lists from finished scan jobs`

## Steps

### Step 1: Audit references

Run the grep above. Confirm no status/results handler reads `Endpoints` after run.

### Step 2: Nil at terminal status

Every `job.Status = "done"` or `"cancelled"` assignment in run paths should also
set `job.Endpoints = nil` under the same `job.mu` lock when possible.

Early-return cancel paths in `runCleanScan` must be included.

### Step 3: Test (lightweight)

If easy: package-level test that constructs a `ScanJob`, pretends terminal
cleanup via a tiny helper `releaseJobInputs(job *ScanJob)` you extract — optional.
Not required if grep + suite pass; prefer a 5-line helper for clarity:

```go
func releaseScanJobInputs(job *ScanJob) {
    job.Endpoints = nil
}
```

call under lock at end of runs.

**Verify**: `go test ./... -count=1`, `go vet ./...`

## Done criteria

- [ ] Terminal scan/clean jobs do not retain Endpoints
- [ ] Results still available until TTL
- [ ] Suite green

## STOP conditions

- Any results/export handler still needs Endpoints — stop and report.
- Concurrent mutation without holding `job.mu` — fix locking, don't race.

## Maintenance notes

- If a "rescan same endpoints" feature is added later, either keep Endpoints or
  regenerate from request params (better).
