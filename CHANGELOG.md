# Changelog

All notable changes to Cloudflare Scanner are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

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
