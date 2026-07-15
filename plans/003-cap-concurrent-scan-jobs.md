# Plan 003: Cap concurrent scan and clean jobs

> **Executor instructions**: Follow step by step; verify each step; STOP on
> mismatch. Reviewer maintains the index.
>
> **Drift check**: `git diff --stat 5945765..HEAD -- httpserver.go scan_handlers.go cleanscan_handlers.go`

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: correctness
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

Every `POST /api/scan` and `POST /api/clean-scan` inserts a job and starts a
goroutine with no limit. Inputs are already clamped (`maxScanCount=100000`,
phase probes, ports), but **N concurrent jobs × those caps** can exhaust CPU,
FDs, and xray port bands on a single machine (multi-tab, accidental double-start
races, or a local script). The UI disables Start only for its own tab state;
nothing stops a second tab or a raw HTTP client.

## Current state

```go
// httpserver.go
scanJobs   = map[string]*ScanJob{}
cleanJobs  = map[string]*CleanIPJob{}
// no max concurrent constant
```

`handleScanStart` / `handleCleanScanStart` always:

1. allocate job id under mutex
2. insert into map
3. `go runScan` / `go runCleanScan`

`stopAllJobs` already walks both maps on shutdown.

## Design

- Add `const maxConcurrentJobs = 2` in `httpserver.go` next to other caps
  (one endpoint + one clean is the normal multi-tab case; 2 is enough headroom
  without allowing unbounded pile-up. If you prefer 3, document why in NOTES —
  do not exceed 4).
- Count only jobs whose `Status` is **not** terminal (`done` / `cancelled`).
  Terminal jobs stay in the map until TTL for result polling — they must **not**
  count against the cap.
- Before insert, under the same mutex that protects the map:
  - count non-terminal jobs in **that** map (scan and clean are separate caps
    of `maxConcurrentJobs` each — a clean scan should not block a WARP scan).
  - if count >= max, unlock and `jsonError(w, "too many concurrent scans", 429)`.
- Do **not** change job IDs, TTL, or cleanup.

Optional pure helper for testability:

```go
func countActiveScanJobs() int // must hold scanJobsMu OR accept map snapshot
```

Prefer testing via a small exported/package-level helper:

```go
func activeJobCount(statuses []string) int {
  n := 0
  for _, s := range statuses {
    if s != "done" && s != "cancelled" {
      n++
    }
  }
  return n
}
```

Or count by ranging the map under lock inside the handlers.

Statuses used today:

- Scan: `running`, `done`, `cancelled` (and possibly empty before set — treat
  empty/non-terminal as active).
- Clean: `pending`, `running-phase1`, `running-phase2`, `done`, `cancelled`.

Active = not `done` and not `cancelled`.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./... -count=1` | pass |
| Vet | `go vet ./...` | exit 0 |

## Scope

**In scope**:

- `httpserver.go` (constant + optional helper)
- `scan_handlers.go` (`handleScanStart` gate)
- `cleanscan_handlers.go` (`handleCleanScanStart` gate)
- New or existing `*_test.go` for the active-count helper and/or handler 429

**Out of scope**:

- Frontend error toast for 429 (optional nice-to-have — only if trivial one-liner
  already has error handling; do not redesign UI)
- Changing maxScanCount or probe caps
- Global single-flight across scan+clean (separate caps by design)

## Git workflow

- Branch: `advisor/003-cap-concurrent-scan-jobs`
- Commit: `fix: cap concurrent scan and clean jobs`

## Steps

### Step 1: Add constant + gate both start handlers

Under the mutex, before storing the new job:

```go
active := 0
for _, j := range scanJobs {
    j.mu.Lock()
    st := j.Status
    j.mu.Unlock()
    if st != "done" && st != "cancelled" {
        active++
    }
}
if active >= maxConcurrentJobs {
    scanJobsMu.Unlock()
    jsonError(w, "too many concurrent scans (max 2)", 429)
    return
}
```

Mirror for clean jobs. Avoid double-locking deadlocks: if you already hold
`scanJobsMu`, only take `j.mu` briefly (current code already does similar).

**Note**: New jobs are inserted with `Status: "running"` or `"pending"` before
the goroutine runs — they count immediately. Good.

**Verify**: compile via tests.

### Step 2: Tests

- Unit test `active` classification if you extracted a helper.
- Optional: insert fake jobs into `scanJobs` under test (package main tests can),
  call handler, expect 429, then clean up maps in `t.Cleanup`.

Be careful to not leak global map state across tests — always delete keys in cleanup.

**Verify**: `go test ./... -count=1`

## Done criteria

- [ ] `maxConcurrentJobs` constant exists
- [ ] Both start handlers return 429 when active jobs >= cap
- [ ] Terminal jobs do not count
- [ ] Scan and clean caps are independent
- [ ] Full test suite passes; gofmt clean

## STOP conditions

- Status string set differs (e.g. new status values) — re-read handlers and adapt
  the active predicate; if unclear, stop.
- Taking `j.mu` while holding map mutex causes existing test deadlocks — restructure
  (copy status under lock carefully) rather than removing the cap.

## Maintenance notes

- If product later wants queueing instead of 429, replace the error path only.
- Document the limit in FAQ only if users hit it (plan 008 can mention).
