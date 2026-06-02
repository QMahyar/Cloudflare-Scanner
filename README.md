# Cloudflare Scanner

**Cloudflare Warp endpoint + Clean IP scanner** — a cross-platform desktop tool (Windows, Linux, macOS) with a web GUI. Three tools in one: finds working Warp WireGuard endpoints when your ISP blocks standard Warp traffic, scans Cloudflare IP ranges for clean proxies using a VLESS URL, and replaces IP:port in subscription configs with discovered clean endpoints.

> [نسخه فارسی](README.fa.md)

## Quick Start

1. Download the latest `Cloudflare-Scanner-*.zip` from [releases](https://github.com/QMahyar/Cloudflare-Scanner/releases)
2. Extract both files, run `Cloudflare-Scanner.exe`
3. A browser tab opens at `http://127.0.0.1:XXXXX`
4. Close the terminal window to stop the server

xray-core v1.8.24 is bundled — no extra downloads needed.

---

## Tabs

### Endpoint Scanner

Scans Cloudflare's Warp IP range to find endpoints where your WireGuard config works. Uses xray-core behind a SOCKS5 proxy, with optional UDP noise to bypass DPI-based ISP blocking.

**How to use:**
1. Select a WireGuard `.conf` file (standard Warp or Hogwarts-style)
2. Set scan depth and IP version (IPv4 / IPv6 / both)
3. Toggle UDP Noise if your ISP blocks Warp
4. Click **Start Scan** — results appear live
5. Click an endpoint to copy it, then use **Apply Endpoint** to batch-update config files
6. Modified configs are saved to your chosen output folder

### IP Scanner

Finds Cloudflare IPs that respond to TCP (Phase 1) and optionally validates them against your VLESS URL through xray-core (Phase 2). Useful for finding clean Cloudflare proxy IPs for tools like v2ray, Nekobox, Sing-box, etc.

**How to use:**
1. Paste a VLESS share URL (for Phase 2 validation) or leave empty for Phase 1 only
2. Toggle **1-phase mode** to skip xray validation and only TCP-probe
3. Set scan depth and Phase 2 probe count
4. Click **Start Clean Scan**
5. **Phase 1** — TCP probes against generated Cloudflare IPs, results update live
6. **Phase 2** — best Phase 1 results are validated through xray-core with your VLESS config
7. Export working IPs as VLESS share URLs or download raw IP list

### IP Replacer

Fetches a subscription link (like `https://example.com/sub?token=...`), deduplicates configs that differ only by IP:port, then generates all combinations of your unique configs with a list of clean endpoints. Perfect for refreshing a stale subscription with fresh IPs from the IP Scanner.

**How to use:**
1. Enter a subscription URL and click **Fetch**
2. Unique configs (with IP:port removed from identity) are shown with checkboxes
3. Keep, select, or deselect configs to use
4. Paste endpoints from Clean IP scan results (or any IP:port list, one per line)
5. Click **Generate Configs** — produces every combination
6. Copy all to clipboard or download as a text file

---

## Features

- **Three integrated tools** — Endpoint Scanner, IP Scanner, IP Replacer — in one app
- **UDP noise** — random padding + delays to evade DPI-based Warp blocking
- **Live results** — endpoints appear in real time as they pass each test
- **Clean IP scanning** — CIDR-based Cloudflare IP generation from official ranges (25 IPv4 + 91 IPv6 CIDRs)
- **Subscription support** — fetch, deduplicate, and batch-replace IP:port in VLESS configs
- **Batch apply** — pick a working endpoint and apply it to multiple config files at once
- **Web GUI** — clean browser interface served by the app itself, no extra dependencies
- **Self-contained** — single `.exe` includes xray-core, no setup
- **Cancellable** — stop, resume, or reset scans at any time
- **Bilingual UI** — English and Persian/Farsi with instant switching

## Getting Warp Configs

The app includes a curated help section with links to online generators, Telegram bots/channels, CLI tools, and client apps. Or see the [Warp config resources](https://github.com/QMahyar/Cloudflare-Scanner/wiki) wiki.

## Building from source

Requires **Go 1.26+** (no C compiler needed). Cross-platform — builds for Windows, Linux, and macOS:

```bash
git clone https://github.com/QMahyar/Cloudflare-Scanner.git
cd Cloudflare-Scanner

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .
```

See [BUILD.md](BUILD.md) for detailed build guide, cross-platform notes, project structure, and architecture docs.

## License

MIT
