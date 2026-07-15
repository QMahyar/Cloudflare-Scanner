# Plan 019: Add minimal frontend pure-lib unit tests

> **Executor instructions**: Add vitest (or light node:test) for `frontend/src/lib` only.
> **Drift check**: `git diff --stat 380c55e..HEAD -- frontend/package.json frontend/package-lock.json frontend/src/lib frontend/vite.config.js`

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests / dx
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Go side has a solid test suite; `frontend/src/lib` (sort, exporters, scanMetrics,
copymode) has **zero** automated tests and CI only runs `vite build`. Pure
functions are cheap to lock.

## Current state

`frontend/package.json` scripts: `dev`, `build`, `preview` only.
Deps: svelte 5, vite 6, svelte-i18n, tanstack virtual, qrcode.

## Commands

| Purpose | Command | Expected |
|---------|---------|----------|
| Install | `cd frontend && npm ci` | exit 0 |
| Build | `cd frontend && npm run build` | exit 0 |
| Test (after add) | `cd frontend && npm test` | pass |

## Scope

**In scope**:

- `frontend/package.json` / lockfile
- vitest (or node:test) config
- tests under `frontend/src/lib/*.test.js` for pure modules
- optional CI step in `frontend.yml` to run tests

**Out of scope**: Svelte component tests, Playwright E2E, visual regression.

## Git workflow

- Branch: `advisor/019-frontend-pure-lib-tests`
- Commit: `test(frontend): add vitest for pure lib helpers`

## Steps

### Step 1: Add vitest as devDependency

```bash
cd frontend
npm install -D vitest
```

Scripts:

```json
"test": "vitest run",
"test:watch": "vitest"
```

Minimal `vitest.config.js` with `environment: 'node'` (avoid happy-dom unless needed).

**Verify**: `npm test` runs 0 tests exit 0 or reports no tests — then add tests.

### Step 2: Tests for pure modules

Priority targets (import and assert):

1. `sort.js` — `parseLatency`, `sortEntries` order
2. `scanMetrics.js` — `computeSummary` if pure
3. `exporters.js` / `copymode.js` — format helpers
4. `api.js` — `csrfToken` needs document cookie mock — skip if awkward; prefer modules without DOM

Do **not** test SSE EventSource heavily.

**Verify**: `npm test` pass with ≥3 meaningful assertions.

### Step 3: CI

In `.github/workflows/frontend.yml` after install:

```yaml
- name: Test
  working-directory: frontend
  run: npm test
```

**Verify**: workflow YAML valid; local npm test + build pass.

## Done criteria

- [ ] `npm test` exists and passes
- [ ] At least sort or metrics helpers covered
- [ ] `npm run build` still works
- [ ] Optional CI step added

## STOP conditions

- Vitest + Svelte 5 tooling conflict — fall back to `node --test` on plain JS
  without vitest.
- Module uses browser-only APIs with no seam — skip that file.

## Maintenance notes

- Keep tests on pure functions; don't block UI iteration on heavy component tests.
