# Plan 004: Align apply docs and i18n with free output paths + Replacer tab

> **Executor instructions**: Docs/i18n only. Depends on plan 001 semantics
> (absolute output dirs allowed). Reviewer maintains index.
>
> **Drift check**: `git diff --stat 5945765..HEAD -- docs/endpoint-scanner.md docs/fa/endpoint-scanner.md frontend/src/locales/en.json frontend/src/locales/fa.json docs/ip-replacer.md`

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: plans/001-fix-apply-endpoint-output-paths.md
- **Category**: docs
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

1. Apply UI lives under **IP Replacer → WireGuard mode**, not on the Endpoint
   Scanner tab. `docs/endpoint-scanner.md` still walks apply as step 7 on that tab.
2. Copy still implies output is constrained; after 001, Browse paths work anywhere.
3. EN/FA locale keys must stay in sync (same keys).

## Current state

- `docs/endpoint-scanner.md:69-73` — apply steps on endpoint tab
- `docs/fa/endpoint-scanner.md` — FA counterpart
- `frontend/src/locales/en.json`:
  - `apply.outputLabel`: "Output folder path (leave empty to save next to .exe)"
  - `apply.outputPlaceholder`: "e.g. E:\\vpn\\WG Configs\\modified"
  - `apply.outputTitle`, `apply.browseTitle`
- Apply UI: `frontend/src/components/Replacer.svelte` WireGuard mode
- Endpoint results can push endpoint via `pendingWarpEndpoint` store into Replacer

## Scope

**In scope**:

- `docs/endpoint-scanner.md`
- `docs/fa/endpoint-scanner.md`
- `docs/ip-replacer.md` (add/clarify WireGuard apply subsection if missing)
- `docs/fa/ip-replacer.md` if it exists
- `frontend/src/locales/en.json`
- `frontend/src/locales/fa.json`

**Out of scope**:

- Go code, Svelte logic, README install sections

## Git workflow

- Branch: `advisor/004-align-apply-docs-i18n`
- Commit: `docs: point apply flow at Replacer and free output paths`

## Steps

### Step 1: Rewrite endpoint-scanner apply section

Replace step 7 with something like:

> ### Step 7 — Apply a working endpoint
>
> 1. Click a result to copy the endpoint (or use **Use** if present) — this can
>    hand off to the **IP Replacer** tab.
> 2. Open **IP Replacer**, switch to **WireGuard / WARP** mode.
> 3. Confirm the endpoint field, choose one or more `.conf` files.
> 4. Optionally set an output folder (empty = next to the app) or **Browse**.
> 5. Click **Generate Configs**.

Keep tips that still apply; remove claims that apply UI is on this tab.

Update FA file to match meaning (not machine-garbled — if unsure, mirror structure
and keep technical tokens in English).

### Step 2: ip-replacer docs

Ensure WireGuard apply + Browse absolute path behavior is documented in EN (and FA if present).

### Step 3: i18n

Keep keys stable; only change string values if needed for accuracy, e.g.:

- `apply.outputTitle`: clarify empty = next to app; Browse picks any folder
- Placeholder can stay as an absolute path example (now correct)

**Verify FA has every EN key**:  
`node -e "const e=require('./frontend/src/locales/en.json');const f=require('./frontend/src/locales/fa.json');const m=Object.keys(e).filter(k=>!(k in f));console.log(m.length?m:'ok')"`  
→ `ok`

## Done criteria

- [ ] Endpoint-scanner docs no longer claim in-tab apply form as primary
- [ ] Replacer docs mention WireGuard apply + free output paths
- [ ] EN/FA keys still match
- [ ] No code changes outside scope

## STOP conditions

- Plan 001 not merged / absolute paths still rejected — stop and report.
- FA file structure differs heavily — update what exists; don't invent new doc trees.

## Maintenance notes

- When UI moves again, update both EN/FA docs in the same PR.
