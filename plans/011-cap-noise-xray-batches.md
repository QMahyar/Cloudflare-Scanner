# Plan 011: Cap concurrent xray batches on WARP noise path

> **Executor instructions**: Step by step; verify each step; STOP if stuck.
> **Drift check**: `git diff --stat 380c55e..HEAD -- scan_handlers.go httpserver.go job_limit_test.go`

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: MED (changes noise scan throughput under high concurrency)
- **Depends on**: none
- **Category**: perf
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Noise scans map user concurrency to concurrent xray **processes**:
`concurrentBatches = ceil(Concurrency / batchSize)` with `batchSize=16` and
`maxEndpointConcurrency=2048` ⇒ up to **128** simultaneous xray processes.
That can thrash disk/CPU/ports. Defaults (concurrency 12) only spawn 1 batch
at a time, so the bug is the **upper bound**, not the default.

## Current state

`scan_handlers.go` `runScanNoiseBatched`:

```go
const batchSize = 16
concurrentBatches := (scanner.Concurrency + batchSize - 1) / batchSize
if concurrentBatches < 1 {
 concurrentBatches = 1
}
```

Clean-IP Phase 2 uses the same formula with `maxCleanPhase2Probes=256` ⇒ max 16
batches — acceptable. This plan is **noise path only** unless you find an
existing shared constant to reuse.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| Vet | `go vet ./...` | clean |

## Scope

**In scope**:

- `scan_handlers.go` (`runScanNoiseBatched`)
- `httpserver.go` (optional new const)
- tests if easy without network (`job_limit_test.go` or new pure helper test)

**Out of scope**: clean Phase-2 batching, native (non-noise) handshake path,
frontend concurrency UI labels (optional one-line comment only if you touch i18n
— prefer not).

## Git workflow

- Branch: `advisor/011-cap-noise-xray-batches`
- Commit: `perf: cap concurrent xray batches for WARP noise scans`

## Steps

### Step 1: Introduce a hard cap

In `httpserver.go` (near other max constants) add:

```go
// maxNoiseConcurrentBatches caps simultaneous xray processes for WARP noise
// scans. concurrentBatches = ceil(concurrency/16); without a cap,
// maxEndpointConcurrency (2048) allows 128 processes.
const maxNoiseConcurrentBatches = 8
```

In `runScanNoiseBatched`, after computing `concurrentBatches`:

```go
if concurrentBatches > maxNoiseConcurrentBatches {
 concurrentBatches = maxNoiseConcurrentBatches
}
```

Keep `batchSize = 16`. Do not change clean-IP constants.

**Verify**: `gofmt` + package builds.

### Step 2: Pure test for the clamp (optional but preferred)

Extract a one-liner pure function if that makes testing trivial:

```go
func noiseConcurrentBatches(concurrency, batchSize, maxBatches int) int {
 if batchSize < 1 {
  batchSize = 16
 }
 n := (concurrency + batchSize - 1) / batchSize
 if n < 1 {
  n = 1
 }
 if maxBatches > 0 && n > maxBatches {
  n = maxBatches
 }
 return n
}
```

Table: concurrency 12 → 1; 16 → 1; 17 → 2; 2048 with max 8 → 8.

**Verify**: `go test -run NoiseConcurrent .` pass.

### Step 3: Full suite

**Verify**: `go test ./...` && `go vet ./...`.

## Done criteria

- [ ] Noise path never schedules more than `maxNoiseConcurrentBatches` concurrent validate calls
- [ ] Default concurrency 12 still yields 1 batch
- [ ] Tests/vet green; scope clean

## STOP conditions

- Cap already present.
- Clean-IP path would need changes to compile (should not).
- Product constant already exists under another name — reuse, do not duplicate.

## Maintenance notes

- If users report noise scans too slow at high concurrency, raise
  `maxNoiseConcurrentBatches` deliberately (document in CHANGELOG).
- Reviewer: confirm clean Phase-2 formula untouched.
