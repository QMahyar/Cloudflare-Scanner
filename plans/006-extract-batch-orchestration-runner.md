# Plan 006: Extract the shared batch-orchestration runner (dedupe the fragile concurrency/retry logic)

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open the files in "Current state" and confirm the
> quoted lines match live. On mismatch, STOP.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED (touches cancellation + retry semantics on both scan paths)
- **Depends on**: none, but coordinate with 005 (both touch the xray batch area); do 005 first
- **Category**: tech-debt
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

Two functions orchestrate "run endpoints through pooled xray batches with a
concurrency cap, retry each batch's failures once, honor cancellation, keep partial
results": `runScanNoiseBatched` (`server.go:628`, WARP noise) and `runPhase2Batches`
(a closure inside `runCleanScan`, `cleanip.go:1206`). They share the same structure
line-for-line: split into fixed-size batches; `sem := make(chan struct{}, concurrentBatches)`
+ `sync.WaitGroup` fan-out; per-batch `select { case sem<-; case <-ctx.Done(): return }`;
the identical partial-failure retry block (`retryIdx`/`retryEps`,
`partialFailure := len(retryEps) > 0 && len(retryEps) < len(batch)`, mark `Attempts=2`);
and the same `Attempts==0→1` / `Passes==0` normalization. The CLAUDE.md explicitly
warns this cancellation/retry logic is "the part that bites" — and it lives in two
copies, so a fix to one won't reach the other. Extracting a single generic runner
removes that divergence risk.

## Current state

- `server.go:628-771` — `func runScanNoiseBatched(ctx context.Context, job *ScanJob, scanner *Scanner)`:
  - `const batchSize = 16`; `concurrentBatches := (scanner.Concurrency + batchSize - 1) / batchSize` (min 1).
  - `allocPortBase` closure: `10800 + (int(warpSocksPortBase.Add(1))*batchSize)%8992`.
  - per-batch validate: `scanner.scanBatchNoise(ctx, batch, allocPortBase())`.
  - result type: `[]ScanResult`; success = `r.Success`.
- `cleanip.go:1197-1290` (inside `runCleanScan`) — `runPhase2Batches` closure:
  - `allocPortBase`: `20799 + (int(cleanSocksPortBase.Add(1))*phase2BatchSize)%11968`.
  - per-batch validate: `validateBatchWithXray(ctx, &validationCfg, batch, xrayPath, allocPortBase(), phase2Timeout)`.
  - result type: `[]CleanIPResult`; success = `r.Success`.
  - has an `onBatch func([]CleanIPResult)` callback invoked per completed batch.
- Both watch `job.Cancel` (via a local `atomic.Bool cancelled`) and `ctx.Done()`.
- `warpSocksPortBase` / `cleanSocksPortBase` are `atomic.Int32` (`server.go:194`,
  `cleanip.go:55`). `phase2BatchSize` is a const in `cleanip.go`.

The two result types (`ScanResult`, `CleanIPResult`) differ, so the runner must be
**generic** (`[T any]`) or operate on an interface. Go generics are available (go 1.26).

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Tests | `go test ./...` | ok |

## Scope

**In scope**:
- New file `batchrun.go` — the generic runner
- `server.go` — `runScanNoiseBatched` delegates to it
- `cleanip.go` — `runPhase2Batches` delegates to it
- Optional: a small `batchrun_test.go` with a fake validate func (no xray) exercising batching/retry/cancel

**Out of scope**:
- `scanBatchNoise` / `validateBatchWithXray` — the per-batch validators stay as-is.
- The port-band constants — keep `+10800/%8992` (WARP) and `+20799/%11968` (clean)
  in their respective callers' `allocPortBase` closures; the runner receives an
  `allocPort func() int`, it does NOT own the band math.
- Result mapping and the `onBatch`/progress updates — passed in as callbacks.

## Git workflow

- Branch: `advisor/006-extract-batch-runner`
- Commit style: conventional commits, e.g. `refactor: unify pooled-batch orchestration runner`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Define the generic runner in `batchrun.go`

