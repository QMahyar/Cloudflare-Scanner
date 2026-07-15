# Plan 007: Split proxy.go along parse / share-URL / xray-build seams

> **Executor instructions**: Mechanical file split only — no behavior change.
> Depends on plan 002 tests already green. Reviewer maintains index.
>
> **Drift check**: `git diff --stat 5945765..HEAD -- proxy.go parsers_test.go`

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: MED
- **Depends on**: plans/002-cover-pure-offline-helpers.md
- **Category**: tech-debt
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

`proxy.go` is ~800 lines after `server.go` / `cleanip.go` were split in v3.7.0.
It mixes: type + parse, share-URL generation, and xray JSON outbound/stream/batch
builders. Reviews and navigation suffer; risk is high only if the split rewrites
logic — so **move code, do not edit logic**.

## Current state (approx line map at 5945765)

| Region | Content |
|--------|---------|
| 1–101 | `ProxyConfig`, helpers, `looseString`, start of parse |
| 102–339 | `ParseVMessURL`, `ParseProxyURL` |
| 340–494 | `WithEndpoint`, `GenerateShareURL`, `generateVMessShareURL` |
| 495–586 | xray JSON DTO types |
| 587–771 | `buildOutboundSettings`, `buildStreamSettings` |
| 772–end | `BuildXrayJSONBatch` |

Still `package main` — same as other splits (`cleanip_gen.go`, etc.). No new packages.

## Target layout

| File | Responsibility |
|------|----------------|
| `proxy.go` | `ProxyConfig` struct + small shared helpers (`parseInsecureFlag`, `decodeBase64Loose`, `looseString`) |
| `proxy_parse.go` | `ParseVMessURL`, `ParseProxyURL` |
| `proxy_share.go` | `WithEndpoint`, `GenerateShareURL`, `generateVMessShareURL` |
| `proxy_xray.go` | stream/outbound DTOs + `buildOutboundSettings`, `buildStreamSettings`, `BuildXrayJSONBatch` |

If a symbol is needed across files, keep it package-level (unexported is fine).

**Do not** rename exported methods. **Do not** change JSON field tags.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./... -count=1` | all pass (including parsers share-URL round-trips) |
| Vet | `go vet ./...` | exit 0 |
| Format | `gofmt -w proxy*.go` then `gofmt -l proxy*.go` | empty |
| Build | `go build -o NUL .` (Windows) or `go build -o /dev/null .` | exit 0 |

Note: if `ui/dist` missing, build may fail embed — use `go test` as primary gate;
for build, either ensure `ui/dist` exists or rely on tests+vet only and note it.

## Scope

**In scope**: `proxy.go` and new `proxy_*.go` files only  
**Out of scope**: behavior changes, new features, moving `GenerateExport` from `cleanip.go`, test file renames

## Git workflow

- Branch: `advisor/007-split-proxy-go`
- Commit: `refactor: split proxy.go into parse/share/xray files`

## Steps

### Step 1: Create files by cut-paste

1. Read full `proxy.go`.
2. Create the three new files with `package main` and needed imports only.
3. Leave struct + tiny helpers in `proxy.go`.
4. `gofmt` all.

**Verify**: `go test ./... -count=1` identical pass to before.

### Step 2: Confirm no logic drift

`git diff` should be almost pure moves (plus import blocks). If you reformatted
incidentally, OK; if you "cleaned up" conditionals — revert that.

**Verify**: `go test -run 'TestParse|TestShare|TestBuild|TestGenerate' ./... -count=1` pass

## Done criteria

- [ ] `proxy.go` substantially smaller; four files exist
- [ ] Full test suite passes with no production logic change intended
- [ ] gofmt clean; go vet clean

## STOP conditions

- Tests fail after pure move — investigate import cycles / lost functions; don't
  "fix" by changing parse behavior.
- Plan 002 not present and parsers tests already failing on HEAD — stop.

## Maintenance notes

- AGENTS.md source map should eventually list the new files (optional doc touch
  only if you already have docs open; not required).
- Next protocol feature lands in `proxy_parse.go` / `proxy_share.go` as appropriate.
