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

`main.go` requires `xray`/`xray.exe` co-located with the executable — it exits with a download link if missing. It calls `startServer` (binds `127.0.0.1:0`, OS-assigned random port), prints the URL, auto-opens a browser, then blocks until an interrupt/terminate signal (`waitForShutdown`), on which it removes the `_xray_work`/`_xray_clean` temp dirs and exits. (xray is still required for clean-IP Phase 2 and the WARP noise fallback; the native WARP handshake path doesn't use it.)

The UI is a Vite + Svelte 5 app under `frontend/`, built to `ui/dist/` and embedded via `//go:embed all:ui/dist` in `server.go` — **the binary is fully self-contained at runtime except for the xray sidecar.** The committed `ui/dist/` bundle is what `go build` embeds, so a UI change is a two-step rebuild: `cd frontend && npm run build` (regenerates `ui/dist/`), then `go build`. Source lives in `frontend/src/` (one `*.svelte` component per tab under `components/`, shared logic in `lib/`, i18n in `locales/en.json`+`fa.json`). Node is only needed to rebuild `ui/dist`, never to `go build`.

`module` is `WarpEndpointScanner` (legacy name); the product/repo is `Cloudflare-Scanner`. Both names are load-bearing — don't "fix" the module path.

## The three features and how they map to code

The app is one Go binary serving a 3-tab UI. Each tab is a distinct pipeline:

1. **Endpoint Scanner** (WARP WireGuard) — `server.go:handleScanStart` → `runScan` → `scanner.go`. Generates random WARP endpoints (`endpoint.go`, a curated list of `188.114.*`/`162.159.*` prefixes + WARP-specific UDP ports), then validates each with a **native WireGuard handshake** (`warp_probe.go:WarpHandshakeProbe` — a Noise_IKpsk2 handshake-initiation over UDP using the uploaded `.conf`'s registered private key + peer public key + reserved triple; success = a handshake response, latency = its RTT). No xray process, no SOCKS hop. **Exception:** when noise/AmneziaWG obfuscation is requested it falls back to spinning up a real xray-core WireGuard outbound (`xray.go:GenerateConfig`/`StartXray`) → SOCKS5 → `GET /generate_204` (204 = success), because only xray applies that obfuscation on the wire. When no `.conf` is uploaded, `scanner.TCPOnly` is set and it degrades to a plain TCP dial — a reachability hint only, since WARP is UDP and TCP can't confirm a working endpoint.

2. **IP Scanner** (clean Cloudflare IPs) — `server.go:handleCleanScanStart` → `cleanip.go:runCleanScan`. **Two-phase**: Phase 1 is a fast concurrent TCP dial (default 500 concurrency) across weighted-random IPs from Cloudflare's published CIDR ranges (`cfIPv4CIDRs`/`cfIPv6CIDRs`) on the official `CFCDNPorts`. The fastest responders are then **enriched in three bounded, concurrent top-N passes** kept off the dial hot path (so dense ranges aren't throttled): colo/loc via `/cdn-cgi/trace` (`buildColoMap`), **quality** — real packet-loss + jitter from K independent dials, folded with latency into a 0–100 `qualityScore` (`measureQuality`/`applyQuality`, `metrics.go`) — and **HTTP/3 reachability** via a QUIC `/cdn-cgi/trace` (`http3.go:buildH3Map`, `quic-go`). These surface as Score / Loss / QUIC / Colo columns. Phase 2 takes the top-N fastest and validates each through a real xray VLESS/Trojan outbound (`proxy.go:BuildXrayJSON`), same SOCKS5→204 check; Phase-2 results inherit the edge IP's measured loss/jitter by IP. "Nearby scan" expands the /24 (v4) or /64 (v6) around **all** Phase-1 responders (bounded by `maxNearbyEndpoints`) as an extra, separately-tracked result set. `OnePhase`/`SkipPhase2` stops after Phase 1. When the request carries `custom_ranges`, `iprange.go:ParseIPRanges`+`GenerateFromRanges` replace the random CF-pool generator (CIDR / `a-b` / short `a-N` / single IP, v4+v6; small inputs enumerated, large ones sampled).

3. **IP Replacer** — `server.go:handleReplacer*` → `replacer.go` + `proxy.go`. Fetch a subscription URL or paste raw configs → parse `vless://`/`trojan://`/`vmess://` (`ParseProxyURL`; VMess is a base64-JSON payload handled by `ParseVMessURL`) → dedupe by full `ConfigFingerprint` → cross-product the chosen configs with new IP:port endpoints, emitting fresh share URLs (`GenerateShareURL`). No scanning here — pure parse/transform.

`proxy.go` is the heaviest file and the one to read before touching protocol handling: `ProxyConfig` is the universal struct for all three protocols; `BuildXrayJSON` → `buildOutboundSettings` + `buildStreamSettings` translate it into xray's outbound/streamSettings JSON, covering security `tls`/`xtls`/`reality` and transports `ws`/`grpc`/`kcp`/`httpupgrade`/`raw`(+http header). `GenerateShareURL` is the inverse (struct → URL) and must stay round-trip-consistent with the parser.

## Concurrency & job lifecycle (the part that bites)

- **Jobs are in-memory maps** (`scanJobs`/`cleanJobs`) keyed by `job_N`/`clean_N`, guarded by package-level mutexes. Each job is **auto-deleted 10 minutes after completion** (`jobTTL` via `scheduleScanJobCleanup`). The frontend streams **status** over SSE (`GET /api/scan-events/<id>` / `/api/clean-events/<id>`, shared `streamSSE` helper — snapshots the job every 250ms and emits a frame only when the JSON changes; `lib/sse.js:subscribeStatus` falls back to `/api/status` / `/api/clean-status` polling when EventSource is unavailable). **Results are fetched event-driven off that status channel**: each scanner component refetches **results** (`/api/results/<id>` / `/api/clean-results/<id>`) on a throttled `scheduleFetch` triggered by status frames (a frame only arrives when the job changed), plus a forced final fetch on completion so the enriched (score/loss/QUIC/colo) terminal snapshot always lands — no blind `setInterval`. Results are cumulative and re-sorted/-filtered client-side on read. Large result tables are virtualized via `components/VirtualTable.svelte` (@tanstack/svelte-virtual): it pushes the visible window into plain `$state` from the virtualizer's store subscription, because under Svelte 5 runes a `$derived($virtualizer…)` never re-renders (the wrapper re-emits the same mutated instance, which a reference-deduped store-rune ignores).
- **Cancellation is two-layer**: an HTTP `/api/stop/<id>` closes the job's `Cancel` channel (once, via `sync.Once` in `job.stop()` — concurrent stops won't panic), which a goroutine bridges to a `context.Context` cancel. Honor *both* `ctx.Done()` and `<-job.Cancel` when adding scan loops; partial results are kept on cancel.
- **xray is pooled per batch, not spawned per endpoint.** Both xray paths (clean-IP Phase 2 → `validateBatchWithXray` + `proxy.go:BuildXrayJSONBatch`; WARP noise fallback → `scanner.go:scanBatchNoise` + `xray.go:GenerateConfigBatch`) build ONE config with a SOCKS inbound + outbound + routing rule **per endpoint** and run the whole batch through a single process (`phase2BatchSize`/`batchSize` = 16, with `concurrentBatches` derived from the `p2Probes`/`scanner.Concurrency` knob). This collapses the dominant cost — process spawn + up-to-4s port wait, previously paid per endpoint — by the batch factor, while each endpoint keeps its own port and independent 204 check. Failures in a batch are retried once in a fresh batch (mirrors the old 2-attempt behavior). The SOCKS+204 core lives once in `cleanip.go:socks204Probe` / `scanner.go:probe204`. Each batch gets a **non-overlapping SOCKS port window** from an atomic counter (WARP band `+10800`, clean band `+20799`). Per-batch xray config + log live in a temp dir `os.RemoveAll`'d via `defer`, both **under `os.TempDir()`** (not the repo or app dir, so scans work from read-only install locations): WARP `os.TempDir()/_xray_work/wgbatch_<port>/`, clean-IP `os.TempDir()/_xray_clean/batch_<port>/`. The native WARP handshake path uses no xray and no work dir at all.
- Always `cmd.Process.Kill()` + `cmd.Wait()` xray children (see `StopXray`); a leaked xray holds its ports and the work dir. Each batch readiness-waits on its **highest** inbound port (xray binds inbounds in array order).
- **SSE streams clear their write deadline** (`http.NewResponseController(w).SetWriteDeadline(time.Time{})` in `streamSSE`): the server-wide `WriteTimeout` is correct for normal JSON handlers but would otherwise sever a long scan's event stream at 30s (the cause of `ERR_INCOMPLETE_CHUNKED_ENCODING`). `ctx.Done()` still tears the stream down on client disconnect.

