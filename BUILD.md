# Building & Technical Guide

> [نسخه فارسی](BUILD.fa.md)

This document is for **developers** who want to build from source, understand the
architecture, or extend the app. For user installation instructions see [README.md](README.md).

---

## Table of Contents

- [Quick Build](#quick-build)
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

The binary embeds `ui/index.html` via Go's `//go:embed` directive — no external files needed at runtime.

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
├── server.go         # HTTP handlers, scan jobs, API, embedding
├── config.go         # WireGuard .conf parser (standard + Hogwarts formats)
├── endpoint.go       # Random WARP endpoint generator (known CF prefixes/ports)
├── scanner.go        # Endpoint scanner — parallel xray-based WARP validation
├── xray.go           # xray-core config builder (WireGuard outbound, SOCKS5 inbound)
├── cleanip.go        # Clean IP scanner — CIDR generation, TCP probe, xray validate, nearby scan
├── replacer.go       # Subscription fetch, config dedup, bulk IP replacement
├── proxy.go          # VLESS/Trojan URL parser, xray JSON builder, share URL generator
├── metrics.go        # Scan quality helpers (median, best, jitter, success-first sort)
├── noise.go          # UDP noise config and validation
├── parsers_test.go   # Unit tests for parsing, replacer, path traversal
├── cleanip_test.go   # Tests for clean IP generation
├── ui/
│   └── index.html    # Single-page web UI (3 tabs, bilingual, no external deps)
├── scripts/          # One-liner install scripts per platform
├── docs/             # User guides (English + Farsi)
├── README.md         # User-facing English documentation
├── README.fa.md      # User-facing Farsi documentation
├── BUILD.md          # This file
├── BUILD.fa.md       # Farsi developer guide
├── .github/
│   └── workflows/
│       ├── ci.yml    # CI: vet + test + build (all platforms)
│       └── release.yml  # Release: build 7 platforms, bundle xray, publish
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
| `/` | GET | Serves embedded `ui/index.html` |
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

### CI (`.github/workflows/ci.yml`)

Runs on push/PR to `master`. Matrix: 6 platform/arch combos.

```yaml
steps:
  - go vet ./...
  - go test ./...          # linux/amd64 only
  - go build -o /dev/null . # all platforms
```

### Release (`.github/workflows/release.yml`)

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
