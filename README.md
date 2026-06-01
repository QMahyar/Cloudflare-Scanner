# Cloudflare Scanner

**Cloudflare Warp endpoint + IP scanner** — a Windows desktop tool that scans Cloudflare's IP range to find working Warp endpoints. Uses [xray-core](https://github.com/XTLS/Xray-core) for WireGuard with UDP noise, useful when your ISP blocks standard Warp traffic.

> [نسخه فارسی](README.fa.md)

## Features

- **Endpoint scanning** — generates random Cloudflare IPv4/IPv6 endpoints and tests each one
- **UDP noise** — random padding + delays to evade DPI-based blocking
- **Live results** — working endpoints appear in real time as they pass the handshake
- **Batch apply** — pick a working endpoint and apply it to multiple config files at once
- **Web GUI** — clean browser interface served by the app itself, no extra dependencies
- **Self-contained** — single `.exe`, auto-downloads xray-core on first run
- **Cancellable** — stop, resume, or reset scans at any time

## How it works

1. Pick your WireGuard `.conf` file (standard Warp or Hogwarts-style with extra fields)
2. Set scan depth, IP version (IPv4/IPv6/both), and toggle UDP noise
3. Click **Start Scan** — the server tests endpoints in parallel using xray-core's WireGuard implementation behind a custom SOCKS5 proxy
4. Watch results appear live — each endpoint's latency and status shows immediately
5. Click an endpoint to copy it, then use **Apply Endpoint** to batch-update multiple config files
6. Modified configs are saved to your chosen output folder

## Getting configs

The app includes a curated help section with links to online generators, Telegram bots/channels, CLI tools, and client apps. Or see the [Warp config resources](https://github.com/QMahyar/Cloudflare-Scanner/wiki) wiki.

## Building from source

```powershell
go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .
```

Requires Go 1.26+. No C compiler needed.

## Usage

Run `Cloudflare-Scanner.exe`. It opens your browser to `http://127.0.0.1:<port>`. Close the terminal window to stop the server.

## License

MIT
