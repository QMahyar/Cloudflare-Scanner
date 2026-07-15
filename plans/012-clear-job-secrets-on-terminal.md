# Plan 012: Clear job config secrets when scans reach terminal status

> **Executor instructions**: Step by step; verify; STOP if stuck.
> **Drift check**: `git diff --stat 380c55e..HEAD -- scan_handlers.go cleanip.go server_shutdown_test.go`

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: security
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Plan 006 already drops finished jobs' **endpoint input slices**. The uploaded
WARP `PrivateKey` (`ScanJob.Config`) and clean-scan `ProxyConfig` UUID/password
(`CleanIPJob.Config`) still sit in process memory for the full `jobTTL`
(10 minutes) after the job is done/cancelled. Results polling does not need
them. Nil them at the same release points.

## Current state

```go
// scan_handlers.go
func releaseScanJobInputs(job *ScanJob) {
 job.Endpoints = nil
}

// cleanip.go
func releaseCleanJobInputs(job *CleanIPJob) {
 job.Endpoints = nil
}
```

Both are called under `job.mu` when status becomes `done` or `cancelled`.
`server_shutdown_test.go` already exercises release helpers.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| Targeted | `go test -run Release\|Shutdown .` | pass |

## Scope

**In scope**:

- `scan_handlers.go` (`releaseScanJobInputs`)
- `cleanip.go` (`releaseCleanJobInputs`)
- `server_shutdown_test.go` (extend assertions)

**Out of scope**: disk temp conf cleanup (already deferred Remove), frontend,
xray config files on disk (already 0600 under TempDir).

## Git workflow

- Branch: `advisor/012-clear-job-secrets-on-terminal`
- Commit: `security: nil job config secrets when scans finish`

## Steps

### Step 1: Nil configs in release helpers

```go
func releaseScanJobInputs(job *ScanJob) {
 job.Endpoints = nil
 job.Config = nil
}

func releaseCleanJobInputs(job *CleanIPJob) {
 job.Endpoints = nil
 job.Config = nil
}
```

Update comments to mention credentials/config, not only endpoint slices.

**Important**: Confirm no code path after `release*JobInputs` still reads
`job.Config` on that job. Grep: after release, only SNI was needed earlier in
`runCleanScan` (copied to locals before phase 2). Noise path already finished
using scanner-local config. If you find a post-release use, STOP.

**Verify**: `grep -n "releaseScanJobInputs\|releaseCleanJobInputs\|job.Config" scan_handlers.go cleanip.go` — release sets nil; no use after release in same function after the call without re-check.

### Step 2: Extend unit tests

In `server_shutdown_test.go` (or adjacent), after calling release helpers on a
job with non-nil Config and Endpoints, assert both are nil.

**Verify**: `go test -run Release\|Shutdown .` pass.

### Step 3: Full suite

**Verify**: `go test ./...` && `go vet ./...`.

## Done criteria

- [ ] Terminal jobs do not retain `Config` pointers
- [ ] Tests assert nil Config + Endpoints
- [ ] Full suite green; scope clean

## STOP conditions

- A post-release code path still needs Config (report which).
- Finding already fixed.

## Maintenance notes

- Any new field holding secrets on the job struct must be cleared here too.
- Reviewer: check cancelled paths all call release (they should already).
