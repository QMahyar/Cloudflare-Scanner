# Building & Technical Guide

> [نسخه فارسی](BUILD.fa.md)

This document is for **developers** who want to build from source, understand the
architecture, or extend the app. For user installation instructions see [README.md](README.md).

---

## Table of Contents

- [Quick Build](#quick-build)
- [Build Scripts (All Platforms)](#build-scripts-all-platforms)
- [Cross-Platform Builds](#cross-platform-builds)
- [Prerequisites](#prerequisites)
- [Project Structure](#project-structure)
- [Architecture](#architecture)
  - [Endpoint Scanning Flow](#endpoint-scanning-flow)
  - [Clean IP Scanning Flow](#clean-ip-scanning-flow)
  - [IP Replacer Flow](#ip-replacer-flow)
- [HTTP API Reference](#http-api-reference)
- [Config Parsing](#config-parsing)
- [UDP Noise](#udp-noise)
- [Frontend (Svelte UI)](#frontend-svelte-ui)
- [Testing](#testing)
- [CI/CD](#cicd)
- [Environment Variables](#environment-variables)
- [Platform Notes](#platform-notes)
- [Contributing](#contributing)

---

## Quick Build

```bash
git clone https://github.com/QMahyar/Cloudflare-Scanner.git
cd Cloudflare-Scanner
go build -ldflags="-s -w -X 'main.Version=dev'" -o Cloudflare-Scanner .
```

The binary embeds the built frontend (`ui/dist/`, a Vite + Svelte 5 SPA — see
[Frontend (Svelte UI)](#frontend-svelte-ui)) via Go's `//go:embed all:ui/dist`
directive. `ui/dist/` is committed, so a plain `go build` needs no Node — only
rebuild it with `npm run build` when you change `frontend/src/`.

## Build Scripts (All Platforms)

The `scripts/` directory contains two batteries-included scripts that replicate
what CI does: auto-install Go if needed, compile the binary, download the
matching xray-core sidecar, and produce a release-identical archive.

### Linux / macOS / Termux — `scripts/build.sh`

```bash
# Build for the current host platform (auto-detected)
./scripts/build.sh

# Build every supported platform
./scripts/build.sh all

# Build one or more specific platforms
./scripts/build.sh linux-amd64
./scripts/build.sh linux-amd64 darwin-arm64
```

### Windows — `scripts/build.ps1`

```powershell
# Build for the current host platform (auto-detected)
.\scripts\build.ps1

# Build every supported platform
.\scripts\build.ps1 all

# Build one or more specific platforms
.\scripts\build.ps1 windows-amd64
.\scripts\build.ps1 windows-amd64 linux-amd64
```

### Supported platform keys

| Key | OS | Arch |
|-----|----|------|
| `windows-amd64` | Windows | x86-64 |
| `windows-arm64` | Windows | ARM64 |
| `linux-amd64` | Linux | x86-64 |
| `linux-arm64` | Linux / Raspberry Pi | ARM64 |
| `termux-arm64` | Android (Termux) | ARM64 |
| `darwin-amd64` | macOS | Intel |
| `darwin-arm64` | macOS | Apple Silicon |

### Environment overrides

| Variable | Default | Description |
|----------|---------|-------------|
| `VERSION` | `git describe --tags` | Version string baked into the binary |
| `XRAY_VERSION` | `v1.8.24` | xray-core release to bundle |
| `NO_XRAY=1` | — | Skip xray download (binary only) |
| `NO_ARCHIVE=1` | — | Leave loose files in `dist/<platform>/` instead of archiving |
| `GO_VERSION` | `1.26.2` | Go version to auto-install if Go is absent or too old |

### What the scripts produce

```
dist/
├── windows-amd64/
│   ├── Cloudflare-Scanner.exe
│   └── xray.exe
├── linux-amd64/
│   ├── Cloudflare-Scanner
│   └── xray
├── Cloudflare-Scanner-v3.0.1-windows-amd64.zip
├── Cloudflare-Scanner-v3.0.1-linux-amd64.tar.gz
└── ...
```

Artifacts are structurally identical to GitHub Release downloads.

## Cross-Platform Builds

```bash
# Windows (amd64)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .

# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# Linux (arm64) — Raspberry Pi etc.
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .
```

On PowerShell: `$env:GOOS="windows"; $env:GOARCH="amd64"; go build ...`

## Prerequisites

- **Go 1.26+** ([go.dev](https://go.dev/dl/))
- **No C compiler** — pure Go
- Cross-platform: Windows, Linux, macOS, Termux/Android

---

## Project Structure

```
Cloudflare-Scanner/
├── main.go           # Entry point — xray check, server start, browser open
├── server.go         # HTTP handlers, scan jobs, API, embedded UI (//go:embed all:ui/dist)
├── about.go          # /api/version + /api/update-check (GitHub releases)
├── config.go         # WireGuard .conf parser (standard + Hogwarts formats)
├── endpoint.go       # Random WARP endpoint generator (known CF prefixes/ports)
├── scanner.go        # Endpoint scanner — native WireGuard handshake + xray noise fallback
├── warp_probe.go     # Native WireGuard (Noise_IKpsk2) handshake prober
├── xray.go           # xray-core config builder (WireGuard outbound, SOCKS5 inbound)
├── cleanip.go        # Clean IP scanner — CIDR generation, TCP probe, xray validate, nearby scan
├── iprange.go        # Custom IP-range parsing/generation for the IP Scanner
├── replacer.go       # Subscription fetch, config dedup, bulk IP replacement
├── proxy.go          # VLESS/Trojan/VMess URL parser, xray JSON builder, share URL generator
├── metrics.go        # Scan quality helpers (median, best, jitter, success-first sort)
├── noise.go          # UDP noise config and validation
├── parsers_test.go   # Unit tests for parsing, replacer, path traversal
├── cleanip_test.go   # Tests for clean IP generation
├── iprange_test.go   # Tests for custom IP-range parsing
├── replacer_name_test.go # Tests for config naming templates
├── warp_probe_test.go    # Tests for the native WireGuard handshake prober
├── frontend/         # Vite + Svelte 5 SPA source (see Frontend (Svelte UI))
│   └── src/
│       ├── components/  # One *.svelte component per tab
│       ├── lib/          # Shared helpers (api, sse, stores, i18n, ...)
│       └── locales/      # en.json / fa.json (svelte-i18n)
├── ui/
│   └── dist/         # Built frontend bundle, committed and embedded by Go
├── scripts/          # Build + one-liner install scripts per platform
├── docs/             # User guides (English + Farsi)
├── README.md         # User-facing English documentation
├── README.fa.md      # User-facing Farsi documentation
├── BUILD.md          # This file
├── BUILD.fa.md       # Farsi developer guide
├── .github/
│   └── workflows/
│       ├── ci.yml         # CI: Go vet + test + build (6 platform/arch combos)
│       ├── frontend.yml   # CI: frontend rebuild + ui/dist freshness check (paths-filtered)
│       └── release.yml    # Release: build 7 platforms, bundle xray, publish
├── AGENTS.md         # Agent/Copilot instructions
├── CHANGELOG.md      # Release history
├── LICENSE           # MIT
└── sample.conf       # Example WireGuard config
```

---

## Architecture

### Endpoint Scanning Flow

```
User uploads .conf  ──>  ParseWarpConfig() → extracted keys/addresses/reserved
                               │
                 random IP:port │ (prefixes + ports from endpoint.go)
                               ▼
                        runScan() → Scanner(testEndpointAttempts)
                                        │
                          ┌──────────────┼──────────────┐
                          ▼              ▼              ▼
                     Worker 1        Worker 2 ...  Worker N
                          │              │              │
                     testEndpointAttempts (2 attempts)
                     └─ GenerateConfig(endpoint, port) → xray JSON
                     └─ StartXray() → xray process
                     └─ WaitForPort() → SOCKS5 listener up
                     └─ SOCKS5 handshake → HTTP GET generate_204
                     └─ median latency across attempts
                          │              │              │
                          └──────────────┴──────────────┘
                                          │
                                    sortScanResults()
                                    (success-first, then latency)
                                          │
                                    Return results
```

### Clean IP Scanning Flow

```
User provides VLESS URL        CleanIPGenerator
       (optional)                  └─ GenerateIPs()
                                      └─ 25 IPv4 CIDRs + 91 IPv6 CIDRs
                                      └─ Weighted random distribution
                                           │
                                    Phase 1: TCP probe
                                      └─ 500 concurrent workers
                                      └─ net.DialTimeout(ip:port, 3s)
                                      └─ probeCloudflareTrace() for colo/loc
                                      └─ Results streaming via job.Phase1Results
                                           │
                              ┌───────────┴──────────┐
                              ▼                      ▼
                        SkipPhase2=true         SkipPhase2=false
                              │                      │
                         DONE                 Phase 2: xray validation
                                                   └─ sortCleanIPResults()
                                                   └─ Take top N candidates
                                                   └─ validateWithXrayAttempts (2 attempts)
                                                   └─ Each: xray config → SOCKS5 → HTTP GET
                                                   └─ median latency, best, jitter
                                                        │
                                                   GenerateExport()
                                                   └─ VLESS share URLs or raw IP:port list
```

### IP Replacer Flow

```
Subscription URL ──>  FetchSubscription()
                        └─ URL scheme validation → HTTP GET → 10 MB limit
                        └─ base64 decode → ParseSubscription()
                             │
                        DeduplicateConfigs()
                        └─ Fingerprint: protocol, UUID, encryption, security,
                           SNI, fingerprint, network, host, path, flow,
                           publicKey, shortId, spiderX, allowInsecure,
                           ALPN, headerType, mode, serviceName
                        └─ Returns unique config templates
                             │
                        User selects configs + pastes endpoints
                             │
                        GenerateReplacedConfigs(configs, endpoints)
                        └─ Cross product: every config × every endpoint
                        └─ Skips duplicates (same fingerprint + same endpoint)
                        └─ Appends " @ endpoint" to remark
                        └─ Returns VLESS share URLs
```

---

## HTTP API Reference

All endpoints serve from `127.0.0.1:{random_port}`. The API is designed for
the embedded UI — no authentication.

### Endpoint Scanner

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Serves the embedded Svelte SPA (`ui/dist/index.html` + assets) |
| `/api/scan` | POST | Start WARP endpoint scan |
| `/api/status/{id}` | GET | `{status, progress, total}` |
| `/api/results/{id}` | GET | `{entries[], raw[], failures[], status}` |
| `/api/stop/{id}` | POST | Cancel a running scan |
| `/api/apply-endpoint` | POST | Apply endpoint to uploaded .conf files |

**`/api/scan` request** (multipart/form-data):

| Field | Type | Description |
|-------|------|-------------|
| `config` | file | Warp .conf file (required when useConfig=true) |
| `params` | JSON string | `{noise, noiseConfig, ipv4, ipv6, count, outCount, concurrency, attempts}` |

**`/api/results/{id}` response**:

```json
{
  "entries": [{"endpoint":"162.159.192.1:2408","latency":"142ms","attempts":2,"passes":2,"best":"138ms","jitter":"8ms"}],
  "raw": [{"endpoint":"...","latency":"...","attempts":2,"passes":2,"best":"...","jitter":"..."}],
  "failures": [{"endpoint":"...","error":"tcp dial: timeout"}],
  "failed_count": 45,
  "total": 12,
  "scanned": 100,
  "status": "done"
}
```

### Clean IP Scanner

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/clean-scan` | POST | Start clean IP scan |
| `/api/clean-status/{id}` | GET | `{status, phase1_progress, phase1_total, phase2_progress, phase2_total}` |
| `/api/clean-results/{id}` | GET | `{entries[], raw[], nearby_entries[], status, phase}` |
| `/api/clean-stop/{id}` | POST | Stop a running clean scan |
| `/api/clean-export` | POST | Generate share URLs from VLESS URL + endpoints |

**`/api/clean-scan` request** (JSON):

```json
{
  "vless_url": "vless://...",
  "count": 500,
  "ipv4": true,
  "ipv6": true,
  "phase2_count": 20,
  "one_phase": false,
  "nearby_scan": true,
  "nearby_count": 10,
  "phase1_probes": 500,
  "phase2_probes": 12,
  "ports": [443, 8443, 2053]
}
```

**`/api/clean-results/{id}` response** (phase2):

```json
{
  "entries": [{"endpoint":"1.2.3.4:443","latency":"85ms","attempts":2,"passes":2,"best":"82ms","jitter":"6ms","colo":"FRA","loc":"DE"}],
  "raw": [...],
  "nearby_entries": [...],
  "total": 8,
  "scanned": 20,
  "status": "done",
  "phase": "phase2"
}
```

### IP Replacer

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/replacer/fetch` | POST | Fetch + deduplicate configs from subscription URL |
| `/api/replacer/parse` | POST | Parse raw text into configs |
| `/api/replacer/apply` | POST | Generate replaced configs from selected + endpoints |

### Security headers (applied to all responses)

```http
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
Content-Security-Policy: default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:
```

---

## Config Parsing

### WireGuard (.conf)

Handles both standard Warp and Hogwarts-style formats.

**Standard:**

```ini
[Interface]
PrivateKey = ...
Address = 2606:4700:110:.../128
Reserved = 0,0,0
MTU = 1280

[Peer]
PublicKey = bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
Endpoint = 162.159.192.1:2408
```

**Hogwarts-style (extra fields):**

```ini
[Interface]
PrivateKey = ...
Address = ...
S1 = 0
S2 = 0
S3 = 0
Jc = ...   ; ignored (passthrough)
Jmin = ... ; ignored

[Peer]
PublicKey = ...
Endpoint = ...
```

The parser lowercases all keys. S1/S2/S3 fill Reserved bytes when the
`Reserved` field is absent. Unknown fields pass through silently.

### VLESS / Trojan share URL

Parsed with Go's `url.Parse`. Supports IPv4, IPv6 (bracketed), all standard
query parameters: `security`, `sni`, `fp`, `type`, `host`, `path`, `flow`,
`pbk`, `sid`, `spx`, `allowInsecure`, `alpn`, `headerType`, `mode`,
`serviceName`, `packetEncoding`.

---

## UDP Noise

When enabled, xray-core sends random-size UDP packets before each WireGuard
handshake with random delays. This evades DPI-based blocking of standard WARP
traffic.

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| Type | `rand` | rand / base64 / hex / str | Noise packet content type |
| Packet | `50-100` | 1-1500 bytes | Packet size or value range |
| Delay | `1-5` | 1-1000 ms | Delay between noise packets |
| Count | 5 | 1-50 | Number of noise packets per handshake |

---

## Frontend (Svelte UI)

The UI is a Vite + Svelte 5 single-page app under `frontend/`. The committed
`ui/dist/` bundle is what `go build` embeds, so **a UI change is a two-step
rebuild**: rebuild `ui/dist/`, then rebuild the Go binary.

```bash
cd frontend
npm install        # first time only
npm run dev        # hot-reload dev server
npm run build      # regenerates ../ui/dist — commit this with your change
cd ..
go build -ldflags="-s -w -X 'main.Version=dev'" -o Cloudflare-Scanner .
```

Layout:

- `src/components/` — one `*.svelte` file per tab (`EndpointScanner`,
  `IpScanner`, `Replacer`, `About`) plus shared widgets (`VirtualTable`,
  `SplitCopyButton`, `QrModal`, ...).
- `src/lib/` — shared helpers: `api.js` (fetch wrapper), `sse.js` (status
  streaming), `stores.js` (persisted settings/results), `handoff.js`
  (cross-tab "use this endpoint" handoff), `sort.js`, `copymode.js`, etc.
- `src/locales/en.json` + `fa.json` — UI strings via `svelte-i18n`, keyed
  identically. Update **both** files for any user-facing string change.

For `npm run dev`, also run the Go server (`go run .`) and note the random
port it prints, then point Vite's dev proxy at it so `/api/*` calls resolve:

```bash
VITE_API_TARGET=http://127.0.0.1:<port> npm run dev   # see frontend/vite.config.js
```

## Testing

```bash
go vet ./...
go test ./...       # parser, security, generation tests
go test -race ./... # optional — slower, catches data races
```

Tests cover:

- WARP config parsing (valid, missing fields, defaults)
- VLESS/Trojan URL parsing (standard, IPv6, allowInsecure variants)
- Config deduplication and cross-product generation
- Path traversal guard logic
- Clean IP CIDR generation and weight distribution

## CI/CD

### CI — Go (`ci.yml`)

Runs on every push/PR to `master`. Matrix: 6 platform/arch combos (windows/linux/darwin × amd64/arm64).

```yaml
steps:
  - go vet ./...
  - go test ./...          # linux/amd64 only
  - go build -o /dev/null . # all platforms
```

### CI — Frontend (`frontend.yml`)

Runs only when `frontend/**` or `ui/dist/**` changes (paths-filtered, so
Go-only pushes skip it). Rebuilds the Svelte bundle and verifies `ui/dist/`
matches the committed artifact:

```yaml
steps:
  - npm ci && npm run build
  - git diff --quiet -- ui/dist  # fails if ui/dist is stale
```

### Release (`release.yml`)

Triggered by `git tag v*` or manually via GitHub UI. Builds 7 targets,
downloads matching xray-core v1.8.24, creates `.zip`/`.tar.gz` archives,
generates checksums, and publishes to GitHub Releases.

---

## Environment Variables

None required. Everything is configured through the web UI at runtime.

---

## Platform Notes

### Linux

- Browser opens via `xdg-open` — install `xdg-utils` if missing.
- `chmod +x xray` required after extraction.

### Termux / Android

- The release bundles the Android xray-core build.
- Browser opens via `termux-open-url` — install with `pkg install termux-open-url`.
- Script: `scripts/termux-setup.sh` for one-liner install.

### macOS

- Gatekeeper may block `xray` — run `xattr -dr com.apple.quarantine xray`.
- If the app itself is blocked: System Settings → Privacy & Security → Open Anyway.

### Windows

- Antivirus may flag `xray.exe` — add exclusion for the extracted folder.
- Run as Administrator if the app fails to start on locked-down systems.

---

## Contributing

1. Fork the repo
2. Create a feature branch (`git checkout -b my-feature`)
3. Make changes, run `go vet ./...` and `go test ./...`
4. Build and test locally: `go build -o Cloudflare-Scanner.exe .`
5. Commit with a clear message
6. Push and open a PR

---

## License

MIT — free to use, modify, and distribute.
