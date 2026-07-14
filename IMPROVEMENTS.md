# Implementation Summary: Three Key Improvements

> **Historical record (2026-06-13).** This documents a past change set and is kept
> for provenance. It is **not** a live to-do list. For the current build contract
> see `CLAUDE.md` / `AGENTS.md` / `BUILD.md` (short version: `ui/dist/` is
> git-ignored and rebuilt with `npm run build` before `go build`; CI does this
> automatically). The `ui/dist/`-related rollout steps below have already shipped.

This document summarizes the three improvements made to the Cloudflare Scanner codebase based on the architectural review.

---

## 1. Move `ui/dist/` to `.gitignore` + CI Build Step ✅

### Problem
The compiled frontend assets (`ui/dist/`) were committed to Git, which:
- Bloats the repository size
- Creates merge conflicts on UI changes
- Goes against best practices for generated artifacts

### Solution
- **Updated `.gitignore`**: Added `ui/dist/` to ignore list (removed the exception rules)
- **Updated CI workflow** (`.github/workflows/ci.yml`): Added Node.js setup and `npm ci && npm run build` before Go build
- **Updated Release workflow** (`.github/workflows/release.yml`): Added Node.js setup and UI build step before building binaries

### Impact
- Repository stays clean of generated artifacts
- CI/CD automatically builds the UI before embedding
- Developers must run `cd frontend && npm run build` before `go build` (documented behavior)
- No functional changes to the application

### Files Changed
- `.gitignore`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`

---

## 2. Fix Module Name Mismatch ✅

### Problem
The `go.mod` declared `module WarpEndpointScanner`, but the GitHub repository is `Cloudflare-Scanner`. This:
- Breaks `go install github.com/QMahyar/Cloudflare-Scanner@latest`
- Confuses Go tooling and dependency resolution
- Creates inconsistency between repo path and module name

### Solution
Changed `go.mod` module declaration to match the repository path:
```go
module github.com/QMahyar/Cloudflare-Scanner
```

### Impact
- `go install` now works correctly
- Go tooling can properly resolve the module
- Aligns with Go module best practices
- **Note**: This is a breaking change for any external code importing this module (though unlikely given it's a `package main` application)

### Files Changed
- `go.mod`

---

## 3. Expose Concurrency Settings in UI "Advanced" Panel ✅

### Problem
Concurrency was hardcoded:
- Endpoint Scanner: 256 workers (native handshake) / 12 (noise mode)
- IP Scanner Phase 1: 500 workers
- IP Scanner Phase 2: 12 batches

This couldn't be tuned for:
- Slow/mobile connections (overwhelming network)
- Fast connections (underutilizing capacity)
- Different CPU counts

### Solution

#### Backend
The backend already supported `concurrency` parameter in `scanRequest` struct and passes it to the `Scanner`. No changes needed.

#### Frontend

**Added i18n keys** (English + Persian):
- `settings.concurrencyLabel`: "Concurrent workers"
- `settings.concurrencyTitle`: "Number of parallel endpoint tests (default: 256 for native handshake, 12 for noise mode)"

**EndpointScanner.svelte:**
- Added `concurrency` state variable (persisted to `endpointConcurrency` setting)
- Added input field in Advanced Settings panel: "Concurrent workers"
- Input has placeholder "0 (auto)" to indicate default behavior
- Sends `concurrency` in params to `/api/scan`
- Layout: placed in a row with "Probe timeout" for symmetry

**UI Location:**
```
Settings
  └─ Advanced settings (dropdown)
     ├─ UDP Noise toggle
     ├─ Probe timeout (ms) | Concurrent workers  [new field]
     └─ Stop after N results | Notify when finished
```

### Behavior
- **0 or empty**: Uses backend default (`DefaultConcurrency` function)
  - Native handshake: 256
  - Noise mode: 12
- **Positive integer**: Uses that exact value
- Validated server-side (existing code already handles this)

### Impact
- Users can tune concurrency for their network/CPU
- Advanced users can optimize scan speed
- Defaults remain unchanged (0 = auto)
- No breaking changes to existing scans

### Files Changed
- `frontend/src/locales/en.json`
- `frontend/src/locales/fa.json`
- `frontend/src/components/EndpointScanner.svelte`

---

## Testing Recommendations

1. **UI Build in CI**: Verify that CI builds pass and produce valid binaries with embedded UI
2. **Module Import**: Test `go get github.com/QMahyar/Cloudflare-Scanner@latest` (though not expected to be used since it's `package main`)
3. **Concurrency Control**: 
   - Test with 0 (auto) → should use defaults
   - Test with custom value (e.g., 50) → should respect it
   - Test with very low (5) and very high (1000) values → should work but may be slow/overwhelming

## Documentation Updates Needed

- **README.md**: Add note about `npm run build` requirement before `go build`
- **BUILD.md**: Update build instructions to mention UI build step
- **AGENTS.md**: Update module name references if any

---

## Rollout Plan (completed)

1. ✅ Committed these changes
2. ✅ `ui/dist/` moved out of git and into `.gitignore`; CI now rebuilds it before every Go step
3. ✅ CI verifies builds across all platform targets
4. ✅ Merged to master
5. ✅ Release workflow builds the UI automatically

---

## Follow-up status (updated 2026-07-14)

Items originally listed as "not implemented" — current state:

- **IP Scanner concurrency controls**: ✅ shipped — the clean-scan request exposes
  `phase1_probes` / `phase2_probes` (clamped to `maxCleanPhase1Probes` /
  `maxCleanPhase2Probes` in `server.go`).
- **Rate limiting / resource-exhaustion guards**: ✅ partially shipped — request
  inputs (`count`, concurrency, probe counts, and the port list) are clamped
  server-side so no single request can drive an unbounded allocation.
- **Auto-tuning** (detect CPU/network to suggest concurrency): not implemented.
- **Persistent scan state** (resume after restart): not implemented.

---

**Original implementation date**: 2026-06-13  
**Reviewed By**: Architecture review findings  
**Implemented By**: AI Assistant (Claude Sonnet 4.5)