Parameterize over the result type and inject everything that varies:
```go
package main

import (
    "context"
    "sync"
    "sync/atomic"
)

// runBatches splits endpoints into batches of batchSize, runs up to
// concurrentBatches validate() calls at once, retries an endpoint's failures once
// in a fresh follow-up batch (unless the WHOLE batch failed — that's systemic,
// skip the retry), and calls onBatch with each completed batch's results. Honors
// both ctx.Done() and cancel. Returns true if cancelled mid-run.
//
// allocPort returns a fresh non-overlapping SOCKS base port per batch (caller owns
// the port-band math). isSuccess reports whether a result counts as a pass (drives
// the retry set). normalize is applied to every result before onBatch (e.g. force
// Attempts>=1). Generic so both ScanResult and CleanIPResult paths share it.
func runBatches[T any](
    ctx context.Context,
    cancel <-chan struct{},
    endpoints []string,
    batchSize, concurrentBatches int,
    allocPort func() int,
    validate func(batch []string, basePort int) []T,
    endpointOf func(T) string,
    isSuccess func(T) bool,
    markRetried func(*T),      // set Attempts=2 on a result that went through retry
    onBatch func([]T),
) bool {
    // ... build [][]string batches ...
    // ... sem + WaitGroup fan-out, watching cancel/ctx ...
    // ... per batch: res := validate(batch, allocPort()); compute retry set from isSuccess;
    //     if partial (0 < failures < len(batch)) re-run failures once, merge, markRetried;
    //     onBatch(res) ...
    // return cancelled
}
```
Port the exact retry rule from the current code: retry only when
`0 < len(failures) < len(batch)` (skip when the whole batch failed). Preserve the
"keep partial results on cancel" behavior. Study BOTH current implementations and
make the generic body reproduce their shared semantics exactly — where they differ
only in types, that's what the generics absorb.

**Verify**: `go vet ./...` → exit 0.

### Step 2: Delegate `runScanNoiseBatched`

Rewrite its body to call `runBatches[ScanResult](...)` with:
- `batchSize=16`, `concurrentBatches` computed as today,
- `allocPort` = the existing WARP closure (`+10800/%8992`),
- `validate` = `func(b []string, p int) []ScanResult { return scanner.scanBatchNoise(ctx, b, p) }`,
- `isSuccess` = `func(r ScanResult) bool { return r.Success }`,
- `onBatch` = the existing per-batch progress/append into `job` (move that logic into the callback),
- `markRetried` = set `Attempts=2` as today.

Keep the function’s external behavior (progress updates, stop-after, final status)
identical. If `runScanNoiseBatched` currently handles stop-after inside the loop,
keep that in the `onBatch` callback.

**Verify**: `go build` → exit 0; `go test ./...` → ok.

### Step 3: Delegate `runPhase2Batches`

Rewrite the `cleanip.go` closure to call `runBatches[CleanIPResult](...)` with the
clean-path values (`phase2BatchSize`, `+20799/%11968` allocPort,
`validateBatchWithXray` as `validate`, the existing `onBatch`). Preserve the
`retryEps` behavior and the loss/jitter inheritance done in `onBatch`.

**Verify**: `go build` → exit 0; `go test ./...` → ok.

### Step 4: Add an offline runner test

In `batchrun_test.go`, exercise `runBatches` with a **fake** `validate` (no xray) that
returns a deterministic pass/fail per endpoint, and assert:
- All endpoints appear in `onBatch` output exactly once (batching covers the set).
- A partially-failing batch triggers exactly one retry of the failures (count the
  validate invocations for those endpoints).
- A fully-failing batch triggers NO retry.
- Cancelling before start returns `true` and does minimal work.

Model on the table-test style in `cleanip_test.go`.

**Verify**: `go test -run TestRunBatches ./...` → ok.

## Test plan

- `batchrun_test.go` (new): batching completeness, single-retry-on-partial-failure,
  no-retry-on-total-failure, cancel short-circuit — all with a fake validate, no xray/network.
- Existing `go test ./...` must stay green (the scan paths compile and behave).
- Verification: `go test ./...` → all pass including new runner tests.

## Done criteria

- [ ] `runBatches[T]` exists in `batchrun.go`; both callers delegate to it
- [ ] Port-band math stays in each caller's `allocPort` closure (not in the runner)
- [ ] Retry rule preserved: retry only when `0 < failures < batch`; none on total failure
- [ ] Partial results kept on cancel; progress/stop-after behavior unchanged
- [ ] `batchrun_test.go` covers batching/retry/no-retry/cancel and passes
- [ ] `go vet` + `go build` + `go test ./...` green
- [ ] Only in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- The two orchestrators differ in a semantic way beyond types/constants (e.g. one
  retries on total failure, one uses a different concurrency formula) — reconcile
  and report which behavior the unified runner adopts BEFORE shipping; do not guess.
- Generics on the result type force an awkward interface that obscures the logic —
  if the abstraction is fighting you, STOP and report; a clean two-copy state is
  better than a confusing one-copy state.
- Cancellation behavior changes (Stop no longer keeps partial results) — STOP.

## Maintenance notes

- This is the "part that bites" per CLAUDE.md — reviewer should trace cancel/ctx and
  the retry set carefully, and compare against the pre-refactor behavior on both paths.
- Do plan 005 (config builder) around the same time; together they make the xray
  batch machinery a single small surface.
- If a third batch consumer is ever added, it should use `runBatches` too.
