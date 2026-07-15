# Plan 013: Cover remaining pure offline helpers with unit tests

> **Executor instructions**: Step by step; verify; STOP if stuck.
> **Drift check**: `git diff --stat 380c55e..HEAD -- metrics.go metrics_test.go noise.go http3.go replacer.go parsers_test.go security_test.go`

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Plan 002 covered some pure helpers, but coverage still shows **0%** on
latency aggregation (`medianDuration`, `bestDuration`, `jitterDuration`),
`sortCleanIPResults`, `applyH3`, `ParseRawConfigs`, `validateSubscriptionURL`,
and thin `noise.Validate` (~35%). These are free, offline, table-driven wins
that protect refactors of scoring and noise.

Measured: `go test -coverprofile` → those symbols at 0% / Validate 35%.

## Current state

| Symbol | File | Cover |
|--------|------|-------|
| medianDuration / bestDuration / jitterDuration | metrics.go | 0% |
| sortScanResults | metrics.go | ~25% |
| sortCleanIPResults | metrics.go | 0% |
| lossPercent / qualityScore | metrics.go | tested |
| NoiseConfig.Validate | noise.go | ~35% (hex path only-ish) |
| applyH3 | http3.go | 0% |
| ParseRawConfigs | replacer.go | 0% |
| validateSubscriptionURL | replacer.go | 0% |

Pattern: `metrics_test.go`, `security_test.go`, `cleanip_test.go` table tests.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| Cover sample | `go test -coverprofile=cov.out . && go tool cover -func=cov.out \| findstr /i "median best jitter applyH3 Validate validateSubscription ParseRaw sortClean"` | non-zero on listed funcs |

## Scope

**In scope**: new/extended `*_test.go` only (prefer existing files):

- `metrics_test.go`
- new `noise_test.go` or extend an existing test file
- `http3_test.go` or `cleanip_test.go` for `applyH3`
- `parsers_test.go` or `security_test.go` for subscription URL + ParseRawConfigs

**Out of scope**: production code changes (unless a test reveals a real bug —
then STOP and report, do not "fix while testing" unless trivial and in-scope
of a one-line clamp already documented). No frontend. No live network.

## Git workflow

- Branch: `advisor/013-pure-offline-helper-tests`
- Commit: `test: cover metrics/noise/h3/subscription pure helpers`

## Steps

### Step 1: metrics aggregation + sort

In `metrics_test.go` add tables for:

- `medianDuration`: empty→0; odd list; even list average of middle two
- `bestDuration`: empty→0; picks min
- `jitterDuration`: len<2→0; else max-min after sort
- `sortScanResults` / `sortCleanIPResults`: success before fail; latency ascending within success

Use `time.Millisecond` helpers like existing quality tests.

**Verify**: `go test -run 'Median|Best|Jitter|SortScan|SortClean' .` pass.

### Step 2: noise.Validate matrix

Table over types `rand|base64|hex|str|""|unknown`, good/bad packet, delay
ranges, count 0/1/50/51. Expect errors containing known substrings.

**Verify**: `go test -run Noise .` pass.

### Step 3: applyH3

Build a small `[]CleanIPResult` and `map[string]bool` keyed by IP (use
`ipOnly` behavior: keys are host without port). Call `applyH3` and assert
`H3` flags.

**Verify**: `go test -run ApplyH3 .` pass.

### Step 4: validateSubscriptionURL + ParseRawConfigs

- URL: reject empty, ftp, missing host; accept `https://example.com/sub`
- ParseRaw: mixed separators comma/semicolon/pipe; ignore garbage tokens;
  accept one vless and one trojan line

**Verify**: `go test -run 'SubscriptionURL|ParseRaw' .` pass.

### Step 5: Full suite + coverage spot-check

**Verify**: `go test ./...` pass; cover lines for listed funcs > 0.

## Done criteria

- [ ] Listed pure helpers have meaningful tests (not just "doesn't panic")
- [ ] `go test ./...` green
- [ ] No production logic changes (or only STOP-reported)

## STOP conditions

- Helper signatures changed vs plan excerpts.
- Temptation to rewrite production for testability beyond a tiny extract —
  extract only if already planned pattern exists (`clampInt` style).

## Maintenance notes

- Keep tests offline (no DNS, no xray).
- csrfMiddleware httptest is plan 018, not this plan.
