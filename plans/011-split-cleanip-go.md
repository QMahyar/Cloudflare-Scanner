# Plan 011: Split cleanip.go (1388 LoC) into generation / enrichment / xray-validation / orchestration

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open `cleanip.go` and confirm `grep -nE '^func ' cleanip.go`
> still matches "Current state". On mismatch, STOP.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW (pure code movement within one package)
- **Depends on**: do AFTER 003 and 006 (they edit `cleanip.go`). Not interleaved.
- **Category**: tech-debt
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

`cleanip.go` is 1388 lines spanning four distinct concerns: IP generation (CF-pool +
custom-range + nearby + IPv6), Phase-1/enrichment measurement (TCP dial, colo trace,
quality, HTTP/3 folded in from `metrics.go`/`http3.go`), Phase-2 xray validation, and
the big `runCleanScan` orchestrator (~420 lines by itself). Splitting along those
seams makes each independently readable and shrinks the merge surface. As with
`server.go`, it's all `package main`, so this is mechanical file movement with no API
change — move, don't modify.

## Current state

`grep -nE '^func ' cleanip.go` — group as:

- **Generation** → `cleanip_gen.go`:
  `NewCleanIPGenerator`, `init` (the CF-CIDR weight precompute — keep it with the
  generator data it initializes), `(*CleanIPGenerator).GenerateIPs`, `pickWeighted`,
  `generateNearbyIPs`, `randomIPv6InCIDR`, plus the generator's package-level data
  (`v4CIDRInfo`, `v6CIDRList`, `cleanSocksPortBase`, the `cidrInfo` type, CF CIDR
  tables). Move the data blocks that `init`/`GenerateIPs` read.
- **Measurement / enrichment** → `cleanip_measure.go`:
  `dialReachable`, `runCleanPhase1TCP`, `probeCloudflareTrace`, `ipOnly`,
  `buildColoMap`, `applyColo`, `measureQuality`, `applyQuality`.
- **Xray validation + error summarizing** → `cleanip_validate.go`:
  `summarizeFailure`, `extractXrayErrorFrom`, `socks204Probe`, `validateBatchWithXray`.
- **Orchestration** → keep in `cleanip.go` (or `cleanip_run.go`):
  `CleanIPJob` struct + `(*CleanIPJob).stop`, `runCleanScan`, `(*ProxyConfig).GenerateExport`.

If plan 006 has landed, the `runPhase2Batches` closure inside `runCleanScan` now calls
`runBatches` — that's fine, it stays inside `runCleanScan`.

Convention: `package main`; imports settle per file. CRLF line endings (see note).

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Inventory | `grep -nE '^func ' cleanip.go` | matches "Current state" before starting |
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Tests | `go test ./...` | ok (incl. `cleanip_test.go`) |

> **CRLF note**: `.go` files here use CRLF; `gofmt -l` will flag them regardless.
> Preserve CRLF in new files; judge by vet+build+test, not `gofmt -l`. (If plan 013
> landed, ignore this.)

## Scope

**In scope** (create, moving code out of `cleanip.go`):
- `cleanip_gen.go`, `cleanip_measure.go`, `cleanip_validate.go`, optionally `cleanip_run.go`
- `cleanip.go` shrinks to the orchestrator (or is emptied if you make `cleanip_run.go`)

**Out of scope**:
- ANY change to function bodies/signatures/data values. Cut-and-paste only.
- `cleanip_test.go` — the tests call the same symbols; they must keep passing untouched.
- `metrics.go` / `http3.go` — leave them; only move functions currently IN `cleanip.go`.
- Other `.go` files.

## Git workflow

- Branch: `advisor/011-split-cleanip-go`
- Commit style: conventional commits, e.g. `refactor: split cleanip.go into gen/measure/validate files`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Baseline

`go test ./...` (save output) + `go build`. Confirm `grep -nE '^func ' cleanip.go`
matches "Current state"; else STOP (drift).

**Verify**: tests ok; inventory matches.

### Step 2: Create `cleanip_gen.go`

Move the generation functions AND the package-level generator data they depend on
(`cidrInfo` type, `v4CIDRInfo`, `v6CIDRList`, CF CIDR tables, `cleanSocksPortBase`,
`init`). Add `package main` + imports (`math/rand`, `fmt`, `net`, `sync/atomic`,
`strings` as needed).

**Verify**: `go build` → exit 0.

### Step 3: Create `cleanip_measure.go`, then `cleanip_validate.go`

Move each group verbatim, build after each. `socks204Probe` uses the SOCKS5 helper
(`socks5Handshake`) — confirm where that's defined and that it remains accessible (it
stays wherever it is; you're not moving it unless it's in `cleanip.go`).

**Verify** (after each): `go build` → exit 0.

### Step 4: Settle `cleanip.go`

Leave `CleanIPJob`, `runCleanScan`, `GenerateExport` (rename file to `cleanip_run.go`
only if you prefer; not required). Ensure each moved var/type is declared once.

**Verify**: `go vet ./...` → exit 0.

### Step 5: Full verification

**Verify**: `go build -ldflags="-s -w" -o /dev/null .` → exit 0; `go test ./...` → ok
(identical to Step 1 baseline, including `cleanip_test.go`).

## Test plan

No new tests. `cleanip_test.go` exercises the generator + scoring; it must pass
identically. Behavior is unchanged by construction; changed test results ⇒ a move
altered behavior ⇒ STOP.

## Done criteria

- [ ] The three/four new files exist; `cleanip.go` holds only the orchestrator (or is `cleanip_run.go`)
- [ ] Each moved symbol declared exactly once
- [ ] `go vet` + `go build` + `go test ./...` green and unchanged from baseline
- [ ] `git diff` is move-only (spot-check a couple of functions)
- [ ] `plans/README.md` status row updated

## STOP conditions

- Inventory doesn't match "Current state" (drift from 003/006 — rebase first).
- A duplicate-declaration build error (a data block moved twice) — keep one copy.
- Temptation to tidy a function while moving — DON'T.

## Maintenance notes

- Keep generator data next to the generator; keep enrichment measurement together so
  the `metrics.go`/`http3.go` seam stays clean.
- Reviewer: verify move-only via `git diff -M`.
- Independent of plan 010; either order.
