# Changelog

All notable changes to Cloudflare Scanner are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

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
