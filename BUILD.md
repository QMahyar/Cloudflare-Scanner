# Building & Technical Guide

> [نسخه فارسی](BUILD.fa.md)

## Prerequisites

- **Go 1.26+** (download from [go.dev](https://go.dev/dl/))
- **No C compiler needed** — all dependencies are pure Go
- Windows 10/11 (the app only targets Windows)

## Build commands

```powershell
# Standard build (debug symbols included, larger binary)
go build -o Cloudflare-Scanner.exe .

# Release build (stripped, smaller binary)
go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .

# 32-bit build
$env:GOARCH="386"; go build -ldflags="-s -w" -o Cloudflare-Scanner-x86.exe .
```

The binary embeds `ui/index.html` via Go's `//go:embed` directive — no external files needed at runtime.

## Project structure

```
Cloudflare-Scanner/
├── main.go          # Entry point, checks for xray.exe, starts HTTP server
├── server.go        # HTTP handlers (scan, status, results, stop, apply-endpoint)
├── config.go        # WireGuard .conf parser (case-insensitive, Hogwarts-style support)
├── endpoint.go      # Random Cloudflare IPv4/IPv6 endpoint generator
├── scanner.go       # Parallel endpoint tester (12 workers, SOCKS5 handshake)
├── xray.go          # xray-core config JSON generator and process manager
├── noise.go         # UDP noise config struct and validation
├── ui/
│   └── index.html   # Web UI (embedded in binary)
├── xray.exe         # xray-core v1.8.24 (bundled)
├── sample.conf      # Example WireGuard config for testing
├── README.md        # English documentation
├── README.fa.md     # Persian/Farsi documentation
└── BUILD.md         # This file
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

xray-core v1.8.24 (`xray.exe`) is bundled in the repo and included in every release zip. It must be in the same directory as `Cloudflare-Scanner.exe`.

## Environment variables

None required. Everything is configured through the web UI at runtime.

## Contributing

1. Fork the repo
2. Create a feature branch (`git checkout -b my-thing`)
3. Make changes, run `go vet ./...` to check for issues
4. Build and test: `go build -o Cloudflare-Scanner.exe . && .\Cloudflare-Scanner.exe`
5. Commit with a clear message
6. Push and open a PR

## License

MIT
