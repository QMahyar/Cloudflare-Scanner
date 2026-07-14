# Plan 002: Graceful shutdown cancels in-flight jobs, tears down SSE promptly, and logs cleanly

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in "STOP conditions" occurs, stop and report — do not
> improvise. When done, update the status row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on top of commit `6f7a19c`. Do not rely on `git diff` alone — open the
> files in "Current state" and confirm the quoted lines match. On mismatch, STOP.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: bug (correctness)
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

The graceful-shutdown path in `main.go:waitForShutdown` is recently-added
(uncommitted) code with three coupled defects:

1. **Orphaned xray children.** `waitForShutdown` calls `srv.Shutdown(ctx)` then
   `os.RemoveAll(_xray_work)` / `os.RemoveAll(_xray_clean)` then `os.Exit(0)`. It
   never cancels running scan jobs. Scan goroutines run under
   `context.Background()` (not a request context), so `srv.Shutdown` doesn't
   touch them. `os.Exit` skips all `defer`s, including `BatchProbe`'s
   `defer cmd.Process.Kill()` in `xray.go`. Result: Ctrl+C during a Phase-2 or
   noise batch leaves orphaned xray processes holding SOCKS ports, and the
   immediately-following `RemoveAll` races those still-alive processes (on Windows
   the files are locked), leaving the temp dirs behind — exactly the leak the
   project docs warn about.
2. **5-second hang on open SSE.** `srv.Shutdown` waits for connections to go
   idle; the `streamSSE` loop only returns on client disconnect or job-done, so
   an open `/api/scan-events` stream (always open during a scan) prevents idle and
   burns the full 5s timeout every time. The code comment claims SSE streams "are
   torn down immediately" — false.
3. **Spurious error log.** `server.go` filters the `Serve` return with
   `!errors.Is(err, net.ErrClosed)`, but `srv.Shutdown` makes `Serve` return
   `http.ErrServerClosed`, which is NOT filtered — so every clean shutdown prints
   `server error: http: Server closed` to stderr.

Fixing all three together (one shutdown seam) makes Ctrl+C fast, leak-free, and quiet.

## Current state

- `main.go:94-113` — `waitForShutdown`:
  ```go
  func waitForShutdown(srv *http.Server) {
      sig := make(chan os.Signal, 1)
      signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
      <-sig
      fmt.Println("\n  Shutting down — cleaning up xray work dirs...")
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      if err := srv.Shutdown(ctx); err != nil {
          fmt.Fprintf(os.Stderr, "  server shutdown: %v\n", err)
      }
      os.RemoveAll(filepath.Join(os.TempDir(), "_xray_work"))
      os.RemoveAll(filepath.Join(os.TempDir(), "_xray_clean"))
      os.Exit(0)
  }
  ```
- `server.go:354-358` — the Serve error filter:
  ```go
  go func() {
      if err := srv.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
          fmt.Fprintf(os.Stderr, "server error: %v\n", err)
      }
  }()
  ```
- `server.go:34-53` — `ScanJob` struct; `server.go:162-166` — `(*ScanJob).stop()`
  uses `cancelOnce.Do(func(){ close(j.Cancel) })` (idempotent). `CleanIPJob` has an
  equivalent `stop()` at `cleanip.go:350` with its own `cancelOnce`.
- Job maps + mutexes: `server.go:183-189` — `scanJobs`/`scanJobsMu`,
  `cleanJobs`/`cleanJobsMu`.
- `server.go:832-875` — `streamSSE`; its loop selects on `r.Context().Done()` and
  a 250ms ticker, returns when the snapshot reports done.
- The scan goroutine bridges `job.Cancel` → context cancel: `server.go:515-521`
  (in `runScan`) and the clean equivalent in `runCleanScan` (`cleanip.go:964`).
  So calling `job.stop()` is sufficient to unwind a running scan and let
  `BatchProbe`'s deferred `Kill()`/`Wait()` run.

Convention: errors use `errors.Is`; the file already imports `errors` and
`net/http`. Match existing comment density.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Tests | `go test ./...` | ok (all pass) |

(UI must be built once first if `ui/dist/` is absent: `cd frontend && npm run build`.)

## Scope

**In scope**:
- `main.go` — `waitForShutdown` (cancel jobs before RemoveAll; fix comment)
- `server.go` — the `Serve` error filter (add `http.ErrServerClosed`); optionally
  the SSE teardown wiring (Step 3)

**Out of scope**:
- The scan engine, worker pools, `BatchProbe`, or port math — untouched.
- `xray.go` — its deferred Kill/Wait is correct; the fix is to *let it run* by
  cancelling jobs, not to change it.
- Do not change the 5s shutdown grace value's intent; you may shorten the SSE wait
  but keep a bounded grace for in-flight HTTP.

## Git workflow

- Branch: `advisor/002-harden-shutdown-path`
- Commit style: conventional commits, e.g. `fix: cancel in-flight jobs and tear down SSE on shutdown`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add a helper to stop all running jobs

In `server.go` (near the job maps), add:
```go
// stopAllJobs cancels every in-flight scan/clean job so their goroutines unwind
// and each xray BatchProbe's deferred Kill()/Wait() can run before the process
// exits. Idempotent: job.stop() uses sync.Once.
func stopAllJobs() {
    scanJobsMu.Lock()
    for _, j := range scanJobs {
        j.stop()
    }
    scanJobsMu.Unlock()

    cleanJobsMu.Lock()
    for _, j := range cleanJobs {
        j.stop()
    }
    cleanJobsMu.Unlock()
}
```

