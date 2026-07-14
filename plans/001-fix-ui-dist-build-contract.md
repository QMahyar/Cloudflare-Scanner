# Plan 001: A fresh clone builds, and the docs describe the real build contract

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: This plan was written against a WORKING TREE with
> uncommitted changes on top of commit `6f7a19c`. Do **not** rely on
> `git diff <sha>..HEAD` alone. Instead, open each file named in "Current state"
> and confirm the quoted lines still match the live file. On a mismatch, treat
> it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx / docs
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

`server.go` embeds the built frontend with `//go:embed all:ui/dist`. But
`ui/dist/` is listed in `.gitignore` and **nothing under it is tracked in git**
(`git ls-files ui/dist` returns zero files). A fresh clone therefore has no
`ui/dist/`, so a plain `go build .` fails with a cryptic embed error until the
user first runs `cd frontend && npm run build`. Meanwhile `CLAUDE.md` and
`AGENTS.md` both assert the opposite — that `ui/dist/` is committed and "Node is
only needed to rebuild `ui/dist`, never to `go build`." The docs are actively
wrong and will misdirect anyone debugging the build. This plan makes the docs
match reality (npm build is a prerequisite of `go build`) and adds a guard so
the failure is legible instead of cryptic.

Design decision already made by the maintainer (do NOT reverse it): `ui/dist/`
stays git-ignored and CI builds it (`.github/workflows/ci.yml` runs
`npm run build` before every Go step). `IMPROVEMENTS.md` documents this as a
deliberate change. So the fix is **docs + guard**, not "commit the bundle."

## Current state

- `.gitignore:8` — `ui/dist/` (bundle is ignored, not committed).
- `.github/workflows/ci.yml:28-32` — CI runs `cd frontend && npm ci && npm run build` before Go steps, so CI is unaffected.
- `server.go` around line 27 — the embed directive:
  ```go
  //go:embed all:ui/dist
  var distEmbed embed.FS   // (confirm the exact var name in the live file)
  ```
- `CLAUDE.md` — the paragraph stating (quote): "The committed `ui/dist/` bundle
  is what `go build` embeds, so a UI change is a two-step rebuild... Node is only
  needed to rebuild `ui/dist`, never to `go build`."
- `AGENTS.md` — repeats "`ui/dist/` is committed so `go build` needs no Node"
  (appears around lines 25, 74, 89 — grep to find every occurrence).

Convention: docs in this repo are bilingual (English + Persian). `BUILD.md` has
a Persian twin `BUILD.fa.md`; `README.md` has `README.fa.md`. If you edit a
user-facing build instruction in an English doc that has a `.fa.md` twin, make
the equivalent edit in the twin. `CLAUDE.md` and `AGENTS.md` have no Persian twin.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Confirm bundle untracked | `git ls-files ui/dist \| wc -l` | `0` |
| Build UI | `cd frontend && npm ci && npm run build` | exit 0, regenerates `../ui/dist/` |
| Build binary | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Vet | `go vet ./...` | exit 0 |

## Scope

**In scope**:
- `CLAUDE.md` (fix the build-contract paragraph)
- `AGENTS.md` (fix every "ui/dist is committed" claim)
- `README.md` + `README.fa.md` (add the npm-build prerequisite note if the build section doesn't already state it)
- `BUILD.md` + `BUILD.fa.md` (same)
- `server.go` (add a build-time-friendly comment on the embed line; see Step 4 — this is a comment only, no logic change)

**Out of scope** (do NOT touch):
- `.gitignore` — do NOT un-ignore `ui/dist/`; the ignore is intentional.
- `.github/workflows/*.yml` — CI already builds the UI correctly.
- Any Go logic, the embed mechanism itself, or the frontend source.
- `IMPROVEMENTS.md` — handled by a separate plan (014).

## Git workflow

- Branch: `advisor/001-ui-dist-build-contract`
- Commit style: conventional commits (repo uses `fix:`, `docs:`, `chore:` — see `git log --oneline`). Example: `docs: correct ui/dist build contract (npm build precedes go build)`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Confirm the premise is still true

Run `git ls-files ui/dist | wc -l`. If it prints `0`, the bundle is untracked and
this plan applies. If it prints a non-zero number, the bundle has since been
committed — **STOP and report** (the problem may already be resolved a different
way).

**Verify**: output is `0`.

### Step 2: Fix `CLAUDE.md` and `AGENTS.md`

Rewrite every claim that `ui/dist/` is committed / that `go build` needs no Node.
Replace with the true contract, e.g.:

> `ui/dist/` is **git-ignored and not committed**. `go build` embeds it via
> `//go:embed all:ui/dist`, so you MUST run `cd frontend && npm run build` at
> least once before `go build` will succeed on a fresh clone. CI builds the UI
> automatically before every Go step (`.github/workflows/ci.yml`).

Grep first to find all occurrences: `grep -rn "ui/dist" CLAUDE.md AGENTS.md`.
Fix each. Keep the surrounding prose style intact.

**Verify**: `grep -rn "committed .*ui/dist\|ui/dist.* committed\|never to .go build" CLAUDE.md AGENTS.md` → no matches.

### Step 3: Ensure README/BUILD state the prerequisite

Check `README.md` and `BUILD.md` build sections. If they don't already tell a
from-source builder to run `npm run build` before `go build`, add one clear line.
Mirror the addition into `README.fa.md` / `BUILD.fa.md` (Persian). If they already
say it, leave them and note so in your report.

**Verify**: `grep -rn "npm run build" README.md BUILD.md` → at least one match each.

### Step 4: Add a one-line pointer comment at the embed site

In `server.go`, immediately above the `//go:embed all:ui/dist` line, add a plain
Go comment (NOT a directive) so the next reader knows the prerequisite:
```go
// ui/dist is git-ignored; run `cd frontend && npm run build` before `go build`.
//go:embed all:ui/dist
```
Do not change the directive or the variable. A blank line between a comment and
the `//go:embed` directive would break the directive — keep them adjacent as shown.

**Verify**: `go vet ./...` → exit 0.

### Step 5: Prove the build still works end to end

```
cd frontend && npm ci && npm run build && cd ..
go build -ldflags="-s -w" -o /dev/null .
```

**Verify**: both exit 0.

## Test plan

No unit tests (docs + comment only). The build commands in Step 5 are the
verification. Do not add tests.

## Done criteria

- [ ] `git ls-files ui/dist | wc -l` still `0` (bundle not accidentally committed)
- [ ] `grep -rn "ui/dist" CLAUDE.md AGENTS.md` shows only corrected statements (no "committed"/"no Node for go build")
- [ ] `grep -rn "npm run build" README.md BUILD.md` matches in both
- [ ] `go vet ./...` exits 0
- [ ] `go build -ldflags="-s -w" -o /dev/null .` exits 0 after an npm build
- [ ] Only in-scope files modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

- `git ls-files ui/dist` returns non-zero (bundle now tracked — premise changed).
- The `//go:embed all:ui/dist` line is absent or differs materially from the excerpt.
- `go build` fails even after a successful `npm run build` (a deeper build problem exists — report it, don't paper over).

## Maintenance notes

- If a future maintainer decides to commit `ui/dist/` after all, this plan's docs
  must be reverted in lockstep — the docs and `.gitignore` must always agree.
- Reviewer: verify no logic changed in `server.go` (comment-only diff there).
- The Persian doc twins are easy to forget — reviewer should confirm both sides changed.
