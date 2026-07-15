# Plan 008: Direction spike notes (docs only)

> **Executor instructions**: Write a short design note under `docs/`. No product
> code. Reviewer maintains index.
>
> **Drift check**: `git diff --stat 5945765..HEAD -- docs/ IMPROVEMENTS.md`

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: direction
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

The audit surfaced grounded product options (CLI/headless, Clash/Sing-box export,
auto-tune concurrency, scan resume). Capturing them as a single design note
prevents re-litigating the same ideas and gives a future implementer evidence +
trade-offs without committing to build them now.

## Scope

**In scope**: create `docs/roadmap-notes.md` (and optional one-line link from
`IMPROVEMENTS.md` "Follow-up status" if that file still exists)

**Out of scope**: any Go/Svelte implementation

## Content requirements

Write ~1–2 pages covering **four** options. For each: evidence from the repo,
who benefits, coarse effort (S/M/L), main risk, recommendation (do / defer / skip).

1. **CLI / headless** — FAQ already documents SSH tunnel to random loopback
   (`docs/faq.md`). Evidence: `startServer` binds `127.0.0.1:0`; no flags in
   `main.go`. Recommendation lean: small `--port` / `--no-browser` first.
2. **Clash / Sing-box export** — README names those clients; export is share-URL
   only (`GenerateShareURL`, clean export). Effort M; risk of format drift.
3. **Auto-tune concurrency** — `IMPROVEMENTS.md` lists as not implemented; UI
   already exposes workers. Defer unless support burden is high.
4. **Persistent/resumable scans** — localStorage history is summary-only
   (`stores.js`); full resume is L effort for a one-shot desktop tool. Skip unless
   requested.

Tone: advisor, not roadmap commitment. Title the doc clearly as non-binding notes.

## Steps

### Step 1: Write `docs/roadmap-notes.md`

### Step 2 (optional): one paragraph in `IMPROVEMENTS.md` pointing at it

**Verify**: file exists; no Go files changed (`git status`).

## Done criteria

- [ ] `docs/roadmap-notes.md` exists with all four topics
- [ ] No application code changes

## STOP conditions

- User later says not to add docs — skip and mark REJECTED in index.

## Git workflow

- Branch: `advisor/008-direction-spike-notes`
- Commit: `docs: add non-binding roadmap notes from improve audit`