**Verify**: `go vet ./...` → exit 0.

### Step 2: Call it in `waitForShutdown`, with a short grace before RemoveAll

In `main.go`, change `waitForShutdown` so that after `srv.Shutdown` (or before it —
see Step 3) it calls `stopAllJobs()` and waits a brief grace (e.g. 500ms) for
xray children to die before `os.RemoveAll`. Target shape:
```go
    // Cancel in-flight scans so their xray children get killed (os.Exit skips
    // defers, so we must trigger the cancellation-driven cleanup explicitly).
    stopAllJobs()
    time.Sleep(500 * time.Millisecond) // brief grace for BatchProbe Kill()/Wait()

    os.RemoveAll(filepath.Join(os.TempDir(), "_xray_work"))
    os.RemoveAll(filepath.Join(os.TempDir(), "_xray_clean"))
    os.Exit(0)
```
`stopAllJobs` lives in package `main`, so it is directly callable.

**Verify**: `go build -ldflags="-s -w" -o /dev/null .` → exit 0.

### Step 3: Make SSE streams tear down promptly (fix the 5s hang + the comment)

Pick the lower-risk option:

**Option A (preferred, smallest):** Cancel jobs *before* `srv.Shutdown`. When
`stopAllJobs()` runs first, every job's snapshot flips to `done`/`cancelled`
within one 250ms `streamSSE` tick, so each SSE handler returns and its connection
goes idle — then `srv.Shutdown` completes fast instead of waiting 5s. Reorder so
`stopAllJobs()` precedes `srv.Shutdown(ctx)`, then keep the grace + RemoveAll after.

**Option B (if A proves insufficient):** give `startServer` a package-level
`context.Context` (cancelled from `waitForShutdown` via a stored cancel func) that
`streamSSE` also selects on, so streams end on shutdown regardless of job state.
Only do this if Option A still hangs.

Either way, **fix the false comment** in `main.go` (the "SSE streams are torn down
immediately" line) to describe what actually happens.

**Verify**: `go vet ./...` → exit 0. Manual check in Step 5.

### Step 4: Fix the Serve error filter

In `server.go:354-358`, extend the guard to also ignore `http.ErrServerClosed`:
```go
if err := srv.Serve(listener); err != nil &&
    !errors.Is(err, net.ErrClosed) &&
    !errors.Is(err, http.ErrServerClosed) {
    fmt.Fprintf(os.Stderr, "server error: %v\n", err)
}
```

**Verify**: `go vet ./...` → exit 0.

### Step 5: Manual shutdown smoke test

Build and run the binary (requires `xray` next to it and a built `ui/dist/`).
Start a scan from the UI, then send Ctrl+C. Confirm:
- The process exits in well under 5 seconds.
- No `server error: http: Server closed` line is printed.
- After exit, `ls "$(go env GOTMPDIR 2>/dev/null || echo ${TMPDIR:-/tmp})"/_xray_work "$TMPDIR/_xray_clean" 2>/dev/null` shows the dirs gone (on Windows check `%TEMP%`).
- No lingering `xray` process (`pgrep xray` / Task Manager).

If you cannot run the binary in this environment (no xray, headless), skip the live
run and note it in your report; Steps 1-4 verification (vet/build/test) still gate.

**Verify**: `go test ./...` → ok.

## Test plan

The shutdown path is process-exit code and not cheaply unit-testable (it calls
`os.Exit`). Do NOT try to unit-test `os.Exit`. If you want a regression guard for
the *reusable* piece, add a small test that `stopAllJobs()` is safe to call with
no jobs and safe to call twice (idempotent) — put it in a new `server_shutdown_test.go`
modeled on the table style in `security_test.go`. This is optional; the vet/build/test
gates plus the Step 5 smoke test are the primary verification.

## Done criteria

- [ ] `go vet ./...` exits 0
- [ ] `go build -ldflags="-s -w" -o /dev/null .` exits 0
- [ ] `go test ./...` passes
- [ ] `stopAllJobs()` is called in `waitForShutdown` before `os.RemoveAll`
- [ ] Serve error filter ignores both `net.ErrClosed` and `http.ErrServerClosed`
- [ ] The false "SSE torn down immediately" comment is corrected
- [ ] (If runnable) Step 5 smoke test passes: fast exit, no spurious log, temp dirs gone, no orphan xray
- [ ] Only in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- `waitForShutdown` or the Serve error filter differ materially from the excerpts (drift).
- `job.stop()` is no longer idempotent / `cancelOnce` is gone (design changed) — closing an already-closed channel would panic; STOP.
- Option A still hangs ~5s after reordering AND Option B would require touching more than `startServer` + `streamSSE` — report before expanding scope.

## Maintenance notes

- Any new long-lived streaming endpoint must also observe the shutdown signal, or
  it will reintroduce the idle-wait hang.
- If scan jobs are ever moved off `context.Background()` onto request contexts,
  revisit whether `stopAllJobs()` is still needed (it would be for the batch-Kill path regardless).
- Reviewer: confirm `stopAllJobs` takes the mutexes and that `job.stop()` remains `sync.Once`-guarded.