## Security boundaries (don't regress these)

- Server binds **127.0.0.1 only**; `handleIndex` sets CSP/nosniff/referrer headers.
- `handleApplyEndpoint` writes modified `.conf` files but **confines output strictly inside the app directory** via `filepath.Rel` + `..` checks, and uses `filepath.Base` on uploaded filenames (path-traversal guard). Endpoints are validated with `net.SplitHostPort`.
- `FetchSubscription` allows only `http`/`https` schemes and caps the body at 10 MB; request bodies use `http.MaxBytesReader`.
- WARP "Hogwarts"-style config keys (`S1/S2/S3`, `Jc`, `H1–H4`, etc.) in `.conf` files are community conventions; `config.go` maps `S1/S2/S3` into the `Reserved` triple.

## Conventions

- No HTTP router/framework — plain `http.ServeMux` with prefix path matching (`r.URL.Path[len(prefix):]` to extract IDs). No env-var config; everything flows through the web UI.
- Latency stats (median/best/jitter across N attempts), packet `lossPercent`, and the 0–100 `qualityScore` (latency+jitter+loss; a speed term can be added later by reweighting) live in `metrics.go`; reuse those rather than re-deriving. Both `ScanResult` and `CleanIPResult` carry `Loss`/`Score`; clean IP additionally carries `H3`.
- HTTP/3 / QUIC reachability uses `github.com/quic-go/quic-go` (the one third-party runtime dep beyond `x/crypto`). `http3.go:h3RoundTrip` is split from the port-gated `h3Probe` so the wiring is covered offline by a loopback h3 server (`http3_test.go`) without needing real UDP/443 egress. A blocked-UDP network simply yields `h3=false`.
- Tests cover parsing (`parsers_test.go`), IP generation (`cleanip_test.go`), scoring (`metrics_test.go`), and the HTTP/3 wiring via a loopback server (`http3_test.go`) — the live network/xray paths are not unit-tested. Add table-driven tests in those files for any parser/generator/scoring change.
- Docs are bilingual (English + Persian `.fa.md` / `docs/fa/`); UI i18n lives in `frontend/src/locales/en.json` + `fa.json` (keyed identically, loaded via `svelte-i18n`). Update both sides when changing user-facing strings.
</content>
</invoke>
