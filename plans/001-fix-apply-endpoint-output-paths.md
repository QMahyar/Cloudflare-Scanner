# Plan 001: Fix apply-endpoint output paths + handler-level tests

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 5945765..HEAD -- scan_handlers.go parsers_test.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: bug
- **Planned at**: commit `5945765`, 2026-07-15

## Why this matters

The Replacer tab's **Browse** button calls `/api/select-output-dir`, which
returns an arbitrary absolute path chosen by the OS folder picker. The apply
handler then rejects any path outside the install directory with
`output_dir must stay inside the app directory`. UI copy and docs promise free
folder choice (`E:\vpn\WG Configs\modified`). The path-traversal "test" only
unit-tests a local `reject` helper and never calls `handleApplyEndpoint`, so
the bug survived.

This is a local desktop tool; the user already picked the folder via a native
dialog. Absolute paths anywhere the process can write are correct. Keep
rejecting path tricks that would write outside a *relative* base (e.g. `..`
components when resolving relative to `exeDir`), and still strip filenames with
`filepath.Base`.

## Current state

- `scan_handlers.go` — `handleApplyEndpoint` (approx lines 585–707):

  ```go
  outputDir := r.FormValue("output_dir")
  if outputDir == "" {
      outputDir = exeDir
  }
  outputDir = filepath.Clean(outputDir)
  if !filepath.IsAbs(outputDir) && (strings.HasPrefix(outputDir, "/") || strings.HasPrefix(outputDir, `\\`)) {
      jsonError(w, "output_dir must be relative to app directory or an absolute path inside it", 400)
      return
  }
  if !filepath.IsAbs(outputDir) {
      outputDir = filepath.Join(exeDir, outputDir)
  }
  rel, err := filepath.Rel(exeDir, outputDir)
  if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
      jsonError(w, "output_dir must stay inside the app directory", 400)
      return
  }
  ```

- `pickers.go` — `handleSelectOutputDir` returns any user-chosen absolute path.
- `parsers_test.go:670-708` — `TestHandleApplyEndpoint_PathTraversal` reimplements
  a local `reject` function and never calls the HTTP handler.
- Conventions: JSON errors via `jsonError(w, msg, code)`; multipart cleanup with
  `defer r.MultipartForm.RemoveAll()`; `filepath.Base(fh.Filename)` for writes.
  See existing `TestHandleApplyEndpoint_InvalidEndpointFormat` for httptest style.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Tests   | `go test ./... -count=1` | all pass |
| Vet     | `go vet ./...` | exit 0 |
| Format  | `gofmt -l scan_handlers.go parsers_test.go` | empty |

## Scope

**In scope**:

- `scan_handlers.go` (`handleApplyEndpoint` path resolution only)
- `parsers_test.go` (replace/extend apply-endpoint path tests)

**Out of scope**:

- `pickers.go` (folder picker is fine)
- Frontend i18n/docs (plan 004)
- CSRF / other handlers
- Changing how Endpoint lines are rewritten inside configs

## Git workflow

- Branch: `advisor/001-fix-apply-endpoint-output-paths`
- Commit style (from `git log`): `fix: allow absolute apply output dirs outside install dir`
- Do NOT push unless the operator instructed it.

## Steps

### Step 1: Extract or rewrite path resolution

In `handleApplyEndpoint`, replace the "must stay inside exeDir" check with:

1. Empty `output_dir` → `exeDir` (unchanged).
2. `filepath.Clean` the value.
3. If not absolute, join with `exeDir` (relative paths stay under the app dir —
   this preserves the safe default for typed relative paths).
4. If absolute, use it as-is after Clean (this is what Browse returns).
5. Reject empty result, and reject paths that after Clean still contain a
   `..` path element that escapes a relative base — for absolute paths, Clean
   already resolves `..`; do **not** require Rel(exeDir) to be non-escaping.
6. Optionally reject the Windows-only weird case of a non-absolute path that
   still looks rooted (`/` or `\\` prefix without volume) — keep that existing
   check if it still makes sense after Clean; otherwise drop it with a comment.

Suggested helper (same package, can live just above the handler or next to
`validateEndpointHostPort` in `httpserver.go` if you prefer shared helpers —
prefer keeping it in `scan_handlers.go` or a tiny `resolveApplyOutputDir(exeDir, raw string) (string, error)` in `scan_handlers.go` so tests can call it directly):

