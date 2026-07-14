# Changelog

All notable changes to Cloudflare Scanner are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

---

## [v3.7.0] — 2026-07-14

Advisor-plan implementation batch (16 plans, `plans/`): correctness/security
hardening, test coverage, two internal refactors, a feature, and doc/CI cleanup.
No breaking API changes — the one field rename is backward-compatible.

### Fixed
- Graceful shutdown now cancels in-flight scan/clean jobs before draining the HTTP
  server, so xray child processes are always killed/waited (previously `os.Exit`
  could skip their deferred cleanup) and open SSE streams no longer stall shutdown
  for the full 5s grace window. `http.ErrServerClosed` is no longer logged as an error.
- The native WARP handshake probe now honors `context.Context`: Stop is responsive
  (aborts within ~700ms) instead of waiting out the full per-probe timeout (up to
  60s) across every in-flight probe.
- The clean-scan port list is now deduped and capped (`maxScanPorts = 64`) —
  previously unbounded, so a large/duplicate-heavy port list could drive an
  outsized endpoint allocation.
- The apply-endpoint value is now validated against control characters/whitespace
  before being written into a `.conf` file, closing a config-injection edge case.

### Added
- Clean-IP export is now protocol-neutral: the request accepts `config_url`
  (the legacy `vless_url` field still works), and the exported filename/header
  derive from the parsed protocol (e.g. `clean_ips_trojan.txt` for a Trojan
  config) instead of always saying "vless".
- Test coverage: replacer `ConfigFingerprint`/`DeduplicateConfigs` (previously
  0%), REALITY/xTLS/gRPC/kcp share-URL round-trips, a WARP-probe context-
  cancellation test, and an offline test suite for the new batch-orchestration runner.
- CI now runs a `gofmt -l` gate (linux/amd64) so formatting regressions are caught.

### Changed
- Internal refactor: the two xray batch-config builders (WARP noise fallback,
  clean-IP Phase 2) now share one `buildBatchXrayConfig`; the two batch
  orchestration loops now share one generic `runBatches[T]`. Structural/offline
  tests pin both to their pre-refactor behavior.
- `server.go` and `cleanip.go` (each 1300–1800 lines) split into focused files
  along their existing seams (pickers / http bootstrap / scan handlers / clean-scan
  handlers / replacer handlers; IP generation / measurement / xray validation /
  orchestration). No behavior change.
- `.go` sources normalized to LF line endings (`.gitattributes`), so `gofmt -l`
  is meaningful going forward.

### Docs
- Corrected the build contract everywhere it was stated wrong: `ui/dist/` is
  git-ignored and must be built with `npm run build` before `go build` — several
  docs previously said the opposite (`CLAUDE.md`, `AGENTS.md`, `README.md`,
  `BUILD.md`, `BUILD.fa.md`).
- `IMPROVEMENTS.md` marked as a historical record and reconciled with what has
  since shipped.
- `sample.conf`'s real-looking WireGuard key material replaced with placeholders.

---

## [v3.6.3] — 2026-06-28

Performance and reliability pass on the scan pipeline. No user-facing behavior
change for normal scans; large scans use far less memory and clean-IP Phase 2 is
more reliable on Linux.

### Fixed

- **Clean-IP Phase-2 SOCKS port band overlapped Linux's ephemeral range.** Batch
  windows were allocated as `20799 + n·16 mod 20000`, topping out near 40798 —
  inside Linux's default ephemeral range (32768–60999). An xray inbound bind
  landing on a port the OS had already handed out as an ephemeral source port
  failed sporadically under load, surfacing as a spurious "xray startup timeout".
  The band now caps at 32766 (`mod 11968`), clear of both the WARP band below and
  the ephemeral range above. Windows/macOS were unaffected (`cleanip.go`).
- **Stop now works in the brief `pending` window** between clean-scan creation and
  Phase 1 starting, instead of returning "scan not running" (`server.go`).
- **Nil-config guard** before Phase 2 dereferences the proxy config (`cleanip.go`).

### Changed

- **Bounded goroutine use in the scan hot loops.** The WARP scan loop and the
  clean-IP Phase-1 dial loop spawned one goroutine per endpoint (gated by a
  semaphore), parking up to `count` goroutines at once — ~200 MB of stacks for a
  100k-endpoint scan. Both now use a fixed worker pool fed over a channel, capping
  live goroutines at the configured concurrency. Cancellation/stop-after semantics
  are preserved, and the clean path now always drains in-flight workers before
  snapshotting results (`server.go`, `cleanip.go`).
- **xray run-log read once per Phase-2 batch** instead of re-read per failed
  endpoint (up to 16× the same file). Failure-cause enrichment moved out of the
  per-endpoint probe into a single post-batch pass, still scoped per IP
  (`cleanip.go`).

---

