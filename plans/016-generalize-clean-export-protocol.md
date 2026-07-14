# Plan 016 (design/spike): Make clean-IP export protocol-neutral (Trojan/VMess, not just VLESS)

> **Executor instructions**: This is a DESIGN/SPIKE plan. The goal is to make a small,
> well-scoped change AND surface any open questions before deep implementation. Follow
> the steps, run the verifications, and if you hit a "STOP condition" (including an
> unresolved design question), stop and report rather than guessing. Update
> `plans/README.md` when done.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted changes
> on commit `6f7a19c`. Open the files in "Current state" and confirm the excerpts match. On mismatch, STOP.

## Status

- **Priority**: P3
- **Effort**: M
- **Risk**: LOW-MED (touches a user-facing export contract + UI + i18n)
- **Depends on**: none
- **Category**: direction
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

The clean-IP scanner already works end-to-end for **any** protocol its parser handles:
`ParseProxyURL` reads vless/trojan/vmess, Phase-2 validation (`BuildXrayJSONBatch`)
builds outbounds for all of them, and `GenerateExport` (`cleanip.go:1381`) loops
through the protocol-agnostic `GenerateShareURL` — which emits vless/trojan/vmess URLs
correctly. **The capability exists.** But the entire export/import *surface* is named
and shaped as VLESS-only: the request field is `VLESSURL` / `vless_url`, the export
filename is hard-coded `clean_ips_vless.txt`, the file header says "Clean IP VLESS
Configs", the UI variable is `vlessURL`, and validation error strings say "vless_url
required". A Trojan or VMess user can paste their config and scan, but the UX pretends
it's VLESS, which is confusing and quietly discourages non-VLESS use. This is a rename +
derive-filename-from-protocol change over an already-correct engine — high-value polish
that unlocks the tool's real breadth.

## Current state

- `server.go:1518-1555` — `handleCleanExport`:
  ```go
  type cleanExportRequest struct {
      VLESSURL  string   `json:"vless_url"`
      Endpoints []string `json:"endpoints"`
  }
  // ...
  cfg, err := ParseProxyURL(req.VLESSURL)   // already parses vless/trojan/vmess
  urls := cfg.GenerateExport(req.Endpoints) // already protocol-agnostic
  content := "# Clean IP VLESS Configs\n" + ...
  w.Header().Set("Content-Disposition", "attachment; filename=clean_ips_vless.txt")
  ```
- `server.go:1142` and `1159-1176` — `handleCleanScanStart` also uses `VLESSURL` /
  `vless_url` for the Phase-2 validation config (same rename applies there).
- `cleanip.go:1381-1389` — `GenerateExport` (protocol-neutral; no change needed to logic):
  ```go
  func (c *ProxyConfig) GenerateExport(endpoints []string) []string {
      urls := make([]string, 0, len(endpoints))
      for _, ep := range endpoints {
          clone := c.WithEndpoint(ep)
          urls = append(urls, clone.GenerateShareURL())
      }
      return urls
  }
  ```
- Frontend `frontend/src/components/IpScanner.svelte`: `vlessURL` variable,
  `vless_url` in request bodies (lines ~211, ~399), and download filenames
  `clean_ips_vless.txt` / `nearby_ips_vless.txt` (~line 403).
- i18n: `frontend/src/locales/en.json` + `fa.json` — any "VLESS URL" labels (grep for `vless`).
- `ProxyConfig.Protocol` holds `"vless"`/`"trojan"`/`"vmess"` after parse — use it to
  derive the filename/header.

Convention: JSON fields are snake_case; user-facing strings are bilingual (en.json +
fa.json, keyed identically). A share-URL round-trip contract is tested in `parsers_test.go`.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Vet | `go vet ./...` | exit 0 |
| Build Go | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Tests | `go test ./...` | ok |
| Build UI | `cd frontend && npm run build` | exit 0 |
| Grep vless surface | `grep -rn "vless_url\|VLESSURL\|vlessURL\|clean_ips_vless" server.go frontend/src` | the sites to rename |

## Scope

**In scope**:
- `server.go` — rename request fields to protocol-neutral names **while keeping
  `vless_url` accepted for back-compat** (see Step 1); derive filename + header from `cfg.Protocol`
- `frontend/src/components/IpScanner.svelte` — rename var, send the new field, derive filename
- `frontend/src/locales/en.json` + `fa.json` — relabel "VLESS URL" → "Config URL" (both languages)
- `ui/dist/**` — regenerated via `npm run build`
- Optionally a small test asserting export filename/header follows protocol

**Out of scope**:
- `GenerateExport` / `GenerateShareURL` / `ParseProxyURL` logic — already correct; do NOT change.
- Phase-2 validation engine.
- Adding NEW protocols to the parser — this is about surfacing existing support.

## Design decisions to make (resolve in Step 0, report if blocked)

