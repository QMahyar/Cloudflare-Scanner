# Building & Technical Guide

> [نسخه فارسی](BUILD.fa.md)

This document covers the complete project structure, architecture, build commands, and API reference for all three tools: **Endpoint Scanner** (Warp), **IP Scanner** (Clean IP), and **IP Replacer** (subscription IP replacement). Useful for developers building from source, extending the app, or understanding how the parts fit together.

## Prerequisites

- **Go 1.26+** (download from [go.dev](https://go.dev/dl/))
- **No C compiler needed** — all dependencies are pure Go
- Cross-platform: Windows, Linux, macOS (all architectures)

## Build commands

```bash
# Windows (amd64)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .

# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# Linux (arm64) — e.g. Raspberry Pi
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .
```

On **Windows (PowerShell)** the same commands work with `$env:GOOS="windows"; $env:GOARCH="amd64"; go build ...`.

The binary embeds `ui/index.html` via Go's `//go:embed` directive — no external files needed at runtime.

## Project structure

```
Cloudflare-Scanner/
├── main.go          # Entry point, checks for xray.exe, starts HTTP server
├── server.go        # HTTP handlers (scan, status, results, stop, apply-endpoint, clean scan, replacer)
├── config.go        # WireGuard .conf parser (case-insensitive, Hogwarts-style support)
├── endpoint.go      # Random Cloudflare IPv4/IPv6 endpoint generator
├── scanner.go       # Parallel endpoint tester (12 workers, SOCKS5 handshake)
├── xray.go          # xray-core config JSON generator and process manager
├── noise.go         # UDP noise config struct and validation
├── proxy.go         # VLESS/Trojan share URL parser and xray JSON builder
├── cleanip.go       # Clean IP scanner: CIDR-based IP generator, TCP probe, xray validation, export
├── replacer.go      # Subscription fetcher, deduplicator, cross-product IP replacer
├── cleanip_test.go  # Tests for IP generation, weight calculation, IPv6
├── ui/
│   └── index.html   # Web UI (3 tabs, bilingual i18n, tooltips, embedded in binary)
├── xray.exe         # xray-core v1.8.24 (bundled)
├── sample.conf      # Example WireGuard config for testing
├── README.md        # English documentation
├── README.fa.md     # Persian/Farsi documentation
├── BUILD.md         # This file
└── BUILD.fa.md      # Persian build guide
```

## Architecture

### How scanning works

```
User picks .conf  ──>  ParseWarpConfig() extracts keys/addresses/reserved bytes
                          │
EndpointGenerator        │
  └─ random IP:port      │
     from known CF       │
     prefixes + ports    ▼
                    runScan()  ──>  NewScanner()
                                       │
                          ┌────────────┼──────────────┐
                          ▼            ▼              ▼
                     Worker 1      Worker 2 ...  Worker 12
                          │            │              │
                     XrayManager  XrayManager    XrayManager
                     └─ GenerateConfig(endpoint, port)
                     └─ StartXray() → xray.exe process
                     └─ WaitForPort() (SOCKS5 listener)
                     └─ socks5Handshake() → HTTP GET
                     └─ StopXray() (kill process)
                          │            │              │
                          └────────────┴──────────────┘
                                          │
                                    Sort by latency
                                          │
                                    Return results
```

### Key components

| File | What it does |
|------|-------------|
| `main.go` | Checks for xray.exe next to the binary, starts HTTP server on random port, opens browser |
| `server.go` | API endpoints: `/api/scan`, `/api/status/{id}`, `/api/results/{id}`, `/api/stop/{id}`, `/api/apply-endpoint`. Embeds `ui/` as a filesystem. |
| `config.go` | Parses WireGuard configs. Handles both standard Warp and Hogwarts-style formats. Lowercases all keys, auto-appends `/128` to bare IPv6, derives Reserved bytes from S1/S2/S3 fields. |
| `endpoint.go` | Generates random endpoints from known Cloudflare Warp IP prefixes (14 IPv4 ranges, 4 IPv6 ranges) and 50+ ports. Deduplicates by IP. |
| `scanner.go` | 12 concurrent workers. Each worker: generates xray config → starts xray → waits for SOCKS5 → performs SOCKS5 handshake → HTTP GET to gstatic.com → checks for 204 → records latency. |
| `xray.go` | Builds xray-core JSON config with WireGuard outbound, SOCKS5 inbound, routing rules. Manages process lifecycle (start, wait for port, kill). |
| `noise.go` | UDP noise: random packet size (50-100 bytes) with random delay (1-5ms) sent before each handshake. Evades DPI-based Warp blocking. |
| `proxy.go` | Parses VLESS/Trojan share URLs (`ParseProxyURL`), builds xray JSON configs (`BuildXrayJSON`), generates share URLs from config (`GenerateShareURL`). Always includes `sni` when set. |
| `cleanip.go` | Clean IP scanner. Generates IPs from 25 IPv4 + 91 IPv6 Cloudflare CIDR ranges (`GenerateIPs`). Phase 1: TCP probe with 500 concurrent workers (`runCleanPhase1TCP`). Phase 2: xray validation through SOCKS5 (`validateWithXray`). Config export (`GenerateExport`). Supports SkipPhase2 mode. |
| `replacer.go` | Fetches subscription URLs (`FetchSubscription`), parses VLESS configs (`ParseSubscription`), deduplicates ignoring Address/Port/Remark (`DeduplicateConfigs`), generates all config×endpoint combos (`GenerateReplacedConfigs`). |
| `cleanip_test.go` | Go tests for `GenerateIPs`, weight calculation, and IPv6 generation. All passing. |
| `xray.exe` | xray-core v1.8.24 binary, bundled in the repo and release zip |

### Web UI API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Serves embedded `index.html` |
| `/api/scan` | POST | Upload config + JSON params → returns job ID |
| `/api/status/{id}` | GET | Returns `{status, progress, total}` |
| `/api/results/{id}` | GET | Returns working endpoints (live during scan, full on done) |
| `/api/stop/{id}` | POST | Cancels a running scan |
| `/api/apply-endpoint` | POST | Upload configs + endpoint → saves modified copies |
| `/api/clean-scan` | POST | Start a clean IP scan → returns job ID |
| `/api/clean-status/{id}` | GET | Returns clean scan `{status, phase1Progress, phase1Total, phase2Progress, phase2Total}` |
| `/api/clean-results/{id}` | GET | Returns phase1Results + phase2Results (live during scan, full on done) |
| `/api/clean-stop/{id}` | POST | Stops a running clean scan |
| `/api/clean-export/{id}` | GET | Downloads clean scan results as text file |
| `/api/replacer-fetch` | POST | Fetches and deduplicates configs from a subscription URL |
| `/api/replacer-apply` | POST | Generates replaced configs from selected configs + endpoints |

### Config parsing details

The parser handles two config formats:

**Standard Warp:**
```ini
[Interface]
PrivateKey = ...
Address = 2606:4700:110:8d48:...
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
Jc = ...
Jmin = ...
H1 = ...
H2 = ...
H3 = ...
H4 = ...
I1 = ...
I2 = ...

[Peer]
PublicKey = ...
Endpoint = ...
```

The parser lowercases all keys, ignores unknown fields, falls back to S1/S2/S3 for Reserved bytes if no `Reserved` field is present.

### UDP noise

When enabled, xray-core sends random-size packets before each WireGuard handshake with random delays. This makes the traffic look like noise instead of a WireGuard session, helping bypass ISPs that DPI-block Warp's standard port 2408.

Default: 5 noise packets, each 50-100 bytes random data, 1-5ms apart.

## xray-core

xray-core v1.8.24 is bundled in every release archive. The app looks for `xray.exe` (Windows) or `xray` (Linux/macOS) in the same directory as its own binary. If not found, it prints an error with the download URL.

### Clean IP scanning flow

```
User provides VLESS URL        CleanIPGenerator
       (optional)                  └─ GenerateIPs()
                                      └─ 25 IPv4 CIDRs (5955 /24 subnets)
                                      └─ 91 IPv6 CIDRs (weighted distribution)
                                           │
                                    Phase 1: TCP probe
                                      └─ 500 concurrent workers
                                      └─ net.DialTimeout(ip:port, 2s)
                                      └─ Writes results incrementally to job.Phase1Progress
                                           │
                              ┌───────────┴──────────┐
                              ▼                      ▼
                        SkipPhase2=true         SkipPhase2=false
                              │                      │
                         DONE (status)        Phase 2: xray validation
                                                   └─ Sort Phase 1 by latency
                                                   └─ Take top N (Phase 2 probe count)
                                                   └─ 12 concurrent xray workers
                                                   └─ Each: BuildXrayJSON → StartXray → SOCKS5 → HTTP GET
                                                   └─ Writes results incrementally to job.Phase2Progress
                                                        │
                                                   GenerateExport()
                                                   └─ VLESS share URLs
                                                   └─ Raw IP:port list
```

### IP Replacer flow

```
Subscription URL ──> FetchSubscription()
                        └─ HTTP GET → base64.decode → ParseSubscription()
                             │
                        DeduplicateConfigs()
                        └─ Fingerprint: exclude Address, Port, Remark
                        └─ Returns unique config templates
                             │
                        User selects configs + pastes endpoints
                             │
                        GenerateReplacedConfigs(configs, endpoints)
                        └─ Cross product: every config × every endpoint
                        └─ Skips duplicates (same config + same endpoint)
                        └─ Appends " @ endpoint" to each remark
                        └─ Returns VLESS share URLs
```

## Environment variables

None required. Everything is configured through the web UI at runtime.

## Platform-specific notes

### Linux

After extracting, make `xray` executable: `chmod +x xray`. The app opens the browser via `xdg-open` — install `xdg-utils` if missing:

| Distro | Command |
|---|---|
| Debian / Ubuntu / Mint | `sudo apt install xdg-utils` |
| Fedora / RHEL / CentOS | `sudo dnf install xdg-utils` |
| Arch / Manjaro | `sudo pacman -S xdg-utils` |

### Termux / Android

The `termux-arm64` release archive bundles the Android xray-core build and the same `linux/arm64` app binary.

1. Download `Cloudflare-Scanner-*-termux-arm64.tar.gz` and extract:
   ```bash
   tar -xzf Cloudflare-Scanner-*-termux-arm64.tar.gz
   ```
2. Make both files executable: `chmod +x Cloudflare-Scanner xray`
3. The app auto-detects Termux via `$PREFIX` and opens the browser with `termux-open-url`
4. Run: `./Cloudflare-Scanner`

> Install `termux-open-url` if missing: `pkg install termux-open-url`

### macOS
- After extracting, make `xray` executable: `chmod +x xray`
- The app opens the browser via `open` (built-in)
- If macOS blocks the unsigned binary, run: `xattr -d com.apple.quarantine Cloudflare-Scanner`

## Contributing

1. Fork the repo
2. Create a feature branch (`git checkout -b my-thing`)
3. Make changes, run `go vet ./...` to check for issues
4. Build and test: `go build -o Cloudflare-Scanner.exe . && .\Cloudflare-Scanner.exe`
5. Commit with a clear message
6. Push and open a PR

## License

MIT
