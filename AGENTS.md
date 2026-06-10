# Cloudflare Scanner — AGENTS.md

## Build & Test

```powershell
go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .
go vet ./...
go test ./...
```

Cross-compile: set `$env:GOOS` / `$env:GOARCH` (linux, darwin, windows × amd64, arm64). CI matrix covers all 6 + termux-arm64.

## Key Architecture

- **Single binary + embedded UI**: `//go:embed all:ui/dist` in `server.go` compiles the built Svelte bundle (`ui/dist/`) into the binary. No runtime files. Rebuild the UI with `cd frontend && npm run build` before `go build`; `ui/dist/` is committed so `go build` needs no Node.
- **Requires `xray.exe`/`xray` co-located** — app exits with download link if missing.
- **Entrypoint**: `main.go` → `startServer(xrayPath)` (random `127.0.0.1` port) → auto-open browser → `waitForShutdown` (blocks on SIGINT/SIGTERM, then cleans temp dirs and exits).
- **No HTTP router**: plain `http.ServeMux`; job IDs sliced out of the path (`r.URL.Path[len(prefix):]`).
- **Two xray temp dirs** (both under `os.TempDir()`, one config.json + log per probe, `os.RemoveAll`'d on defer):
  - WARP noise-fallback scan → `os.TempDir()/_xray_work/<tag>/`.
  - Clean-IP scan → `os.TempDir()/_xray_clean/<tag>/`.
  - The default native WARP handshake path (`warp_probe.go`) uses no temp dir.

### Source file map (flat `package main`)

| File | Responsibility |
|------|----------------|
| `main.go` | Entry, xray presence check, browser launch (Termux/win/mac/linux) |
| `server.go` | `http.ServeMux`, all `/api/*` handlers, in-memory job maps + 10-min TTL cleanup, embedded UI, output-dir path-traversal guard |
| `scanner.go` | WARP endpoint testing: native WireGuard handshake (`warp_probe.go`) by default; xray WG outbound → SOCKS5 → `GET /generate_204` only when noise is requested; TCP-only when no `.conf` |
| `warp_probe.go` | Native WireGuard (Noise_IKpsk2) handshake-initiation UDP prober — validates WARP endpoints with the uploaded `.conf` creds, no xray process |
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
- Depends on `golang.org/x/crypto` (blake2s + chacha20poly1305) for the native WARP handshake; otherwise stdlib only.
- LDFLAGS `-s -w` strips debug info.
- Hogwarts-style WireGuard configs (S1/S2/S3, Jc, Jmin, H1-H4, I1-I2) are community-specific.
- No env vars — all config through web UI.
- Bilingual docs: `README.fa.md`, `docs/fa/`.

## CI & Releases

- **CI** (`.github/workflows/ci.yml`): matrix of 3 OS × 2 arch, each runs `go vet ./...` → `go build`. `go test ./...` runs on the **linux/amd64** cell only.
- **Release** (`.github/workflows/release.yml`): auto-triggered on `v*` tag. Builds 7 platforms, bundles matching xray-core v1.8.24, uploads `.tar.gz` to GitHub Release.
- Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`

## UI (Vite + Svelte 5)

- Source in `frontend/src/`: one `*.svelte` component per tab in `components/` (`EndpointScanner`, `IpScanner`, `Replacer`, `About`) plus shared helpers in `lib/`.
- All i18n in `frontend/src/locales/en.json` + `fa.json` (identical keys), via `svelte-i18n`; language toggled in `lib/i18n.js`.
- API calls through `lib/api.js` `apiJSON()` wrapper.
- Scan polling: `setInterval` at 300-1500ms intervals.
- Build: `cd frontend && npm run build` → emits `ui/dist/` (committed, embedded by Go). `npm run dev` for a hot-reload dev server.