1. **Back-compat of the JSON field.** Recommended: accept BOTH `config_url` (new) and
   `vless_url` (legacy) on the request, preferring the new one. This avoids breaking any
   saved client/bookmarklet. Confirm there's no external caller that only sends `vless_url`
   you'd strand — since the UI is the only client and is updated in lockstep, dual-accept is safe and cheap.
2. **Filename scheme.** Recommended: `clean_ips_<protocol>.txt` (e.g. `clean_ips_trojan.txt`),
   falling back to `clean_ips.txt` if protocol is empty/unknown.
3. **Header text.** `# Clean IP <PROTOCOL> Configs` derived from `cfg.Protocol` (uppercased), or a neutral `# Clean IP Configs`.

## Git workflow

- Branch: `advisor/016-protocol-neutral-export`
- Commit style: conventional commits, e.g. `feat: protocol-neutral clean-IP export (trojan/vmess, not just vless)`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 0: Decide the design points above

Write down your choices for the three decisions (default to the recommendations). If
you discover an external caller that would break from a dual-accept field rename, STOP
and report — otherwise proceed with dual-accept.

### Step 1: Backend — dual-accept field + derived filename/header

In `cleanExportRequest` (and the scan-start request struct at `server.go:1142`), add a
`ConfigURL string \`json:"config_url"\`` field alongside the existing `VLESSURL`. Resolve
the effective URL as `configURL := req.ConfigURL; if configURL == "" { configURL = req.VLESSURL }`.
Update the required-field check + error message to be protocol-neutral
("config_url required"). Derive:
```go
proto := cfg.Protocol
if proto == "" { proto = "config" }
filename := fmt.Sprintf("clean_ips_%s.txt", proto)
header := fmt.Sprintf("# Clean IP %s Configs\n", strings.ToUpper(proto))
```
and use them for the header text and `Content-Disposition`.

**Verify**: `go build` → exit 0; `go vet ./...` → exit 0.

### Step 2: Backend test

Add a test (in `parsers_test.go` or a new `export_test.go`) that `GenerateExport` for a
**trojan** config produces `trojan://...` URLs (proving the engine is protocol-neutral)
and — if you factor the filename/header into a testable helper — that a trojan config
yields `clean_ips_trojan.txt`. Model on existing table tests.

**Verify**: `go test ./...` → ok.

### Step 3: Frontend — send new field, derive filename, relabel

In `IpScanner.svelte`: rename `vlessURL` → `configURL` (keep the input working), send
`config_url` in the request bodies, and set the download filename from the response's
`Content-Disposition` if present, else a protocol-neutral default. Relabel the input's
placeholder/label via i18n. Update `en.json` + `fa.json` keys (both languages, keyed
identically) from "VLESS URL" to "Config URL" (or "Proxy Config URL").

**Verify**: `grep -rn "vless" frontend/src/components/IpScanner.svelte` → no user-facing "VLESS-only" labels remain (internal back-compat aside).

### Step 4: Rebuild UI + full verification

```
cd frontend && npm run build && cd ..
go build -ldflags="-s -w" -o /dev/null .
go test ./...
```

**Verify**: all exit 0 / ok.

### Step 5: Report open questions

In your completion report, list anything you deferred: e.g. whether to fully drop the
legacy `vless_url` field in a future major version, and whether the UI should show the
detected protocol next to the input. These are follow-ups, not blockers.

## Test plan

- Backend: `GenerateExport` on a trojan config → trojan URLs; filename helper → protocol-based name.
- Round-trip suite (`parsers_test.go`) stays green (this plan doesn't touch parse/generate).
- Manual (if runnable): paste a trojan config, run a clean scan, export → file named
  `clean_ips_trojan.txt` with trojan URLs.
- Verification: `go test ./...` all pass; UI builds.

## Done criteria

- [ ] Request accepts `config_url` (and still `vless_url` for back-compat)
- [ ] Export filename + header derive from `cfg.Protocol` (trojan → `clean_ips_trojan.txt`)
- [ ] UI sends `config_url`, labels are protocol-neutral in en.json + fa.json
- [ ] `ui/dist` rebuilt
- [ ] New backend test proves protocol-neutral export; `go test ./...` green
- [ ] `go vet` + `go build` green
- [ ] Only in-scope files modified
- [ ] Completion report lists deferred follow-ups
- [ ] `plans/README.md` status row updated

## STOP conditions

- A non-UI external caller depends on `vless_url` in a way dual-accept can't cover — report before renaming.
- `GenerateShareURL` does NOT actually emit correct trojan/vmess URLs when you test it
  (contradicting the premise) — STOP; that would be a parser/generator bug for a separate plan.
- The i18n keys aren't mirrored between en.json and fa.json — fix the mirror or STOP (the repo requires both).

## Maintenance notes

- When a new protocol is added to `ParseProxyURL`, export "just works" — but add a
  round-trip test case (see plan 008) and confirm the filename scheme covers it.
- Reviewer: confirm back-compat (`vless_url` still works) and that both i18n files changed.
- Follow-up (deferred): consider dropping `vless_url` in the next major version and
  showing the detected protocol in the UI.