```go
// resolveApplyOutputDir turns the user-supplied output_dir into an absolute
// directory. Empty → exeDir. Relative → filepath.Join(exeDir, cleaned).
// Absolute → cleaned absolute path (any drive/folder the OS allows).
func resolveApplyOutputDir(exeDir, raw string) (string, error) {
    if strings.TrimSpace(raw) == "" {
        return exeDir, nil
    }
    out := filepath.Clean(raw)
    if !filepath.IsAbs(out) {
        // Reject half-rooted forms that Clean leaves non-absolute on some OSes.
        if strings.HasPrefix(out, "/") || strings.HasPrefix(out, `\`) {
            return "", fmt.Errorf("output_dir must be a relative path or an absolute path")
        }
        out = filepath.Join(exeDir, out)
    }
    // Final Clean after Join.
    out = filepath.Clean(out)
    if out == "" || out == "." {
        return "", fmt.Errorf("invalid output_dir")
    }
    return out, nil
}
```

Wire the handler to use this helper and keep `os.MkdirAll(outputDir, 0755)`.

**Verify**: `gofmt -l scan_handlers.go` → empty; code compiles via step 3.

### Step 2: Replace the mock path test with real handler tests

Delete or rewrite `TestHandleApplyEndpoint_PathTraversal` so it:

1. Builds a real multipart POST to `handleApplyEndpoint` with:
   - a valid endpoint `1.2.3.4:2408`
   - a minimal WireGuard config body containing `[Peer]` + `Endpoint = old:1`
   - `output_dir` set to cases below
2. Cases:
   - **empty** `output_dir` → 200 when write succeeds under a temp-controlled
     environment **OR** if writing next to the test binary is awkward, unit-test
     `resolveApplyOutputDir` for empty/relative/absolute and use httptest only for
     absolute temp dir write success + relative escape rejection if any remains.
   - **absolute temp dir** (outside any install path) → **200**, file written,
     response `saved >= 1`, file content contains `Endpoint = 1.2.3.4:2408`.
   - **relative `subdir`** → resolves under exeDir (unit-test on helper).
   - **invalid endpoint** already covered — leave those tests.

Preferred split (cleanest for executors):

- `TestResolveApplyOutputDir` — table-driven pure tests for the helper.
- `TestHandleApplyEndpoint_WritesToAbsoluteDir` — httptest multipart with
  `output_dir = t.TempDir()`, assert file on disk.
- Remove the old local-`reject` test that never called the handler.

Model multipart construction after existing tests in `parsers_test.go` if any;
otherwise use `mime/multipart.Writer` with fields `endpoint`, `output_dir`, and
file field `configs`.

**Verify**: `go test -run 'TestResolveApplyOutputDir|TestHandleApplyEndpoint' ./... -count=1` → pass

### Step 3: Full suite

**Verify**:

- `go test ./... -count=1` → all pass
- `go vet ./...` → exit 0
- `gofmt -l scan_handlers.go parsers_test.go` → empty

## Test plan

| Test | File | Cases |
|------|------|-------|
| `TestResolveApplyOutputDir` | `parsers_test.go` or new `scan_handlers_test.go` | empty→exeDir; relative join; absolute preserved; weird half-rooted rejected if policy keeps it |
| `TestHandleApplyEndpoint_WritesToAbsoluteDir` | same | absolute outside exeDir succeeds; Endpoint line updated |

Pattern: table-driven like `TestValidateEndpointHostPort` / existing apply tests.

## Done criteria

- [ ] Absolute `output_dir` outside install dir is accepted and files are written there
- [ ] Empty `output_dir` still defaults to `exeDir`
- [ ] Relative `output_dir` still resolves under `exeDir`
- [ ] Old mock-only `reject` path test is gone or rewritten to call real code
- [ ] `go test ./... -count=1` passes
- [ ] `go vet ./...` exits 0
- [ ] No files outside scope modified
- [ ] `plans/README.md` status row → DONE (if you maintain the index)

## STOP conditions

- `handleApplyEndpoint` signature or multipart field names differ from the plan.
- Writing tests requires changing CSRF middleware behavior (don't; call the
  handler function directly with httptest, not through `csrfMiddleware`).
- You believe absolute-path writes need a confirmation dialog — do not add UI;
  report and stop.

## Maintenance notes

- Plan 004 updates docs/i18n to match free absolute paths.
- Reviewers: confirm `filepath.Base` still strips uploaded names so a crafted
  multipart filename cannot escape the chosen output directory.