## [v3.6.2] — 2026-06-25

Hardening release. Closes two SSRF vectors and a DNS-rebinding hole in the local
server, fixes several share-URL round-trip bugs in the IP Replacer, and removes a
latent port-collision and a deadlock from the scan pipeline.

### Security

- **SSRF via DNS rebinding in the subscription fetcher.** `FetchSubscription`
  resolved the host, checked the IPs, then dialed the host *again* — a rebinding
  DNS server could answer the check with a public IP and the dial with
  `127.0.0.1`/a cloud-metadata address. Validation now runs in a `net.Dialer`
  `Control` hook against the concrete connect IP, so the address that's checked is
  the address that's dialed (and every redirect hop is re-checked) (`replacer.go`).
- **Widened the SSRF blocklist** to cover CGNAT `100.64.0.0/10` (RFC 6598),
  `0.0.0.0/8`, and interface-local multicast — none of which `IsPrivate()` catches
  (`replacer.go`).
- **DNS-rebinding to the local API.** A malicious page that rebinds its own
  hostname to `127.0.0.1` could read scan results over the GET endpoints. The
  server now rejects any request whose `Host` header isn't a loopback name before
  it reaches a handler (`server.go`).
- **Folder picker resolves `powershell.exe` by absolute System32 path** instead of
  a `PATH` lookup, closing a PATH-injection vector (`server.go`).

### Fixed

- **VLESS share URLs lost a bare `path=/`.** Many Workers/CDN configs use `path=/`;
  the generator treated `/` as a droppable default and stripped it, producing a
  dead exported config. A bare `/` now survives the parse → URL → parse round trip,
  while path-less configs still emit no `path` (`proxy.go`).
- **VLESS share URLs dropped `encryption=none`.** The field is required by the
  VLESS share-link standard; strict clients reject a URL without it. It's now
  always emitted for VLESS (`proxy.go`).
- **WebSocket/httpupgrade configs picked up a spurious `serviceName`.** The
  gRPC-only path→serviceName derivation ran for every transport. Gated to gRPC
  (`proxy.go`).
- **Non-`xudp` `packetEncoding` was dropped** on regeneration; the generator now
  preserves whatever was parsed (`proxy.go`).
- **WARP noise-scan SOCKS port windows could overlap.** The window stride
  (`(n*16) % 9000`) wasn't aligned to the 16-port batch size, so windows began
  overlapping after ~562 lifetime batches and silently failed 16-endpoint batches.
  Aligned to an exact multiple of the stride (`server.go`).
- **Guarded a 0-concurrency deadlock.** A 0 here made the worker semaphore
  unbuffered and hung the whole job; concurrency is now clamped to ≥1 (`server.go`).
- **Phase-2 cause attribution could match the wrong IP** — `1.2.3.4` substring-
  matched `1.2.3.40` in the shared batch log. It now requires the IP's delimiter,
  for both IPv4 and IPv6 (`cleanip.go`).
- **A failed/short upload of a WARP `.conf` was misreported** as a parse error; the
  `io.Copy` error is now surfaced directly (`server.go`). Multipart uploads also
  clean up any spilled temp files.

---

## [v3.6.1] — 2026-06-24

Bug-fix release. Sharper Phase-2 failure diagnostics and a lighter clean-IP
results payload, alongside the install/doc fixes that had been queued.

### Fixed

- **Phase-2 validation failures could be blamed on the wrong IP.** A whole batch
  of clean IPs is validated through one pooled xray process that shares a single
  run log, but each endpoint's failure reason was mined from the *last* error
  line in that log — so one IP could inherit a neighbour's reset (two endpoints
  reporting an error that named a third IP). `extractXrayError` now only accepts
  a log line naming the failing endpoint's own IP, and lets the honest base error
  stand when none does (`cleanip.go`).
- **Clean-IP result responses carried a redundant duplicate of every successful
  entry** (`entries` plus an identical `raw` the UI never read), doubling the
  JSON on large scans. Removed (`server.go`).
- **One-liner installers could install the wrong binary.** The Unix installers
  only checked CPU architecture, so running `install-linux.sh` in Git Bash/MSYS
  on Windows downloaded a Linux ELF that then failed with `Exec format error`.
  Each installer now verifies the OS first and points users to the correct
  script: `install-linux.sh` requires Linux (and rejects Termux), `install-
  macos.sh` requires macOS, `termux-setup.sh` requires Termux, and `install-
  windows.ps1` rejects non-Windows `pwsh`.
- **Windows one-liner failed on stock Windows PowerShell 5.1** behind older TLS
  defaults. The documented command and the generated release notes now set
  `Tls12` before fetching from GitHub, and the “run as Administrator” note was
  dropped (the installer only edits the per-user PATH).

### Documentation

