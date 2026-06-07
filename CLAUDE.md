# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

A complementary `AGENTS.md` exists with build/CI quick-reference; this file focuses on architecture and non-obvious invariants. Where they overlap, prefer what you verify in the code.

## Commands

```bash
go build -ldflags="-s -w -X 'main.Version=dev'" -o Cloudflare-Scanner.exe .   # build (drop .exe off Windows)
go vet ./...
go test ./...                       # runs parsers_test.go + cleanip_test.go
go test -run TestParseProxyURL ./.  # single test
```

Cross-compile via `GOOS`/`GOARCH` (windows/linux/darwin × amd64/arm64). On PowerShell use `$env:GOOS="..."`.

CI (`.github/workflows/ci.yml`) vets + builds all 6 platform combos and runs `go test` **only on linux/amd64**. Releases trigger on `v*` tags (`.github/workflows/release.yml`), bundling xray-core v1.8.24.

## Runtime model

`main.go` requires `xray`/`xray.exe` co-located with the executable — it exits with a download link if missing. It calls `startServer` (binds `127.0.0.1:0`, OS-assigned random port), prints the URL, auto-opens a browser, then blocks on `select{}` forever. There is no graceful shutdown; closing the terminal kills it.

The UI is a single `ui/index.html` embedded via `//go:embed ui` in `server.go` — **the binary is fully self-contained at runtime except for the xray sidecar.** Edits to `ui/index.html` require a rebuild to take effect.

`module` is `WarpEndpointScanner` (legacy name); the product/repo is `Cloudflare-Scanner`. Both names are load-bearing — don't "fix" the module path.

## The three features and how they map to code

The app is one Go binary serving a 3-tab UI. Each tab is a distinct pipeline:

1. **Endpoint Scanner** (WARP WireGuard) — `server.go:handleScanStart` → `runScan` → `scanner.go`. Generates random WARP endpoints (`endpoint.go`, a curated list of `188.114.*`/`162.159.*` prefixes + WARP-specific UDP ports), then for each spins up a **real xray-core process** with a WireGuard outbound (`xray.go:GenerateConfig`/`StartXray`), waits for its local SOCKS5 port, and proxies an HTTP `GET /generate_204` through it. Success = HTTP 204. When no `.conf` is uploaded, `scanner.TCPOnly` is set and it degrades to a plain TCP dial.

2. **IP Scanner** (clean Cloudflare IPs) — `server.go:handleCleanScanStart` → `cleanip.go:runCleanScan`. **Two-phase**: Phase 1 is a fast concurrent TCP dial (default 500 concurrency) across weighted-random IPs from Cloudflare's published CIDR ranges (`cfIPv4CIDRs`/`cfIPv6CIDRs`) on the official `CFCDNPorts`; survivors are probed for colo/loc via `/cdn-cgi/trace`. Phase 2 takes the top-N fastest and validates each through a real xray VLESS/Trojan outbound (`proxy.go:BuildXrayJSON`), same SOCKS5→204 check. "Nearby scan" expands the /24 (v4) or /64 (v6) around Phase-1 winners as an extra, separately-tracked result set. `OnePhase`/`SkipPhase2` stops after Phase 1.

3. **IP Replacer** — `server.go:handleReplacer*` → `replacer.go` + `proxy.go`. Fetch a subscription URL or paste raw configs → parse `vless://`/`trojan://`/`vmess://` (`ParseProxyURL`; VMess is a base64-JSON payload handled by `ParseVMessURL`) → dedupe by full `ConfigFingerprint` → cross-product the chosen configs with new IP:port endpoints, emitting fresh share URLs (`GenerateShareURL`). No scanning here — pure parse/transform.

`proxy.go` is the heaviest file and the one to read before touching protocol handling: `ProxyConfig` is the universal struct for all three protocols; `BuildXrayJSON` → `buildOutboundSettings` + `buildStreamSettings` translate it into xray's outbound/streamSettings JSON, covering security `tls`/`xtls`/`reality` and transports `ws`/`grpc`/`kcp`/`httpupgrade`/`raw`(+http header). `GenerateShareURL` is the inverse (struct → URL) and must stay round-trip-consistent with the parser.

## Concurrency & job lifecycle (the part that bites)

- **Jobs are in-memory maps** (`scanJobs`/`cleanJobs`) keyed by `job_N`/`clean_N`, guarded by package-level mutexes. Each job is **auto-deleted 10 minutes after completion** (`jobTTL` via `scheduleScanJobCleanup`). The frontend polls `/api/status/<id>` and `/api/results/<id>` on a 300–1500ms `setInterval` — results are cumulative and re-sorted on read.
- **Cancellation is two-layer**: an HTTP `/api/stop/<id>` closes the job's `Cancel` channel (once, via `sync.Once` in `job.stop()` — concurrent stops won't panic), which a goroutine bridges to a `context.Context` cancel. Honor *both* `ctx.Done()` and `<-job.Cancel` when adding scan loops; partial results are kept on cancel.
- Each xray invocation gets a **unique local SOCKS port** derived from atomic counters with large offsets (`+10799`, `+20799`) to avoid collisions across concurrent goroutines. Per-probe xray config + log live in a temp dir `os.RemoveAll`'d via `defer`. **Two different locations**: WARP scans write `<appdir>/_xray_work/<tag>/` (gitignored, created at runtime); clean-IP scans write `os.TempDir()/_xray_clean/<tag>/` — *not* under the repo. Don't assume a single work dir.
- Always `cmd.Process.Kill()` + `cmd.Wait()` xray children (see `StopXray`); a leaked xray holds its port and the work dir.

## Security boundaries (don't regress these)

- Server binds **127.0.0.1 only**; `handleIndex` sets CSP/nosniff/referrer headers.
- `handleApplyEndpoint` writes modified `.conf` files but **confines output strictly inside the app directory** via `filepath.Rel` + `..` checks, and uses `filepath.Base` on uploaded filenames (path-traversal guard). Endpoints are validated with `net.SplitHostPort`.
- `FetchSubscription` allows only `http`/`https` schemes and caps the body at 10 MB; request bodies use `http.MaxBytesReader`.
- WARP "Hogwarts"-style config keys (`S1/S2/S3`, `Jc`, `H1–H4`, etc.) in `.conf` files are community conventions; `config.go` maps `S1/S2/S3` into the `Reserved` triple.

## Conventions

- No HTTP router/framework — plain `http.ServeMux` with prefix path matching (`r.URL.Path[len(prefix):]` to extract IDs). No env-var config; everything flows through the web UI.
- Latency stats (median/best/jitter across N attempts) live in `metrics.go`; reuse those rather than re-deriving.
- Tests cover parsing (`parsers_test.go`) and IP generation (`cleanip_test.go`) — the network/xray paths are not unit-tested. Add table-driven tests in those files for any parser/generator change.
- Docs are bilingual (English + Persian `.fa.md` / `docs/fa/`); UI i18n is a `TR` object in `index.html`. Update both sides when changing user-facing strings.
</content>
</invoke>
