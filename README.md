# Warp Endpoint Scanner

A Windows GUI tool that scans Cloudflare Warp endpoints to find fast, working ones. Uses [xray-core](https://github.com/XTLS/Xray-core) for UDP noise support — useful when your ISP blocks standard WireGuard Warp traffic.

## Features

- **Scans Cloudflare Warp IPs** — generates random IPv4/IPv6 endpoints and tests each one
- **UDP noise support** — random padding + delays to evade DPI-based blocking
- **Batch apply** — takes a working endpoint and applies it to multiple config files at once
- **Live results** — see endpoints appear in real time as they pass the handshake check
- **Web GUI** — clean browser interface, no separate UI framework needed
- **Self-contained** — single `.exe`, auto-downloads xray-core if missing

## How it works

1. Pick your WireGuard `.conf` file (standard Warp or Hogwarts-style with extra fields)
2. Configure scan depth, IP version, and noise settings
3. Click Start — the server tests endpoints in parallel using xray-core's WireGuard implementation
4. Apply a working endpoint to your configs with one click

## Getting Warp Configs

See the **Getting Warp Configs** section inside the app for a curated list of:
- Online generators (warp-generator.github.io, lanrat.github.io, warp-mirrors.vercel.app, and more)
- Telegram bots (@warp_generator_bot, @WarpGenerator_bot, and channels)
- CLI tools (ViRb3/wgcf, bash-warp-generator, etc.)
- Client apps (Amnezia VPN, WireSock, WG Tunnel, 1.1.1.1, NekoBox)

## Building

```powershell
go build -ldflags="-s -w" -o WarpEndpointScanner.exe .
```

Requires Go 1.26+. No C compiler needed.

## Usage

Run `WarpEndpointScanner.exe`. It opens your browser to `http://127.0.0.1:<port>`. Close the terminal window to stop the server.

## License

MIT
