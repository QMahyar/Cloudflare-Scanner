# Plan 002: Cover pure offline helpers with unit tests

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving on. If any
> STOP condition occurs, stop and report — do not improvise. Reviewer maintains
> `plans/README.md` unless told otherwise.
>
> **Drift check**: `git diff --stat 5945765..HEAD -- scanner.go cleanip_validate.go cleanip_measure.go about.go metrics_test.go`

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

Overall statement coverage is ~33%. Several **pure, offline** helpers that shape
user-visible failure text and the update-check badge have **0%** coverage:

- `summarizeFailure` (`cleanip_validate.go`)
- `summarizeWarpFailure` (`scanner.go`)
- `isNewerVersion` / `parseVer` (`about.go`)
- `ipOnly` / `applyColo` / `applyQuality` (`cleanip_measure.go`)

These are easy table-driven tests with no network/xray. Plan 007 will move
proxy code; having characterization tests first reduces refactor risk on the
helpers that stay put and establishes the pattern.

## Current state

Coverage evidence (`go test -cover`): those functions report 0.0%.

Exemplars:

- `metrics_test.go` — small table tests for `lossPercent` / `qualityScore`
- `cleanip_test.go` — `TestExtractXrayErrorScopesByIP` for related validate helpers

`summarizeFailure` categories (read live file for exact strings): startup timeout,
start xray, incomplete config, socks connect, socks5, forcibly closed/reset,
connection refused, http write/read, `http` prefix, cancelled, default passthrough.

`summarizeWarpFailure`: no handshake response, startup timeout, start xray,
socks, i/o timeout, reset/refused, invalid private key, cancelled, tcp dial,
empty → "unknown", default passthrough.

`parseVer`: strips `v`/`V`, truncates at `-+`, splits dots, nil for `dev`/empty/non-numeric.
`isNewerVersion`: nil current + non-empty latest → true; component-wise compare.

`ipOnly`: `net.SplitHostPort`; returns `""` on error.
`applyColo` / `applyQuality`: mutate slices by IP key from maps; no-op on empty maps.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./... -count=1` | all pass |
| Focused | `go test -run 'TestSummarize|TestParseVer|TestIsNewer|TestIPOnly|TestApplyColo|TestApplyQuality' ./... -count=1` | pass |
| Format | `gofmt -l *_test.go` | empty for new/edited |

## Scope

**In scope** (create and/or extend test files only):

- `scanner_test.go` (new) — `TestSummarizeWarpFailure`
- `about_test.go` (new) — `TestParseVer`, `TestIsNewerVersion`
- `cleanip_test.go` (extend) — `TestSummarizeFailure`, `TestIPOnly`, `TestApplyColo`, `TestApplyQuality`

**Out of scope**:

- Production code changes (unless a test reveals a clear 1-line bug — then STOP and report)
- Network tests for `BatchProbe`, `runCleanScan`, handlers
- Frontend tests

## Git workflow

- Branch: `advisor/002-cover-pure-offline-helpers`
- Commit: `test: cover summarize/version/colo pure helpers`

## Steps

### Step 1: summarizeWarpFailure + summarizeFailure tables

Add ~8–12 cases each covering every switch branch listed above. Assert exact
returned strings (copy from source).

**Verify**: focused test run passes.

### Step 2: parseVer + isNewerVersion

Cases: `v3.7.0`, `3.7.0`, `3.7.0-rc1` → `[3,7,0]`; `dev`/`` → nil;
`isNewerVersion("v3.6.0","v3.7.0")` true; equal false; `"dev"` vs `"v1.0.0"` true;
garbage latest false when current parses.

**Verify**: focused tests pass.

### Step 3: ipOnly + applyColo + applyQuality

- `ipOnly("1.2.3.4:443")` → `"1.2.3.4"`; IPv6 bracketed; bare `"nope"` → `""`
- `applyColo` with map `{"1.2.3.4": {"FRA","DE"}}` fills Colo/Loc on matching
  endpoint rows; leaves others empty; empty map no panic
- `applyQuality` sets Loss/Score (and Best/Jitter when zero) from sample

**Verify**: `go test ./... -count=1` all pass; `go vet ./...` exit 0.

## Done criteria

- [ ] New tests exist for all functions named above
- [ ] `go test ./... -count=1` passes
- [ ] No production files modified
- [ ] gofmt clean on touched test files

## STOP conditions

- Function signatures moved/renamed since plan was written.
- A test failure indicates production bug → report; do not "fix" behavior
  unless the bug is an obvious typo and you document it in NOTES.

## Maintenance notes

- Keep expected summarize strings in sync when copy is rewritten for i18n.
- Plan 007 should re-run this suite after any proxy moves (helpers here stay).
