# Plan 005: Pin frontend CI Node version to 20

> **Executor instructions**: One-line workflow fix. Reviewer maintains index.
>
> **Drift check**: `git diff --stat 5945765..HEAD -- .github/workflows/frontend.yml .github/workflows/ci.yml .github/workflows/release.yml`

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

`ci.yml` and `release.yml` build the UI with **Node 20**. `frontend.yml` uses
**Node 22**. A frontend-only PR can pass `frontend.yml` and still fail (or
differ) on the matrix CI that actually embeds the UI into the Go binary.

## Current state

- `.github/workflows/ci.yml` → `node-version: '20'`
- `.github/workflows/release.yml` → `node-version: "20"`
- `.github/workflows/frontend.yml` → `node-version: '22'`

## Scope

**In scope**: `.github/workflows/frontend.yml` only  
**Out of scope**: upgrading all workflows to 22, package.json engines field (optional)

## Steps

### Step 1

Change `frontend.yml` `node-version` from `'22'` to `'20'` (match `ci.yml` quoting style if you want consistency).

**Verify**: `rg -n "node-version" .github/workflows` shows 20 everywhere.

## Done criteria

- [ ] All three workflows use Node 20
- [ ] No other files changed

## STOP conditions

- Repo intentionally moved to Node 22 everywhere since plan written — align all three to one version and note in NOTES.

## Git workflow

- Branch: `advisor/005-pin-frontend-ci-node`
- Commit: `ci: pin frontend workflow to Node 20`
