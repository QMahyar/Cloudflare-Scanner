# Plan 003: Bound and dedupe the clean-scan port list so it can't drive a huge allocation

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open the files in "Current state" and confirm the
> quoted lines match live. On mismatch, STOP.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: security
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

Every other size knob on the clean-scan request is clamped server-side (`Count`,
`Concurrency`, `Phase1Probes`, `Phase2Probes`, `Phase2Count`, timeouts). The port
list is **not**: `handleCleanScanStart` accepts every port in `1..65535` from the
request with no length cap and no dedup. The generators then allocate
`(v4Count + v6Count) * len(ports)` endpoint strings. With `Count` up to
`maxScanCount` (100000) and a port list of tens of thousands (the request body cap
is 1 MB, room for ~150k integers), that product is a multi-hundred-million to
multi-billion-element slice — the process exhausts memory and crashes before it
dials anything. It's reachable only with a valid CSRF token (local origin), so the
realistic impact is a self-inflicted / malformed-input crash rather than remote
abuse — but it is the one unclamped input in an otherwise carefully-bounded
handler, and the fix is trivial.

## Current state

- `server.go:1222-1231` — port resolution, no cap, no dedup:
  ```go
  // Resolve scan ports — validate and default to 443
  var scanPorts []int
  for _, p := range req.Ports {
      if p >= 1 && p <= 65535 {
          scanPorts = append(scanPorts, p)
      }
  }
  if len(scanPorts) == 0 {
      scanPorts = []int{443}
  }
  ```
- `server.go:1183-1194` — the surrounding block where the other knobs ARE clamped
  (`req.Count` → `maxScanCount`, `clampInt(...)` for probes). Add the port cap here
  in the same style.
- The multiplication that makes this dangerous:
  - `cleanip.go:164` — `endpoints := make([]string, 0, (len(v4IPs)+len(v6IPs))*len(ports))`
  - `iprange.go:196` (custom-ranges path) — analogous `make(..., (v4+v6)*len(ports))`.
- Existing caps live as consts in `server.go:170-178` (`maxScanCount`,
  `maxEndpointConcurrency`, `maxCleanPhase1Probes`, etc.). Add a new const there.
- `clampInt(v, min, max int) int` exists at `server.go:232-240`.

Convention: clamps are simple inline `if`/`clampInt` in the handler; caps are
named `maxXxx` consts grouped at `server.go:170-178`. Match that.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Test (target) | `go test -run TestCleanScanPorts ./...` | ok, new test passes |
| Full tests | `go test ./...` | ok |

## Scope

**In scope**:
- `server.go` — add a `maxScanPorts` const; dedupe + cap `scanPorts` in `handleCleanScanStart`
- `cleanip_test.go` — add a test asserting the bound (this file already tests the generator; see its existing table tests)

**Out of scope**:
- `cleanip.go` / `iprange.go` generators — do NOT change the allocation lines; the
  fix is to bound the input, not the generator (the generator's `cap` hint is correct
  once the input is bounded).
- Any other handler or knob.

## Git workflow

- Branch: `advisor/003-cap-scan-ports`
- Commit style: conventional commits, e.g. `fix: cap and dedupe clean-scan port list`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add a cap constant

In `server.go` with the other `maxXxx` consts (around line 170-178), add:
```go
const maxScanPorts = 64 // dedup + cap the port list; ×IP-count bounds total endpoints
```
64 is generous (the official `CFCDNPorts` list is far smaller) while keeping
`maxScanPorts * maxScanCount` a sane allocation ceiling.

**Verify**: `go vet ./...` → exit 0.

### Step 2: Dedupe and cap the port list

Replace the loop at `server.go:1222-1231` with a version that dedupes and caps:
```go
// Resolve scan ports — validate, dedupe, and cap. The port count multiplies the
// IP count into the endpoint slice, so an unbounded list could blow up allocation.
var scanPorts []int
seenPort := make(map[int]bool)
for _, p := range req.Ports {
    if p < 1 || p > 65535 || seenPort[p] {
        continue
    }
    seenPort[p] = true
    scanPorts = append(scanPorts, p)
    if len(scanPorts) >= maxScanPorts {
        break
    }
}
if len(scanPorts) == 0 {
    scanPorts = []int{443}
}
```

**Verify**: `go build -ldflags="-s -w" -o /dev/null .` → exit 0.

### Step 3: Add a regression test

In `cleanip_test.go`, add a test that the generator's output stays bounded when
given a capped port set, and (if the dedup/cap logic is extracted or callable)
that duplicates collapse. Since the cap lives in the handler, the cleanest unit
test targets the *generator* with a realistic bounded port list and asserts the
endpoint count equals `ipCount * len(uniquePorts)`. Model it on the existing
`GenerateIPs` tests already in `cleanip_test.go`. Name it `TestCleanScanPorts...`.

Example assertion shape:
```go
func TestCleanScanPortsBounded(t *testing.T) {
    g := NewCleanIPGenerator()
    ports := []int{443, 2053, 8443}
    eps := g.GenerateIPs(100, true, false, ports)
    if len(eps) > 100*len(ports) {
        t.Fatalf("endpoint count %d exceeds ipCount*ports %d", len(eps), 100*len(ports))
    }
}
```
If you can factor the handler's dedup/cap into a tiny helper (e.g.
`sanitizePorts([]int) []int`) callable from the test without spinning an HTTP
server, prefer that and test the dedup+cap directly.

**Verify**: `go test -run TestCleanScanPorts ./...` → ok.

## Test plan

- New test in `cleanip_test.go`: (a) endpoint count is bounded by `ipCount * len(ports)`;
  (b) if a `sanitizePorts` helper is extracted: duplicate ports collapse, out-of-range
  ports drop, and the result length never exceeds `maxScanPorts`.
- Pattern: the existing table-driven generator tests in `cleanip_test.go`.
- Verification: `go test ./...` → all pass including the new test.

## Done criteria

- [ ] `maxScanPorts` const exists in `server.go`
- [ ] `handleCleanScanStart` dedupes and caps `scanPorts`
- [ ] New `TestCleanScanPorts*` test exists and passes
- [ ] `go vet ./...` exits 0; `go build` exits 0; `go test ./...` passes
- [ ] Only in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- The port-resolution block at `server.go:1222` differs materially from the excerpt.
- `req.Ports` is not an `[]int` field on the request struct (confirm its type near `server.go:1142`) — adjust the loop accordingly and note it, but if it's a wholly different shape, STOP.

## Maintenance notes

- If `CFCDNPorts` (the official port list) ever exceeds 64, bump `maxScanPorts` to match.
- Reviewer: confirm the default-to-443 fallback still fires when the request sends no valid ports.
- The custom-ranges path (`iprange.go:GenerateFromRanges`) receives the same
  already-capped `scanPorts`, so it inherits the bound — no separate change needed.
