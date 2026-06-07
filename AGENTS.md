# Cloudflare Scanner — AGENTS.md

## Build & Test

```powershell
go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .
go vet ./...
go test ./...
```

Cross-compile: set `$env:GOOS` / `$env:GOARCH` (linux, darwin, windows × amd64, arm64). CI matrix covers all 6 + termux-arm64.

## Key Architecture

- **Single binary + embedded UI**: `//go:embed ui` in `server.go` compiles `ui/index.html` into the binary. No runtime files.
- **Requires `xray.exe`/`xray` co-located** — app exits with download link if missing.
- **Entrypoint**: `main.go` → `startServer(xrayPath)` (random `127.0.0.1` port) → auto-open browser → `select{}` (no graceful shutdown).
- **No HTTP router**: plain `http.ServeMux`; job IDs sliced out of the path (`r.URL.Path[len(prefix):]`).
- **Two xray temp dirs** (both hold one config.json + log per probe, `os.RemoveAll`'d on defer):
  - WARP endpoint scan → `<appdir>/_xray_work/<tag>/` (gitignored).
  - Clean-IP scan → `os.TempDir()/_xray_clean/<tag>/` (NOT under the repo).

### Source file map (flat `package main`)

| File | Responsibility |
|------|----------------|
| `main.go` | Entry, xray presence check, browser launch (Termux/win/mac/linux) |
| `server.go` | `http.ServeMux`, all `/api/*` handlers, in-memory job maps + 10-min TTL cleanup, embedded UI, output-dir path-traversal guard |
| `scanner.go` | WARP endpoint testing: per-endpoint xray WG outbound → SOCKS5 → `GET /generate_204` (or TCP-only when no `.conf`) |
| `xray.go` | WARP xray config gen (`wireguard` outbound), process start/stop, port wait |
| `endpoint.go` | WARP endpoint generator (curated `188.114.*`/`162.159.*` prefixes + WARP UDP ports) |
| `config.go` | Parse WARP `.conf` (`[Interface]`/`[Peer]`, `S1/S2/S3` → `Reserved`) |
| `cleanip.go` | Clean-IP two-phase scan, CF CIDR pools (`cfIPv4CIDRs`/`cfIPv6CIDRs`), `CFCDNPorts`, weighted IP gen, nearby expansion, `/cdn-cgi/trace` colo probe |
| `iprange.go` | Custom-range scanning: `ParseIPRanges` (CIDR/dash/short/single, v4+v6 via `math/big`) + `GenerateFromRanges` (smart enumerate-vs-sample). Used when the IP Scanner is given custom ranges instead of the CF pool |
| `proxy.go` | `ProxyConfig`; parse vless/trojan/vmess URLs; build xray outbound + stream settings (tls/reality/xtls; ws/grpc/kcp/httpupgrade/raw); `GenerateShareURL` |
| `replacer.go` | Subscription fetch (http/https only), base64 sub decode, parse raw configs, fingerprint dedupe, IP×config replacement |
| `noise.go` | UDP noise config + validation (rand/base64/hex/str, count 1–50) |
| `metrics.go` | median/best/jitter over attempts; result sorting (success first, then latency) |
| `parsers_test.go`, `cleanip_test.go` | Parsing + IP-generation tests (network/xray paths untested) |

## Module & Conventions

- `go.mod` module = `WarpEndpointScanner`, GitHub repo = `Cloudflare-Scanner`.
- LDFLAGS `-s -w` strips debug info.
- Hogwarts-style WireGuard configs (S1/S2/S3, Jc, Jmin, H1-H4, I1-I2) are community-specific.
- No env vars — all config through web UI.
- Bilingual docs: `README.fa.md`, `docs/fa/`.

## CI & Releases

- **CI** (`.github/workflows/ci.yml`): matrix of 3 OS × 2 arch, each runs `go vet ./...` → `go build`. `go test ./...` runs on the **linux/amd64** cell only.
- **Release** (`.github/workflows/release.yml`): auto-triggered on `v*` tag. Builds 7 platforms, bundles matching xray-core v1.8.24, uploads `.tar.gz` to GitHub Release.
- Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`

## UI (vanilla HTML/CSS/JS)

- Single `ui/index.html` with inline `<style>` and `<script>`.
- All i18n in `TR` object (`en` + `fa`), switched via `setLang()`.
- API calls through `apiJSON()` wrapper.
- Scan polling: `setInterval` at 300-1500ms intervals.
