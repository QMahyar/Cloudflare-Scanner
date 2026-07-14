# Plan 010: Split server.go (1803 LoC) into cohesive files along natural seams

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open `server.go` and confirm the function list in
> "Current state" still matches (`grep -nE '^func ' server.go`). On mismatch, STOP.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW (pure code movement within one package; no behavior change)
- **Depends on**: do AFTER 002, 003, 005, 006 (they edit `server.go`; moving code first would force painful rebases). Coordinate: run this when those are merged or not yet started, not interleaved.
- **Category**: tech-debt
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

`server.go` is 1803 lines and holds five unrelated concerns: native folder-picker
subprocesses, server bootstrap + CSRF/security middleware, the WARP-scan HTTP
handlers + orchestration, the clean-IP HTTP handlers, and the replacer HTTP
handlers. It's the file everything touches, so every change collides here and review
is harder than it needs to be. Because it's all `package main`, splitting is pure
file movement — no import churn, no API changes — but it makes each concern
independently readable and shrinks merge-conflict surface. This is a mechanical,
low-risk win; the only rule is **move, don't modify**.

## Current state

`grep -nE '^func ' server.go` yields these top-level functions (group them by file):

- **Folder pickers** → `pickers.go`:
  `handleSelectOutputDir`, `selectOutputDir`, `selectOutputDirWindows`,
  `selectOutputDirDarwin`, `selectOutputDirLinux`.
- **Bootstrap + security middleware** → `httpserver.go`:
  `jsonError`, `clampInt`, `newCSRFToken`, `isLoopbackHost`, `csrfMiddleware`,
  `startServer`, `handleIndex`, `streamSSE`. Plus the const block
  (`jobTTL`, `maxScanCount`, … `csrfHeaderName`) at `server.go:168-180` and the
  `scheduleScanJobCleanup`/`scheduleCleanJobCleanup` helpers.
- **WARP-scan handlers** → `scan_handlers.go`:
  `ScanJob` struct + `(*ScanJob).stop`, `scanRequest`, the `scanJobs`/`scanJobsMu`/`jobCounter`
  + `warpSocksPortBase` vars, `handleScanStart`, `runScan`, `runScanNoiseBatched`,
  `handleScanStop`, `handleScanStatus`, `handleScanEvents`, `handleScanResults`,
  `handleApplyEndpoint`.
- **Clean-IP handlers** → `cleanscan_handlers.go`:
  the `cleanJobs`/`cleanJobsMu`/`cleanJobCounter` vars, `handleCleanScanStart`,
  `handleCleanScanStop`, `handleCleanScanStatus`, `handleCleanScanEvents`,
  `handleCleanScanResults`, `handleCleanExport`.
- **Replacer handlers** → `replacer_handlers.go`:
  `proxyConfigToEntry`, `replacerConfigEntry` + `toProxyConfig`, `handleReplacerFetch`,
  `handleReplacerParse`, `handleReplacerApply`.

Note: `distFS`, the `//go:embed` directive, and `startServer`'s route table stay in
`httpserver.go`. Vars must move with (or stay visibly near) their concern but there
must be exactly one declaration of each — watch for the `scanJobs`/`cleanJobs` var
blocks that currently sit mid-file.

Convention: all files are `package main`; no per-file import discipline beyond what
each file uses. `goimports`/`gofmt` will settle imports.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Function inventory | `grep -nE '^func ' server.go` | matches "Current state" before you start |
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Tests | `go test ./...` | ok |
| Format | `gofmt -w <newfiles>` | (writes; then `gofmt -l <newfiles>` prints nothing — see note) |

> **CRLF note**: this repo's `.go` files use CRLF line endings, so `gofmt -l` flags
> them all as "unformatted" even when they're fine. Do NOT mass-reformat. After
> creating each new file, preserve CRLF. Judge formatting by `go vet` + build, not by
> `gofmt -l`. If plan 013 (EOL normalization) has already landed, ignore this note.

## Scope

**In scope** (create these, moving code out of `server.go`):
- `pickers.go`, `httpserver.go`, `scan_handlers.go`, `cleanscan_handlers.go`, `replacer_handlers.go`
- `server.go` — shrinks to (ideally) empty or a tiny remainder; delete it if nothing’s left

**Out of scope**:
- ANY change to function bodies, signatures, var initial values, or the route table
  contents. This is cut-and-paste only.
- Other `.go` files.
- Renaming functions/types.

## Git workflow

- Branch: `advisor/010-split-server-go`
- Commit style: conventional commits, e.g. `refactor: split server.go into pickers/httpserver/handler files`.
- One commit is fine (mechanical). Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Snapshot behavior

Run `go test ./...` and save the output; run `go build` to confirm green baseline.
Record the current `grep -nE '^func ' server.go` — if it doesn't match "Current
state", STOP (drift).

**Verify**: `go test ./...` → ok; function list matches.

### Step 2: Create `pickers.go`

Move the five folder-picker functions verbatim into a new `pickers.go` with
`package main` and the imports they use (`os/exec`, `os`, `path/filepath`, `runtime`,
`strings`, `errors`, plus `net/http` + `encoding/json` for `handleSelectOutputDir`).
Also move `errFolderSelectionCancelled` (`server.go:55`).

**Verify**: `go build -ldflags="-s -w" -o /dev/null .` → exit 0.

### Step 3: Create the remaining files one at a time

For each of `httpserver.go`, `scan_handlers.go`, `cleanscan_handlers.go`,
`replacer_handlers.go`: move the listed functions/types/vars/consts verbatim, add
`package main` + needed imports, and **build after each file** so you localize any
missed dependency. Keep each var/const declared exactly once across the package.

**Verify** (after each): `go build` → exit 0.

### Step 4: Reduce `server.go`

After all moves, `server.go` should be empty of top-level decls (or hold only a
leftover you deliberately kept). If empty, delete it. Ensure the `//go:embed
all:ui/dist` directive and `distFS` live in exactly one file (`httpserver.go`).

**Verify**: `grep -rn '//go:embed' *.go` shows exactly one embed directive.

### Step 5: Full verification

**Verify**: `go vet ./...` → exit 0; `go build -ldflags="-s -w" -o /dev/null .` → exit 0;
`go test ./...` → ok (identical to the Step 1 baseline).

## Test plan

No new tests — behavior is unchanged by construction. The safety net is: the existing
`go test ./...` suite passes identically before and after, plus a clean `go vet`/build.
If any test result changes, a move altered behavior → STOP.

## Done criteria

- [ ] `server.go` is deleted or trivially small; the five new files exist
- [ ] Each moved function/type/var/const appears exactly once in the package
- [ ] Exactly one `//go:embed all:ui/dist` directive remains
- [ ] `go vet` + `go build` + `go test ./...` all green and unchanged from baseline
- [ ] `git diff` shows only moves (no body edits) — spot-check a few functions
- [ ] `plans/README.md` status row updated

## STOP conditions

- The function inventory doesn't match "Current state" (drift — likely because 002/003/005/006 edited `server.go`; rebase this plan onto the new layout first).
- A build error reveals two declarations of the same var (you duplicated a var block) — fix by keeping one; if unclear which concern owns it, put shared vars in `httpserver.go`.
- You feel tempted to "improve" a function while moving it — DON'T. Move only.

## Maintenance notes

- New handlers go in the file matching their concern, not a new god file.
- Reviewer: verify the diff is move-only (e.g. `git diff -M` shows renames/moves, not rewrites).
- This plan and 011 (cleanip split) are independent; either order is fine.