- README (EN/FA) now lead with the working per-OS one-liners plus manual-install
  fallbacks, and warn against running the Linux installer from Git Bash on
  Windows. Added a matching FAQ entry.
- Corrected the WARP endpoint-scanning description in README and BUILD (EN/FA):
  validation uses the native WireGuard handshake by default and only falls back
  to xray for UDP-noise scans.

---

## [v3.6.0] — 2026-06-19

Quality-ranking release. Results are no longer ordered by latency alone — every
endpoint now gets packet-loss, jitter and an HTTP/3 reachability signal folded
into a single quality score, and the results view visualizes the set.

### Added

- **Quality score (0–100).** Latency, jitter and packet loss combine into one
  rank (`metrics.go:qualityScore`); both scanners sort by it by default. The
  formula leaves room for a future throughput term without touching callers.
- **Packet loss + jitter.** A bounded, concurrent top-N pass dials the fastest
  responders with independent probes so dropped connections actually count as
  loss (clean IP: `cleanip.go:measureQuality`; WARP: derived from the native
  handshake's multi-attempt passes). Surfaced as a **Loss** column.
- **HTTP/3 / QUIC reachability.** Each top edge IP is probed over h3
  (`http3.go`, `github.com/quic-go/quic-go`) and flagged in a **QUIC** column.
  The pass early-bails on UDP-blocked networks so it costs ~one probe there.
- **Result visualizations** (pure SVG, no new frontend deps): latency
  distribution, quality mix, and top-datacenter bars (`ResultCharts.svelte`).
- **CSV + JSON export** of results (`lib/exporters.js`), alongside the existing
  raw/QR/subscription outputs.
- **Recent-scans history** — a local, summary-only log of finished scans
  (`ScanHistory.svelte`).

### Changed

- **Results stream off the SSE status channel** (throttled) instead of a blind
  `setInterval` poll — fewer redundant fetches, and the enriched terminal
  snapshot always lands.
- **Cloudflare IP pools updated to the official compact ranges** (v4 + v6 from
  cloudflare.com/ips); the old hand-split list had missed `172.68.0.0–172.71.*`.
- The two scanner tabs now share one results action bar (`ResultsActions.svelte`).

### Fixed

- **IPv6 generator emitted out-of-range addresses** for non-byte-aligned
  prefixes (e.g. `2a06:98c0::/29`), wasting scan budget on non-Cloudflare IPs.
  Host bits are now masked at the bit level. (`cleanip.go:randomIPv6InCIDR`)
- **Colo column could come up empty** when colo/quality/h3 enrichment ran
  concurrently: the QUIC pass starved the colo TLS handshakes past their
  timeout. h3 now runs after colo+quality and the colo probe timeout is more
  forgiving. (`cleanip.go`)
- Hardened parsing/validation: reject empty address or out-of-range port in
  proxy URLs, reject invalid custom-range lines and replacer endpoints, and cap
  replacer output count. (`proxy.go`, `server.go`)

---

## [v3.5.1] — 2026-06-14

Bug-fix release from a full audit of the scan/parse pipelines. No new features;
behaviour is unchanged except where it was previously broken.

### Fixed

- **Endpoint Scanner hung forever on the "Insane" (5000) and "Massive" (10000)
  depth presets.** The WARP IPv4 endpoint generator draws from a finite pool of
  14 prefixes × 256 = **3584** unique addresses, but its uniqueness loop had no
  attempt bound: once the pool was exhausted, every further iteration drew an
  already-seen IP and `continue`d forever — pinning a CPU core while scan
  progress froze at 0 and never completed. Any requested count above the pool
  size triggered it, and two shipped preset buttons (5000 / 10000) exceed it
  directly. The IPv4/IPv6 loops are now attempt-bounded and simply return fewer
  endpoints when the pool can't supply the full count. Requested count is also
  clamped server-side to 100000. (`endpoint.go`, `server.go`)
- **IPv4-only scans could leak IPv6 endpoints when the IPv4 pool was exhausted.**
  The two generator passes shared one combined target, so an under-filled IPv4
  pass let the IPv6 pass backfill the shortfall — emitting IPv6 endpoints even
  when IPv6 was not selected. Each address family now targets its own count
  independently. (`endpoint.go`)
- **Data race on IP Scanner Phase-1 cancellation.** When a scan was stopped,
  `runCleanPhase1TCP` sorted and returned the results slice while in-flight dial
  goroutines were still appending to it under a mutex — an unsynchronised access
  to the slice header that `go test -race` flags and that could return a torn or
  partial result set (or panic). The cancel path now snapshots results under the
  same lock before sorting. (`cleanip.go`)
- **Concurrent scans of the same type collided on local SOCKS ports.** The
  clean-IP Phase-2 and WARP noise-fallback batch runners allocated their xray
  SOCKS port windows from a *per-job* counter, so two scans of the same type
  running at once handed out identical windows and the second job's xray failed
  to bind — surfacing as spurious "xray startup timeout" failures. Both counters
  are now process-global atomics. (`cleanip.go`, `server.go`)
- **TCP + `http`-header obfuscation was dropped during IP Scanner Phase-2
  validation.** A config using a raw TCP transport with an HTTP header and no TLS
  produced empty stream settings, so validation tested plain TCP instead of the
  HTTP-obfuscated transport that the exported share URL describes — a silent
  mismatch between what was validated and what was emitted. Such configs now
  build the correct `RawSettings`. (`proxy.go`)
- **IP Replacer de-duplication ignored WebSocket early-data parameters.** Two
  configs differing only in `max_early_data` / `early_data_header_name` were
  collapsed into one, silently discarding a distinct variant. These fields are
  now part of the de-dupe fingerprint. (`replacer.go`)
- **Idle-connection leak in the colo (`/cdn-cgi/trace`) probe.** Each probe built
  a one-shot HTTP transport whose connection lingered idle until garbage
  collection (up to ~150 probes per scan); it now disables keep-alives and closes
  idle connections on return. (`cleanip.go`)

### Tests

- Added `endpoint_test.go` covering exact-count generation, the exhausted-pool
  attempt bound (a regression guard that hangs the suite if the cap is removed),
  and the IPv4-only / no-IPv6-leak invariant.

---

## [v3.5.0] — 2026-06-13

### Added

- **Concurrency control in Endpoint Scanner Advanced settings.** Users can now
  tune the number of concurrent workers (default: 0 = auto, which uses 256 for
  native WireGuard handshake or 12 for noise mode). Useful for optimizing scan
  speed on fast connections or preventing network overwhelm on slow/mobile
  connections. (`frontend/src/components/EndpointScanner.svelte`)
- **Live scan progress & summary.** Both scanners now show a status pill with a
  live percentage during the run and a post-scan summary strip (found / scanned
  / best latency / elapsed / rate). In-cell latency meter bars, numbered step
  headings, and result-count chips make large result tables easier to read.
  (`components/ScanProgress.svelte`, `lib/scanMetrics.js`)
- **Build version & local-host indicator** in the header, served from a new
  `/api/version` endpoint (`about.go`).

### Performance

- **xray is now pooled per batch instead of spawned per endpoint.** Clean-IP
  Phase 2 and the WARP noise fallback build one xray config (a SOCKS inbound +
  outbound + routing rule per endpoint) and run a batch of 16 through a single
  process, collapsing the dominant process-spawn + port-wait cost while each
  endpoint keeps its own port and independent 204 check. Failures retry once in
  a fresh batch. (`cleanip.go`, `scanner.go`, `proxy.go`, `xray.go`, `server.go`)

### Changed

- **Frontend build artifacts (`ui/dist/`) no longer committed to Git.** CI/CD
  workflows now automatically build the UI before Go compilation. Local builds
  require `cd frontend && npm run build` before `go build`. Keeps the repository
  clean of generated artifacts and prevents merge conflicts on UI changes.
  (`.gitignore`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`)
- **Module name aligned with repository path.** Changed `go.mod` module from
  `WarpEndpointScanner` to `github.com/QMahyar/Cloudflare-Scanner`, enabling
  proper Go tooling support and `go install` compatibility. (`go.mod`)

### Fixed

- **Release build break:** `sort.js` now exports `latBar`, which
  `EndpointScanner.svelte` imports — the missing export had failed the
  production Vite build (and every release-workflow platform job).
- **Long scans no longer drop their status stream.** The SSE event stream clears
  its write deadline, so the server-wide 30s `WriteTimeout` no longer severs a
  running scan (`ERR_INCOMPLETE_CHUNKED_ENCODING`). Also: categorized WARP
  failure-reason breakdown in the results API, and the colo probe now reuses the
  working config's SNI so DPI doesn't reset the `/cdn-cgi/trace` lookup.

### Documentation

- Added `IMPROVEMENTS.md` documenting the three architectural improvements,
  including rationale, implementation details, testing recommendations, and
  rollout plan.
- Documented the `scripts/dev.ps1` fast inner-loop build script in `AGENTS.md`.

---

## [v3.4.1] — 2026-06-11

### Fixed

- **IP Scanner Phase 2 failed for every clean IP when the config had no explicit
  `sni=`.** Validation repoints the config's address at each candidate IP, after
  which xray fell back to using that raw IP as the TLS SNI — which Cloudflare
  can't route, so the whole tunnel timed out (`no usable response through the
  tunnel`). `WithEndpoint` now pins the original hostname into the SNI before the
  swap (mirroring the existing WS Host→SNI fallback), fixing both validation and
  the exported share URLs. Configs that already set `sni=`, and configs whose
  address is already a bare IP, are unaffected. (`proxy.go`)

### Changed

- **Phase-2 failures now surface the real cause from xray's log** instead of a
  generic "no usable response through the tunnel". A reset handshake now reads as
  `connection reset mid-handshake (likely ISP/DPI filtering or a dead origin)`,
  and per-endpoint errors carry xray's deepest message (e.g. a TLS RST), so an
  all-failed Phase 2 is diagnosable rather than a silent dead end. (`cleanip.go`)

### Security

- Force `esbuild` to `^0.25.0` via an npm `overrides`, clearing the transitive
  `svelte-i18n → esbuild@0.19.12` advisory (GHSA-67mh-4wv8-2f99). Dev-only with
  no runtime exposure — the build already used Vite's patched esbuild and the
  embedded `ui/dist` is byte-identical. (`frontend/package.json`)

---

## [v3.4.0] — 2026-06-11

### Changed

- **Frontend rewritten as a Vite + Svelte 5 SPA** (`frontend/`), built to
  `ui/dist/` and embedded via `//go:embed all:ui/dist`. Replaces the old
  monolithic hand-written HTML/JS UI; behavior, settings keys, and persisted
  localStorage data are unchanged. Each tab (Endpoint Scanner, IP Scanner, IP
  Replacer, About) is now its own component, with shared i18n
  (`locales/en.json` / `fa.json`) and helpers under `lib/`.
- Scan status now streams over Server-Sent Events (`/api/scan-events`,
  `/api/clean-events`) with a polling fallback; result tables are virtualized
  for large result sets.
- **Keyboard accessibility** — all clickable result-row tags (`role="button"`)
  now respond to Enter/Space via a Svelte `activateKey` action. Uses an action
  (not an inline `onkeydown` handler) so the DOM event listener is stable
  across virtual-table scroll re-renders.
- **CI split into two workflows** — Go vet/test/build stays in `ci.yml`; the
  frontend rebuild + `ui/dist` freshness check moves to `frontend.yml` with a
  `paths:` trigger so Go-only pushes don't needlessly run `npm ci && npm build`.

### Fixed

- **AudioContext exhaustion on rapid scan completions** — `beep()` previously
  created a new `AudioContext` per call and closed it 600 ms later. Chrome caps
  concurrent instances at 6; back-to-back completions silently failed beyond
  that limit. `notify.js` now lazily creates one shared context and reuses it.
- **VirtualTable ResizeObserver leak** — the `use:measure` Svelte action for
  virtualizer row measurement had no `destroy()` hook. When virtual rows were
  removed from the DOM on scroll, the `@tanstack/virtual` ResizeObserver entry
  for each node was never released. Added `destroy()` that calls
  `measureElement(null)` to let the virtualizer clean up its observer.

---

## [v3.3.0] — 2026-06-09

### Added

- **About tab** — version, GitHub source link, and an update check (proxied
  through Go via `/api/version` + `/api/update-check` so the page CSP stays locked
  to `'self'`). Includes a crypto donation section (USDT-TRC20/TRON, native
  USDT-on-TON, EVM multi-chain, BTC) with per-network notes, copy, and QR.
- **Copy port-mode toggle** — every "Copy All" is a split button with a ▾ menu to
  copy `ip:port` or IP-only per line. IPv6-aware, global, and persisted; applies to
  all copy/download/QR paths.
- **Colo/location filter** on the IP Scanner results (Direct + Nearby).
- **Settings + results persistence** — per-tab settings and the last working
  results survive a page reload; restored results stay copy/download-able after the
  in-memory job expires.
- **Auto-stop on N results** for the Endpoint and IP scanners — the scan halts
  early once enough working results are found and reports `done` (not cancelled),
  keeping partial results.
- **Completion notifications** — optional per-tab "Notify when finished" toggle
  (desktop Notification + a short WebAudio beep + toast; no external assets).
- **Replacer upgrades** — config naming template (`{remark} {ip} {port} {ep}
  {proto} {n}`), base64 subscription output ("Copy as sub" / "Download sub"), live
  endpoint validation, and per-row Copy + QR on generated configs.
- **Single-source versioning** — a repo-root `VERSION` file is now the one place to
  bump the version; it is embedded into the binary as the ldflags fallback (plain
  `go build` reports the right number), the build scripts default to it, and the
  release workflow fails if a pushed tag doesn't match it.
- **WebSocket early-data support** — `max_early_data` / `early_data_header_name`
  (and the `ed` / `eh` shorthands) are now parsed, emitted into xray's `wsSettings`
  during IP-Scanner Phase-2 validation, and round-tripped through the Replacer so
  BPB / edge-tunnel panel configs validate and regenerate correctly instead of
  having the params silently dropped.
- **Phase-2 failure breakdown** — when IP-Scanner validation finds nothing (or only
  some pass), the results panel now explains *why*: an aggregated reason summary
  ("xray didn't come up in time", "Cloudflare reached but didn't route — check
  SNI/Host", …) plus a few example endpoint+error lines. The `/api/clean-results`
  response now carries `failures[]` and `fail_reasons` (`server.go`, `cleanip.go`).
- **xray runnability check at startup** — the launcher now runs `xray version` and
  prints a clear warning if a present-but-broken xray can't execute, instead of
  letting every Phase-2 validation fail with an opaque timeout (`xray.go`,
  `main.go`).

### Changed

- `dist/` is gitignored so local build archives no longer show up as untracked.
- **Result action bars moved above the table** — Copy All / Download / Export /
  Select are now reachable at the top of every results list (Endpoint scanner,
  IP-Scanner Phase 1 & 2, and Nearby) without scrolling past long tables.
- **Phase-1 timeout clarified** — its tooltip now states it is a give-up deadline,
  not a latency filter, and points to the "Max ms" results filter for keeping only
  fast IPs.

### Fixed

- **IP-Scanner Phase 2 failing for CDN-fronted configs** — the xray validation
  builder now falls back the WebSocket / httpupgrade `Host` header to the SNI when
  no explicit `host=` is set (matching `GenerateShareURL`). Previously xray sent
  `Host: <edge-IP>`, so Cloudflare couldn't route and every candidate failed even
  with a known-good IP (`proxy.go:buildStreamSettings`).
- **Tight Phase-1 timeout discarding reachable IPs** — Phase-1 TCP probing now
  retries once on timeout (refused/unreachable return immediately, so no added
  cost), so a single dropped SYN in the high-concurrency burst no longer drops an
  IP whose real RTT is well under the deadline (`cleanip.go:dialReachable`).

---

## [v3.2.0] — 2026-06-09

### Added

- **Native WireGuard handshake probing for the Endpoint Scanner.** WARP endpoints
  are now validated with a real Noise_IKpsk2 handshake over UDP (`warp_probe.go`),
  using the uploaded `.conf`'s registered credentials, instead of spinning up an
  xray process per endpoint. It tests the protocol WARP actually speaks (UDP), is
  far faster (48 endpoints in ~15 s vs. a process+SOCKS hop each), and reports the
  handshake RTT as latency. xray is still used when noise/AmneziaWG obfuscation is
  requested. Adds a dependency on `golang.org/x/crypto` (blake2s, chacha20poly1305).
- **Working QR codes.** The QR buttons previously fell back to plain text because
  no QR library was ever loaded; a CSP-compatible generator is now inlined, so
  configs/endpoints render as scannable QR codes for mobile import.
- **Cloudflare colo (data-center) column** in the IP Scanner results, with the
  country code — surfacing the `/cdn-cgi/trace` data that was already collected but
  never displayed.

### Changed

- **IP Scanner Phase 1 is much faster on dense ranges.** The `/cdn-cgi/trace`
  colo/loc probe was moved out of the TCP-dial hot path; it now runs as a bounded,
  concurrent enrichment pass over the fastest responders (`buildColoMap`) instead
  of a 2 s round-trip per responder while holding a concurrency slot.
- **xray work dirs now live under `os.TempDir()`** (was the app directory for WARP
  scans), so scanning works when the app is installed in a read-only location.

### Fixed

- Clean-IP validation now requires an exact HTTP 204 (was 204 *or* 200), avoiding
  false positives from captive-portal / edge error pages.
- Apply-results filenames, paths, and config contents are now HTML-escaped before
  rendering (previously unescaped, which could corrupt the view or self-inject).

---

## [v3.1.1] — 2026-06-08

### Fixed

- **`scripts/build.ps1` failed to run on stock Windows PowerShell 5.1** — the file
  was BOM-less UTF-8, so 5.1 decoded its box-drawing/em-dash characters as ANSI and
  the script failed to parse (`Unexpected token '}'`). It only worked under
  PowerShell 7. Added a UTF-8 BOM so it parses and runs on a default Windows install.
- **Local build scripts now resolve the repo from their own location** so they work
  no matter where the repo is moved or which directory they are invoked from:
  - `build.sh` uses `${BASH_SOURCE[0]}` (with symlink resolution) instead of `$0`,
    so it is correct when sourced, symlinked, or run from any path.
  - `build.ps1` falls back from `$PSScriptRoot` to `$MyInvocation` when dot-sourced.
  - Both read `go.mod` via an absolute path rather than a CWD-relative one.

---

## [v3.1.0] — 2026-06-07

### Added

- **Custom-range scanning (IP Scanner)** — a new **IP source** toggle lets you
  scan your own ranges instead of the built-in Cloudflare pool. Accepts CIDR
  (`104.16.0.0/13`), dash ranges (`104.16.0.0-104.16.5.255` and short
  `104.16.0.0-255`), and single IPs — IPv4 and IPv6, one entry per line, with
  `#`/`;` comments. Preset chips insert common Cloudflare ranges (plus **All CF
  IPv4** / **All CF IPv6**), a file picker loads ranges from a `.txt`, and an
  inline help panel lists every supported format. Small ranges are enumerated in
  full; large ones are weighted-sampled to the scan-depth count. New `iprange.go`
  (`ParseIPRanges` + `GenerateFromRanges`); wired through `/api/clean-scan` via a
  `custom_ranges` field. (`iprange.go`, `server.go`, `ui/index.html`)

### Changed

- **Nearby scan now expands around *every* working Phase-1 responder**, not just
  the fastest 10 (the previous hardcoded cap). A new `maxNearbyEndpoints` ceiling
  keeps the total bounded when many IPs respond. (`cleanip.go`)
- **Graceful shutdown** — the server now traps `SIGINT`/`SIGTERM` and removes the
  `_xray_work` / `_xray_clean` work dirs before exiting, instead of blocking on
  `select{}` forever. (`main.go`)

### Fixed

- **Clean-IP `/cdn-cgi/trace` colo probe is now cancellable** — it honors the
  job context instead of `context.Background()`, so stopping a scan no longer
  leaves trace probes running. (`cleanip.go`)
- **IP-scanner source toggle losing its active state** — `selectReplacerMethod`
  used a global `.input-method-bar button` selector that stripped the active
  class off the new source toggle on load; scoped it to the replacer tab.
  (`ui/index.html`)

### Internal

- Removed dead code (`Scanner.Run`, `Scanner.testEndpoint`, `XrayManager.WaitForPort`).
- Deduplicated the replacer `ProxyConfig` ↔ entry mapping into shared helpers. (`server.go`)
- Added tests: `iprange_test.go` (range parsing + smart selection) and a
  share-URL round-trip test for vless/trojan/vmess. (`parsers_test.go`)

---

## [v3.0.1] — 2026-06-07

### Fixed

- **Phase 2 mux interference** — strip `PacketEncoding` (mux/xudp) from the
  xray config used during Phase 2 clean-IP validation. A single `GET /generate_204`
  probe through a mux-enabled outbound would stall or mis-report latency because
  xray never opened the mux sub-connection; Phase 2 now always uses a plain
  single-stream outbound. (`cleanip.go`)
- **Phase 2 failure count missing from API** — `/api/clean-results/{id}` now
  returns `phase2_failures` (count of IPs that went through Phase 2 but produced
  no valid result). The frontend shows this counter in the "no results" empty
  state so users know Phase 2 ran but every IP failed — previously the empty
  message gave no diagnosis. (`server.go`, `ui/index.html`)

### Added

- **Local build scripts** — `scripts/build.sh` (Linux / macOS / Termux) and
  `scripts/build.ps1` (Windows PowerShell). Both scripts:
  - Auto-install Go if missing or too old (reads the required version from `go.mod`)
  - Download the matching xray-core sidecar for each target
  - Produce release-identical `.zip` / `.tar.gz` archives under `dist/`
  - Support building for the host platform, one specific target, or every
    supported platform (`all`): 7 targets ×
    `windows-amd64 · windows-arm64 · linux-amd64 · linux-arm64 · termux-arm64 · darwin-amd64 · darwin-arm64`
  - Accept env-var overrides: `VERSION`, `XRAY_VERSION`, `NO_XRAY`, `NO_ARCHIVE`, `GO_VERSION`

---

## [v3.0.0] — 2026-06-07

### Added

- **Fully responsive UI** — phone, tablet, and PC all supported with a single adaptive layout
- **Safe-area insets** — `env(safe-area-inset-*)` padding on body and toast; correct display on notched phones (iPhone X+, Android punch-hole/island)
- **Icon-only tabs on very small phones (≤ 380 px)** — all three tabs fit at 320 px without overflow; labels reappear above 381 px
- **Tab bar overflow scrolling** — hidden scrollbar, `-webkit-overflow-scrolling: touch`; tabs never clip on any screen width
- **Tablet column wrapping (640–768 px)** — 3-column settings rows (Phase 1/2 probes + Phase 2 count) wrap to 2 + 1 when columns would be too narrow to read

### Changed

- **Button bar** — buttons use natural content width on desktop/tablet (≥ 640 px) instead of stretching to fill the full 1040 px container; equal-width fill is kept on mobile for easy tapping
- **Fetch button** — full-width on mobile (≤ 480 px) to match the URL input above it in the stacked layout
- **App shell** — border-radius reduced on mobile; inner padding tightened
- **Header row** — padding reduced on mobile while keeping the logo and language button readable
- **Port checkboxes** — `min-height: 36 px` + `touch-action: manipulation` for reliable tap targets; bumped to 40 px on narrow phones
- **Overflow prevention** — `overflow-x: hidden` on `html` and `body` stops accidental horizontal scroll bleed
- **Tab text overflow** — ellipsis clip on tab labels so long names never push the tab bar wider than the viewport
- **AGENTS.md** — added source-file map table and clarified CI/two-xray-temp-dir behaviour

---

## [v2.0.1] — 2026-06-03

### Fixed

- **Stop now cancels instantly** — `runScan` and `runCleanPhase1TCP` no longer block on `wg.Wait()` after cancel. Partial results return immediately; in-flight goroutines drain in the background.
- **Progress text wasn't updating** — a local variable `t` in `pollCleanStatus` was shadowing the `t()` translation function, breaking Phase 1/2 progress text with a silent `TypeError`.
- **Reset/Rescan buttons** — Start button now works correctly in TCP-only mode after Reset. Scan/Rescan buttons restore immediately after Stop instead of waiting for the next poll interval.
- **Rescan progress bar** — `startScan` now resets the progress bar width and cancelled class on a fresh run, so Rescan after a cancelled scan shows a clean bar.

### Changed

- `switchTab` renamed `forEach` loop variable to avoid confusion with the `t()` translation function.

---

## [v2.0.0] — 2026-06-03

### Added

- **Port selection in IP Scanner** — choose which Cloudflare CDN ports to scan via a persistent checkbox grid
  - Quick-select presets: **443 only**, **HTTPS (6)**, **HTTP (7)**, **All (13)**, **Config port** (reads port from VLESS URL)
  - Supported ports: HTTP 80, 8080, 8880, 2052, 2082, 2086, 2095 · HTTPS 443, 8443, 2053, 2083, 2087, 2096
  - Each IP is probed on every selected port (count × ports endpoints generated)
  - Nearby scan honours the same port selection
- **One-liner installers** for Linux, macOS, Windows (PowerShell), and Termux
  - Auto-detect CPU architecture, download correct release, add to PATH
- **Version injection** — binary reports its build version in the startup banner via `-ldflags`
- **SHA256 checksums** — `checksums.txt` included in every GitHub Release
- **Windows `.zip` archives** — Windows releases packaged as `.zip` in addition to `.tar.gz` for easier extraction

### Changed

- `GenerateIPs` refactored to accept `[]int` ports; separates unique-IP generation from endpoint building
- `generateNearbyIPs` updated to probe all selected ports per nearby IP
- `CleanIPJob` gains `ScanPorts []int` field
- Release workflow: Go module cache, version ldflags, auto-generate changelog from git log
- CI workflow: added `fail-fast: false`, `cache: true` on setup-go
- `README.md` / `README.fa.md`: full rewrite with "What is this?", one-liner installs, feature table, full workflow guide
- `termux-setup.sh`: install path moved to `~/.local/share/cloudflare-scanner`

---

## [v1.8.0] — 2026-06-03

### Added

- Mobile-responsive UI — tabs, tables, buttons scale to 360 px+ screens
- Per-config output cards with copy button, QR code, and selectable textarea
- Browse button for output folder (native `showDirectoryPicker` on Chromium)
- Touch targets ≥44 px throughout

### Changed

- Persian RTL layout fixes on mobile viewports

---

## [v1.7.0] — 2026-06-02

### Added

- Full UI rewrite — IIFE module pattern, config usage toggles, TCP-only scan mode
- Standardised IP Scanner layout: presets, buttons, and `OutCount` filter

---

## [v1.6.0] — 2026-06-01

### Added

- Nearby-scan feature — after Phase 1, expand around working IPs to find adjacent clean IPs
- Subscription deduplication and cross-product config replacement in IP Replacer
- Phase 2 probes selector (5/12/25/50/100 concurrent validators)

### Fixed

- Security: concurrency and resource-leak hardening (double-close on channels, goroutine leaks)
- Path traversal guard on `apply-endpoint` output directory

---

## [v1.5.0] — 2026-05-30

### Added

- Two-phase clean IP scanning (TCP probe → xray-core validation)
- VLESS/Trojan URL parser for Phase 2 validation
- IP Replacer tab — fetch subscription, deduplicate, replace IP:port in bulk
- xray process manager with SOCKS5 handshake verification

### Changed

- Endpoint generator expanded to 14 IPv4 prefixes, 4 IPv6 prefixes, 55 WARP ports
- Bilingual UI (English + Persian/Farsi) with instant language switching

---

## [v1.0.0] — 2026-05-01

### Added

- Initial public release
- Endpoint Scanner — parallel Warp WireGuard endpoint testing
- UDP noise injection to evade DPI-based Warp blocking
- Embedded web UI served on a random local port
- Self-contained binary with bundled xray-core
- Windows, Linux, macOS, Termux (Android) support
