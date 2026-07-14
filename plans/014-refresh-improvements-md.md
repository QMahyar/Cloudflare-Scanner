# Plan 014: Refresh IMPROVEMENTS.md so it reflects what actually shipped

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open `IMPROVEMENTS.md` and confirm the excerpts match. On mismatch, STOP.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW (docs only)
- **Depends on**: none (but if plan 001 already corrected build docs, keep this consistent with it)
- **Category**: docs
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

`IMPROVEMENTS.md` is a point-in-time (2026-06-13) implementation log that has drifted
from reality. Its "Future Enhancements (Not Implemented)" and "Documentation Updates
Needed" sections list items that have since shipped or been overtaken, and its
"Rollout Plan" still describes committing `ui/dist` "one last time" — the opposite of
the now-final state (bundle is git-ignored, CI builds it). A stale status doc is worse
than none: it misleads anyone planning work off it. This plan brings it into line with
the current code, or converts it into a clearly-dated historical record so no one
mistakes it for current status.

## Current state

- `IMPROVEMENTS.md` — dated "2026-06-13", authored as an implementation summary of
  "Three Key Improvements". Problem sections:
  - "## Documentation Updates Needed" lists README/BUILD/AGENTS edits that may already
    be done (verify against the live docs).
  - "## Rollout Plan" step 2: "Commit the current `ui/dist/` one last time before
    adding to .gitignore" — the repo has since fully removed `ui/dist` from git
    (`git ls-files ui/dist` → 0) and CI builds it, so this is historical, not a to-do.
  - "## Future Enhancements (Not Implemented)" lists e.g. "IP Scanner concurrency
    controls" — check whether any have since been implemented (the clean-scan request
    now exposes `Phase1Probes`/`Phase2Probes`, so at least the concurrency-controls item
    is partly done).
- Cross-check source of truth: `CLAUDE.md`, `AGENTS.md`, `.gitignore`, `.github/workflows/ci.yml`,
  and `git log --oneline` since 2026-06-13.

Convention: docs are Markdown; the repo keeps bilingual twins for user-facing docs but
`IMPROVEMENTS.md` has no Persian twin (it's an internal log) — no `.fa.md` needed.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Confirm bundle untracked | `git ls-files ui/dist \| wc -l` | `0` |
| See what shipped since the doc | `git log --oneline --since=2026-06-13` | list of commits to reconcile against |
| Grep claims | `grep -n "Not Implemented\|Rollout\|ui/dist" IMPROVEMENTS.md` | the stale sections |

## Scope

**In scope**:
- `IMPROVEMENTS.md` only

**Out of scope**:
- Code, CI, other docs (plan 001 handles CLAUDE.md/AGENTS.md/README/BUILD).
- Do NOT delete the file's historical value — prefer reframing over erasing.

## Git workflow

- Branch: `advisor/014-refresh-improvements-md`
- Commit style: conventional commits, e.g. `docs: reconcile IMPROVEMENTS.md with shipped state`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Establish ground truth

Run the three commands above. For each claim in IMPROVEMENTS.md, mark it as: SHIPPED,
STILL-PENDING, or OVERTAKEN. Specifically confirm: `ui/dist` is untracked (bundle
finding), and whether the "Not Implemented" items now exist in code (grep the codebase
for the feature — e.g. `Phase1Probes`/`Phase2Probes` in `server.go`).

**Verify**: `git ls-files ui/dist | wc -l` → `0`.

### Step 2: Reframe the doc

Two acceptable approaches — pick one and state which in your report:
- **(A) Historical marker (lightest):** Add a bold banner at the top:
  `> **Historical record (2026-06-13).** Describes a past change set; for current build
  contract see CLAUDE.md/AGENTS.md.` Then correct only the factually-wrong "Rollout"
  step 2 (it reads as a live instruction) and any "Not Implemented" item that has since shipped.
- **(B) Full refresh:** Update each section to current reality, move shipped items out
  of "Not Implemented", and rewrite the Rollout section in past tense.

Either way, the `ui/dist` commit-one-last-time instruction must no longer read as a to-do.

**Verify**: `grep -n "one last time" IMPROVEMENTS.md` → no live-instruction phrasing remains (removed or clearly marked historical).

### Step 3: Cross-check against plan 001

If plan 001 has landed (CLAUDE.md/AGENTS.md corrected), ensure IMPROVEMENTS.md doesn't
contradict it. If 001 hasn't run, at minimum don't reintroduce the "committed ui/dist" claim.

**Verify**: `grep -rn "committed.*ui/dist\|ui/dist.*committed" IMPROVEMENTS.md` → no false claims.

## Test plan

No tests (docs). Verification is the grep checks above plus a human read for internal consistency.

## Done criteria

- [ ] Every IMPROVEMENTS.md claim is either accurate or clearly marked historical
- [ ] The "commit ui/dist one last time" instruction no longer reads as a pending to-do
- [ ] Shipped items are not listed under "Not Implemented"
- [ ] No contradiction with CLAUDE.md/AGENTS.md/.gitignore reality
- [ ] Only `IMPROVEMENTS.md` modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- `git ls-files ui/dist` returns non-zero (the bundle situation differs from this plan's premise) — reconcile and report.
- You're unsure whether a "Not Implemented" item shipped — grep the code; if still
  ambiguous, mark it "status unverified" rather than guessing.

## Maintenance notes

- Consider whether this file should live under `docs/` or be merged into a CHANGELOG
  section — flag for the maintainer; out of scope here.
- Reviewer: a quick diff read for tense/accuracy is enough; no build impact.
