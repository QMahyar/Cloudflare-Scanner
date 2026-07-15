# Plan 017: httptest tests for status/stop/results and CSRF middleware

> **Executor instructions**: Tests only preferred. Step by step.
> **Drift check**: `git diff --stat 380c55e..HEAD -- httpserver.go scan_handlers.go cleanscan_handlers.go job_limit_test.go security_test.go`

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Job lifecycle handlers and `csrfMiddleware` have ~0% coverage. Cap and apply
paths have tests; status/stop/results and CSRF decision matrix do not. Pure
httptest tests need no network/xray.

## Current state

- `csrfMiddleware` in `httpserver.go` ~253–285
- Scan handlers: `handleScanStop`, `handleScanStatus`, `handleScanResults`
- Clean handlers: analogues
- Pattern: `job_limit_test.go` swaps global `scanJobs` map under mutex with cleanup

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| Targeted | `go test -run 'CSRF|ScanStatus|ScanStop|ScanResults|CleanStatus' .` | pass |

## Scope

**In scope**: new/extended `*_test.go` (e.g. `security_test.go`, `job_limit_test.go`, or `handlers_test.go`)

**Out of scope**: production behavior changes; live SSE long-stream tests; frontend.

## Git workflow

- Branch: `advisor/017-httptest-handlers-and-csrf`
- Commit: `test: httptest coverage for CSRF and job status/stop/results`

## Steps

### Step 1: CSRF middleware matrix

Construct middleware with fixed token `"test-token"`. Table:

| Host | Method | Path | Cookie | Header | Want |
|------|--------|------|--------|--------|------|
| evil.com | GET | /api/version | — | — | 403 |
| 127.0.0.1 | GET | /api/version | — | — | 200 (next) |
| 127.0.0.1 | POST | /api/scan | missing | — | 403 |
| 127.0.0.1 | POST | /api/scan | token | token | 200 |
| 127.0.0.1 | GET | /api/update-check | missing | — | 403 (needsToken forced) |
| 127.0.0.1 | GET | /api/select-output-dir | token | token | reaches next (may 500 without picker — accept non-403) |

Use a stub `next` that writes 200 `"ok"`.

**Verify**: tests pass.

### Step 2: Scan status/stop/results

Install a fake job in `scanJobs`:

```go
scanJobsMu.Lock()
prev := scanJobs
scanJobs = map[string]*ScanJob{ "job_t": { ID:"job_t", Status:"running", Results: []ScanResult{{Endpoint:"1.1.1.1:2408", Success:true, Latency: time.Millisecond}}, Cancel: make(chan struct{}), OutCount: 10 } }
scanJobsMu.Unlock()
t.Cleanup(...)
```

- GET status via `httptest` + `handleScanStatus` with `PathValue` — set
  `req.SetPathValue("id", "job_t")` (Go 1.22+).
- POST stop → status becomes cancellable via `job.stop()`; response 200.
- GET results → JSON has entries/raw.

Mirror one clean-job test if time permits.

**Verify**: `go test -run 'ScanStatus|ScanStop|ScanResults' .` pass.

### Step 3: Full suite

**Verify**: `go test ./...`.

## Done criteria

- [ ] CSRF allow/deny cases locked in tests
- [ ] At least status + stop + results for scan jobs covered
- [ ] Full suite green; no prod changes unless tiny bugfix discovered (STOP if large)

## STOP conditions

- `SetPathValue` unavailable (old Go) — use request URL patterns compatible with
  handlers; handlers use `r.PathValue("id")` so SetPathValue is required on 1.22+.
- Global map races with parallel tests — use sequential `t.Run` or mutex cleanup carefully.

## Maintenance notes

- Prefer not starting real `runScan` goroutines in these tests.
