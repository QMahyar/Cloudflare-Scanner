# Plan 013: Add gofmt/lint CI gates and normalize .go line endings so formatting is enforced

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open `.gitattributes` and `.github/workflows/ci.yml`
> and confirm the excerpts match. On mismatch, STOP.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: MED (line-ending normalization rewrites every `.go` file's bytes once — must be an isolated, reviewable commit)
- **Depends on**: ideally do AFTER the code-movement plans (010, 011) and any open
  Go edits land, so the one-time EOL renormalization doesn't collide. If those aren't
  scheduled, this is independent.
- **Category**: dx
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

There is no formatting or lint gate in CI (`ci.yml` runs only `go vet` + build +
test). Separately, the working-tree `.go` files use CRLF line endings (only `ui/dist`
is covered by `.gitattributes`), so `gofmt -l .` flags all ~14 Go files as
"unformatted" even though they're otherwise clean — meaning a real `gofmt` gate can't
be added until EOL is normalized, and contributors get noisy false positives. This
plan (a) normalizes `.go` to LF via `.gitattributes` in one isolated commit, then
(b) adds a `gofmt` check (and optionally `go vet` is already there) to CI so
formatting regressions are caught. This unblocks a clean `gofmt -l` and gives the
repo its first automated style gate.

## Current state

- `.gitattributes` (entire file):
  ```
  # The embedded frontend bundle is committed build output — never apply EOL
  # conversion to it, so the bytes Go embeds are identical on every platform.
  ui/dist/** -text
  ```
  (No rule for `*.go`, so Git uses the platform default and the checkout is CRLF.)
- `gofmt -l *.go` currently lists all Go files — confirmed to be a CRLF artifact, not
  real formatting drift (a `gofmt -d` diff shows only line-ending changes).
- `.github/workflows/ci.yml:39-52` — has a `Vet` step and a `Test` step, no format/lint step.

Convention: CI is a matrix (`goos × goarch`) on `ubuntu-latest`; the `Test` step is
gated to `linux/amd64`. Add the format check as a single `linux/amd64`-gated step
(formatting is platform-independent; no need to run it 6×).

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Show EOL of a file | `file main.go` | before: "CRLF"; after Step 2 (re-checkout): no "CRLF" |
| Format check | `gofmt -l .` | after normalization: prints nothing |
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Tests | `go test ./...` | ok |

## Scope

**In scope**:
- `.gitattributes` — add `*.go text eol=lf` (and optionally other text types)
- The one-time renormalization of `.go` files (via `git add --renormalize .`)
- `.github/workflows/ci.yml` — add a gofmt check step

**Out of scope**:
- Reformatting code content — `gofmt` should produce ONLY line-ending changes on the
  normalization commit. If it wants to change actual formatting, that's a separate concern; STOP.
- `ui/dist/**` — keep its existing `-text` rule untouched (its bytes must stay stable).
- Introducing `golangci-lint` — optional stretch (Step 4); the required gate is `gofmt`.

## Git workflow

- Branch: `advisor/013-lint-eol-ci`
- **Two commits, kept separate for reviewability**:
  1. `chore: normalize .go line endings to LF (.gitattributes + renormalize)`
  2. `ci: add gofmt formatting gate`
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add the `.gitattributes` rule

Append to `.gitattributes` (keep the existing `ui/dist/** -text` line):
```
# Go sources are LF everywhere so gofmt -l stays meaningful across platforms.
*.go text eol=lf
```

**Verify**: `git check-attr eol -- main.go` → `main.go: eol: lf`.

### Step 2: Renormalize in an isolated commit

Run:
```
git add --renormalize .
```
Then inspect: `git diff --cached --stat` should show the `.go` files with line-ending-only
changes. Confirm the change is EOL-only for a sample file:
```
git diff --cached --word-diff=none main.go | head
```
Commit this as commit #1 (normalization only). Do NOT mix any other change in.

**Verify**: after commit, `gofmt -l .` prints **nothing** (files are now LF and gofmt-clean).
If `gofmt -l` still lists files, run `gofmt -d <file>` on one — if the diff is real
formatting (not EOL), STOP and report (there's genuine drift to handle separately).

### Step 3: Add the CI gofmt gate

In `.github/workflows/ci.yml`, add a step after `Vet` (gate it to one cell so it runs once):
```yaml
      - name: Format check
        if: matrix.goos == 'linux' && matrix.goarch == 'amd64'
        run: |
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            echo "These files are not gofmt-clean:"; echo "$unformatted"; exit 1
          fi
```

**Verify**: locally, `gofmt -l .` → empty (the same check CI will run).

### Step 4 (optional stretch): golangci-lint

If you want a linter too, add a `golangci-lint` step using the official action with a
minimal `.golangci.yml` (enable `govet`, `staticcheck`, `errcheck`, `ineffassign`).
Run it locally first (`golangci-lint run`) and only commit if it's green or you fix
the findings — a red new linter that blocks CI is worse than none. If it surfaces many
issues, DO NOT fix them all here; note them and leave golangci-lint out of this plan.

**Verify**: if added, `golangci-lint run` → exit 0 (or omit the step).

### Step 5: Full verification

**Verify**: `go vet ./...` → 0; `go build -ldflags="-s -w" -o /dev/null .` → 0;
`go test ./...` → ok; `gofmt -l .` → empty.

## Test plan

No unit tests. The gate itself is the test: `gofmt -l .` empty locally, and CI now
enforces it. Confirm the existing Go suite still passes (normalization must not change behavior).

## Done criteria

- [ ] `.gitattributes` has `*.go text eol=lf`; `ui/dist/** -text` preserved
- [ ] Normalization is its OWN commit, EOL-only (spot-checked)
- [ ] `gofmt -l .` prints nothing
- [ ] `ci.yml` has a gofmt check step (gated to linux/amd64)
- [ ] `go vet` + `go build` + `go test ./...` green
- [ ] `plans/README.md` status row updated

## STOP conditions

- After renormalization, `gofmt -l .` still lists files AND `gofmt -d` shows real
  formatting changes (not EOL) — there's genuine drift; STOP and report rather than
  bundling a big reformat into the EOL commit.
- `git add --renormalize` touches `ui/dist` bytes — the `-text` rule should prevent it;
  if it doesn't, STOP (the embed must stay byte-stable).

## Maintenance notes

- After this lands, contributors on Windows will get LF `.go` checkouts; editors should
  respect `.gitattributes`. Consider adding an `.editorconfig` as a follow-up.
- Reviewer: verify commit #1 is EOL-only (huge line count, zero content change) and
  commit #2 is just the CI step.
- The advisor's other plans note a "CRLF gofmt" caveat; once this lands, that caveat is obsolete.
