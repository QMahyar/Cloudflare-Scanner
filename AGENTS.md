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
- **Entrypoint**: `main.go` → `startServer(xrayPath)` (random port) → auto-open browser → `select{}`.
- **No HTTP router**: plain `http.ServeMux` path matching.
- **`_xray_work/`** created at runtime for xray temp configs (gitignored).

## Module & Conventions

- `go.mod` module = `WarpEndpointScanner`, GitHub repo = `Cloudflare-Scanner`.
- LDFLAGS `-s -w` strips debug info.
- Hogwarts-style WireGuard configs (S1/S2/S3, Jc, Jmin, H1-H4, I1-I2) are community-specific.
- No env vars — all config through web UI.
- Bilingual docs: `README.fa.md`, `docs/fa/`.

## CI & Releases

- **CI** (`.github/workflows/ci.yml`): `go vet ./...` → `go build` on push/PR to `master`. Tests are NOT run in CI.
- **Release** (`.github/workflows/release.yml`): auto-triggered on `v*` tag. Builds 7 platforms, bundles matching xray-core v1.8.24, uploads `.tar.gz` to GitHub Release.
- Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`

## UI (vanilla HTML/CSS/JS)

- Single `ui/index.html` with inline `<style>` and `<script>`.
- All i18n in `TR` object (`en` + `fa`), switched via `setLang()`.
- API calls through `apiJSON()` wrapper.
- Scan polling: `setInterval` at 300-1500ms intervals.
