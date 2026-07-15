# Plan 018: Document xray-core pin policy and prepare a safe upgrade path

> **Executor instructions**: Prefer docs + version constant alignment. A full
> binary bump needs validation — do not silently jump major versions without
> tests. STOP if upgrade breaks config generation.
> **Drift check**: `git diff --stat 380c55e..HEAD -- .github/workflows/release.yml scripts BUILD.md README.md`

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: HIGH (if actually bumping xray)
- **Depends on**: 009 (Trojan/VMess shape should be fixed before validating new xray)
- **Category**: migration
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Release workflow pins `XRAY_VERSION: v1.8.24` (2024-era). Upstream stable is
far newer (v26.x calendar versioning). Staying frozen is OK **if intentional**;
today the pin looks like neglect. Either (a) document freeze + why, or (b)
bump with a validation checklist.

## Current state

- `.github/workflows/release.yml` `env.XRAY_VERSION: v1.8.24`
- `scripts/build.ps1` / build scripts may echo the same
- `BUILD.md` documents the pin
- Local `xray.exe` may be 1.8.24

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Tests | `go test ./...` | pass |
| Optional xray test | `xray version` / `xray run -test -c ...` | OK on sample configs |

## Scope

**In scope**:

- Docs: `BUILD.md`, maybe `README.md` / `docs/faq.md` one line on bundled xray version
- Optionally bump `XRAY_VERSION` **only if** you run the validation steps below and they pass
- `release.yml` / scripts pin strings stay in sync

**Out of scope**: rewriting outbound builders for hypothetical xray API breaks
beyond plan 009; committing multi-megabyte xray binaries to git.

## Git workflow

- Branch: `advisor/018-xray-pin-upgrade-program`
- Commit: `docs: state xray-core pin policy` and/or `chore: bump bundled xray-core to <ver>`

## Steps

### Step 1: Write an explicit pin policy section in BUILD.md

Add a short section:

- Current pin: v1.8.24
- Why pinned: known-good with WireGuard noise + VLESS/Trojan/VMess builders
- How to bump: change `XRAY_VERSION` in release.yml + scripts; run checklist
- Checklist:
  1. `go test ./...`
  2. `GenerateConfigBatch` noise config `xray run -test`
  3. `BuildXrayJSONBatch` for vless + trojan + vmess `xray run -test`
  4. Smoke manual Phase-2 if possible

### Step 2 (optional bump): Choose a target version

If bumping:

- Prefer a specific tagged release whose zip asset names still match
  `matrix.xray_zip` in release.yml.
- Update all pin sites atomically.
- Download and `xray run -test` against configs produced by unit tests
  (write temp files from `BuildXrayJSONBatch` / `GenerateConfigBatch` in a
  small Go test or script).

If asset names or config schema break: **STOP**, revert pin, document failure
in NOTES — do not leave half-bumped workflow.

### Step 3: Sync docs version strings

README "requires xray" lines should not claim wrong version.

**Verify**: grep XRAY_VERSION / v1.8.24 consistency across release.yml and BUILD.md.

## Done criteria

- [ ] BUILD.md states pin policy and bump checklist
- [ ] If version bumped: all pin sites match; `xray run -test` on generated configs OK
- [ ] If not bumped: pin left at v1.8.24 with documented rationale
- [ ] `go test ./...` still passes

## STOP conditions

- New xray rejects generated configs.
- Release matrix zip name changed and unclear.
- Network blocked for download — document freeze only.

## Maintenance notes

- Revisit pin when noise/Amnezia features need newer xray.
- Plan 009 must be DONE before trusting Trojan/VMess tests on new xray.
