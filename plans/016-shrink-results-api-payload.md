# Plan 016: Shrink scan/clean results API payloads

> **Executor instructions**: Step by step; do not break frontend field names without updating FE.
> **Drift check**: `git diff --stat 380c55e..HEAD -- scan_handlers.go cleanscan_handlers.go frontend/src/components/EndpointScanner.svelte frontend/src/components/IpScanner.svelte`

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: MED (API shape / FE consumers)
- **Depends on**: none
- **Category**: perf
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

`handleScanResults` builds nearly identical `entries` (capped by OutCount) and
`raw` (all successes). The UI often refetches both on every throttled status
tick. Clean Phase-1 returns **all** successes with no server-side display cap.

## Current state

`scan_handlers.go` ~521–585: builds `entries` then `raw` with same fields.
Frontend `EndpointScanner.svelte` uses `raw` for charts/export and `entries`
for table in places — **read both usages before deleting**.

Clean results: no `outCount` server clamp on phase1 success list.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| FE build (if FE touched) | `cd frontend && npm run build` | exit 0 |

## Scope

**In scope**:

- `scan_handlers.go` `handleScanResults`
- `cleanscan_handlers.go` `handleCleanScanResults` (optional phase1 cap)
- FE only if a field is removed/renamed

**Out of scope**: SSE status payload redesign, VirtualTable, new compression.

## Git workflow

- Branch: `advisor/016-shrink-results-api-payload`
- Commit: `perf: dedupe scan results payload; cap clean phase1 display`

## Steps

### Step 1: Map frontend consumers

Search FE for `.raw`, `entries`, `fail_reasons`, `nearby_entries`.

- If `raw` is only a superset of `entries`, prefer returning **one** success
  array (`raw` full successes for export) and make `entries` an alias or
  drop `entries` only if FE updated.
- Safest minimal change: stop filling `entries` separately — set
  `entries` to the first `showN` of the same slice you build for display, and
  build `raw` once then derive `entries = raw[:showN]` without second loop
  re-reading job results. That is a **CPU/alloc** win even if JSON size similar.

Better JSON win: if FE table uses `raw` filtered client-side, set
`"entries": raw` omitted and update FE to use `raw` only — only if you update
all call sites.

### Step 2: Implement minimal safe dedupe

Recommended safe approach (no FE change):

```go
// Build raw (all successes) once.
// entries := raw[:min(showN, len(raw))]  // share underlying array carefully:
entries := raw
if len(entries) > showN {
 entries = raw[:showN]
}
```

Ensure JSON still has both keys for compatibility.

### Step 3 (optional): Cap clean phase1 success list

If clean FE already has client `outCount`, add server max e.g. min(len, maxOutCount)
for phase1 `entries` only when `len > maxOutCount`, **or** document skip.

Only do this if you verify FE does not need the full phase1 list for "export
all responded IPs". If export needs all, keep full list and skip this step.

### Step 4: Tests

If any handler test exists, extend; else add a small httptest test constructing
a job with many successes and asserting `len(entries) <= showN` and raw length.

**Verify**: `go test ./...`; FE still builds if touched.

## Done criteria

- [ ] No double full-scan of results for nearly identical structs without need
- [ ] Both JSON keys remain unless FE updated in-scope
- [ ] Tests green

## STOP conditions

- FE uses divergent fields on entries vs raw (then only share construction, do
  not truncate raw).
- Clean export requires full uncapped phase1 — do not cap.

## Maintenance notes

- Prefer one DTO type for success rows shared by scan/clean later (debt).
