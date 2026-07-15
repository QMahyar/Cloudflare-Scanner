# Plan 010: Cap clean-scan endpoints after port multiplication

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving on.
> STOP conditions: do not improvise. Reviewer may maintain `plans/README.md`.
>
> **Drift check**: `git diff --stat 380c55e..HEAD -- cleanip_gen.go iprange.go cleanscan_handlers.go cleanip_test.go iprange_test.go httpserver.go`

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: perf
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

`maxScanCount` (100000) clamps the **IP count**, but both generators then
cross every IP with every scan port. With `maxScanPorts` (64) that is up to
**6.4 million** `ip:port` strings allocated before Phase 1, defeating the
allocation guard that ports were added to partially fix.

## Current state

- `httpserver.go`: `maxScanCount = 100000`, `maxScanPorts = 64`
- `cleanip_gen.go` `GenerateIPs` — builds IPs then multiplies by ports (lines ~146–158)
- `iprange.go` `GenerateFromRanges` — same pattern (~196–207)
- `cleanscan_handlers.go` clamps `req.Count` then calls generators

```go
// cleanip_gen.go pattern today
endpoints := make([]string, 0, (len(v4IPs)+len(v6IPs))*len(ports))
for _, ip := range v4IPs {
 for _, p := range ports {
  endpoints = append(endpoints, fmt.Sprintf("%s:%d", ip, p))
 }
}
```

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| Vet | `go vet ./...` | clean |
| Format | `gofmt -l cleanip_gen.go iprange.go cleanscan_handlers.go` | empty |

## Scope

**In scope**:

- `cleanip_gen.go` and/or `iprange.go` and/or `cleanscan_handlers.go`
- `cleanip_test.go` / `iprange_test.go` (tests)

**Out of scope**: Phase-1 worker logic, frontend depth presets, WARP endpoint generator.

## Git workflow

- Branch: `advisor/010-cap-endpoints-after-ports`
- Commit: `perf: cap clean-scan endpoints after port multiplication`

## Steps

### Step 1: Add a shared post-multiply cap helper

Prefer one helper in `cleanip_gen.go` (same package main):

```go
// capEndpoints truncates the endpoint list to maxScanCount so port
// multiplication cannot exceed the allocation budget that maxScanCount
// was meant to enforce. maxScanCount lives in httpserver.go (same package).
func capEndpoints(endpoints []string) []string {
 if len(endpoints) > maxScanCount {
  return endpoints[:maxScanCount]
 }
 return endpoints
}
```

Call it at the end of `GenerateIPs` and `GenerateFromRanges` before return.
Alternatively clamp only in `handleCleanScanStart` after generation — same
effect; if you choose the handler, still unit-test via generator or a thin
exported helper.

**Do not** change `maxScanCount` or `maxScanPorts` values unless a test
requires a named constant alias.

**Verify**: `go test -c -o NUL .` builds.

### Step 2: Tests

Add table cases:

1. Generate with `count=10`, `ports=[443,8443,2053]` → `len(endpoints) <= 30` and `<= maxScanCount`.
2. Force overflow path: if feasible, set high count * many ports and assert
   `len(endpoints) <= maxScanCount` (use real generators with e.g. count=1000
   and 13 CFCDNPorts — product is 13000 < cap; to hit the cap without huge
   runtime, either temporarily rely on calling `capEndpoints` directly in a
   unit test with a synthetic slice of length `maxScanCount+5`, **or** pass
   count near max with multiple ports carefully).

Simplest robust test: unit-test `capEndpoints` with slices of length
`maxScanCount-1`, `maxScanCount`, `maxScanCount+10`.

Also assert existing generation tests still pass.

**Verify**: `go test -run 'CapEndpoint|GenerateIPs|GenerateFromRanges' .` pass.

### Step 3: Full suite

**Verify**: `go test ./...` && `go vet ./...` && `gofmt -l` on touched files empty.

## Done criteria

- [ ] Post-port endpoint slices never exceed `maxScanCount`
- [ ] Tests prove the cap
- [ ] Full test/vet pass; scope clean

## STOP conditions

- Generators already cap after multiply (finding fixed).
- Cap would need changing public API JSON fields.
- Verification fails twice.

## Maintenance notes

- If product later wants "100k endpoints across ports" as distinct from "100k
  IPs", rename constants and document; do not silently raise the cap in a
  drive-by.
