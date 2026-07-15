# Plan 014: Refresh agent and user docs after code splits

> **Executor instructions**: Docs-only. Step by step. No Go/Svelte logic changes.
> **Drift check**: `git diff --stat 380c55e..HEAD -- CLAUDE.md AGENTS.md BUILD.md BUILD.fa.md docs docs/fa README.md README.fa.md`

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: docs
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Agent guides still claim module `WarpEndpointScanner`, a monolithic
`server.go`, and apply-endpoint path confinement inside the install dir.
User docs still say **25 IPv4 / 91 IPv6** CF CIDRs while code has **15 / 7**
official compact ranges. Wrong docs cause wrong agent edits and user
confusion.

## Current state (truth to write)

| Topic | Truth in code |
|-------|----------------|
| Module | `github.com/QMahyar/Cloudflare-Scanner` in `go.mod` |
| HTTP server | `httpserver.go` + `scan_handlers.go` + `cleanscan_handlers.go` + `replacer_handlers.go` + `pickers.go` — **no** `server.go` |
| Apply output | `resolveApplyOutputDir` allows absolute dirs outside install dir; relative joins exe dir (`scan_handlers.go`) |
| CF CIDRs | `cfIPv4CIDRs` length 15, `cfIPv6CIDRs` length 7 in `cleanip_gen.go` |
| Deps | `golang.org/x/crypto` + `github.com/quic-go/quic-go` (not "stdlib only") |
| Mux | `http.ServeMux` with `r.PathValue("id")` Go 1.22+ patterns |
| Embed | `//go:embed all:ui/dist` in `httpserver.go` |

Stale locations (fix all that still match):

- `CLAUDE.md` module + server.go handler refs + apply confinement
- `AGENTS.md` file map `server.go`, module name, deps line
- `BUILD.md` tree `server.go`, "25 IPv4 + 91 IPv6"
- `docs/faq.md` / `docs/fa/faq.md` CIDR counts
- `docs/ip-scanner.md` / `docs/fa/ip-scanner.md` CIDR counts
- Any Getting Started version banner still saying ancient `v3.0.0` if present → use `VERSION` file (`3.7.0` → display as v3.7.0) or drop hard-coded version

**Do not** invent new architecture. Mirror the live file map.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Module truth | `type go.mod` / `cat go.mod` | module github.com/QMahyar/Cloudflare-Scanner |
| No server.go | `dir server.go` / `test ! -f server.go` | missing |
| CIDR count | count lines in `cfIPv4CIDRs` / `cfIPv6CIDRs` | 15 / 7 |
| Grep stale | `grep -R "WarpEndpointScanner\|server.go\|25 IPv4\|91 IPv6" --include="*.md" .` | only historical CHANGELOG/IMPROVEMENTS if any; **zero** in CLAUDE/AGENTS/BUILD/docs |

## Scope

**In scope**: markdown listed above (EN + FA pairs must stay aligned).

**Out of scope**: code, i18n JSON (unless a doc-only string), CHANGELOG historical entries (leave history), `IMPROVEMENTS.md` historical body (optional one-line note only if it still claims live TODOs).

## Git workflow

- Branch: `advisor/014-refresh-agent-and-user-docs`
- Commit: `docs: fix module path, file map, CIDR counts, apply paths`

## Steps

### Step 1: Agent docs (CLAUDE.md, AGENTS.md)

- Replace module claim with `github.com/QMahyar/Cloudflare-Scanner`.
- Replace `server.go` rows with the split files (httpserver, scan_handlers,
  cleanscan_handlers, replacer_handlers, pickers, about).
- Fix apply-endpoint: absolute output dirs allowed; filename basenamed; endpoint validated.
- Fix deps: crypto + quic-go.
- Fix path ID extraction: `r.PathValue`, not only `Path[len(prefix):]` if that text remains.

**Verify**: grep those files for `WarpEndpointScanner` and bare `server.go` as current architecture → no matches (except if saying "formerly").

### Step 2: BUILD.md (+ BUILD.fa.md if it mirrors)

- Tree listing: remove or replace `server.go`; mention split handlers.
- Replace 25/91 CIDR wording with 15 IPv4 + 7 IPv6 official compact lists (link cloudflare.com/ips).
- Fix any "stdlib only" claim.

**Verify**: BUILD.md no longer lists non-existent server.go as live source.

### Step 3: User docs EN+FA

- faq + ip-scanner (and fa/): 15 / 7 counts; weighted IPv4 sampling language can stay.
- getting-started: apply lives on Replacer WARP tab; free output path; shutdown cleans xray temp dirs — align with README if stale.
- Keep bilingual parity: same factual corrections on both languages.

**Verify**: `grep -R "25 IPv4\|91 IPv6\|25 CIDR\|91 CIDR" docs README.md README.fa.md` → no matches (or only "historically").

### Step 4: Final grep

**Verify**:

```
grep -R "WarpEndpointScanner" --include="*.md" .
grep -R "25 IPv4" --include="*.md" docs BUILD.md
```

Only acceptable hits: CHANGELOG/IMPROVEMENTS historical, or explicit "old name" notes.

## Done criteria

- [ ] Agent docs match go.mod + live file layout
- [ ] User docs CIDR counts match `cleanip_gen.go`
- [ ] Apply path docs match free absolute dirs
- [ ] EN/FA pairs updated together
- [ ] No code changes

## STOP conditions

- Unsure whether a doc is historical — leave CHANGELOG alone and only fix live guides.
- FA translation capacity: if blocked, apply English truth and mark FA with same numbers using simple parallel edits (numbers transfer).

## Maintenance notes

- After future file splits, update AGENTS.md file map in the same PR.
- CIDR list changes must update docs in the same change.
