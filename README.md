# Cloudflare Scanner

**Cloudflare Warp endpoint + Clean IP scanner** — a cross-platform desktop tool (Windows, Linux, macOS, Termux/Android) with a web GUI. Three tools in one: finds working Warp WireGuard endpoints when your ISP blocks standard Warp traffic, scans Cloudflare IP ranges for clean proxies using a VLESS URL, and replaces IP:port in subscription configs with discovered clean endpoints.

> [نسخه فارسی](README.fa.md)

## Quick Start

1. Download the latest archive for your platform from [releases](https://github.com/QMahyar/Cloudflare-Scanner/releases)
2. Extract, then run:

   | Platform | Extract & Run |
   |---|---|
   | Windows | Extract `.tar.gz` (use 7-Zip or `tar.exe`), double-click `Cloudflare-Scanner.exe` |
   | Linux | `tar -xzf *.tar.gz && chmod +x Cloudflare-Scanner xray && ./Cloudflare-Scanner` |
   | macOS | `tar -xzf *.tar.gz && chmod +x Cloudflare-Scanner xray && ./Cloudflare-Scanner` |
   | Termux | `tar -xzf *-termux-*.tar.gz && chmod +x Cloudflare-Scanner xray && ./Cloudflare-Scanner` |

3. A browser tab opens at `http://127.0.0.1:XXXXX`
4. Close the terminal window to stop the server

xray-core v1.8.24 is bundled in every archive — no extra downloads needed.

---

## How to Use

The app has three tools (tabs). Here's how they work together:

```
Scan Warp Endpoints  ──>  Pick the fastest endpoint
                                   │
Scan Clean IPs       ──>  Find working Cloudflare proxy IPs
                                   │
IP Replacer          ──>  Replace IP:port in configs with clean endpoints
```

### Endpoint Scanner

Finds Cloudflare Warp endpoints where your WireGuard config works. Uses xray-core behind a SOCKS5 proxy, with optional UDP noise to bypass DPI-based ISP blocking.

1. **Select config** — Pick a WireGuard `.conf` file (standard Warp or Hogwarts-style with S1-S4, Jc, Jmin, H1-H4, I1, I2)
2. **Set scan depth** — Quick (100), Normal (500), Deep (1000), Insane (5000), Massive (10000), or custom
3. **Choose IP version** — IPv4 only (default), IPv6 only, or both
4. **Toggle UDP Noise** — Enable if your ISP blocks Warp traffic on port 2408
5. **Start Scan** — Results appear live. Each endpoint is tested through xray-core
6. **Apply endpoint** — Click an endpoint to copy it, then use **Apply Endpoint** to batch-update config files in your output folder

### IP Scanner (Clean IP)

Generates Cloudflare IPs from official CIDR ranges (25 IPv4 + 91 IPv6), TCP-probes them in Phase 1, then optionally validates the fastest ones against your VLESS URL through xray-core in Phase 2.

1. **Enter VLESS URL** — Paste a `vless://...` or `trojan://...` URL for Phase 2 validation. Leave empty for TCP probe only
2. **1-phase mode** — Check to skip xray validation and only TCP-probe. Default port is 443
3. **Set scan depth** — Number of IPs to test
4. **Phase 2 probe count** — How many Phase 1 winners get validated through xray
5. **Start Clean Scan**
6. **Phase 1** (live) — TCP `net.DialTimeout` to each IP:port with 500 concurrent workers. Results stream in real time
7. **Phase 2** — Sorted by latency, top N are validated through xray-core via SOCKS5 with your VLESS config. 12 concurrent workers
8. **Export** — Download working IPs as VLESS share URLs (`GenerateExport`) or raw IP:port list

### IP Replacer

Takes configs (from a subscription URL or pasted directly), deduplicates them by fingerprint (ignoring IP:port and remark), then generates every combination with a list of endpoints.

1. **Choose input method** — Pick **Subscription URL** or **Paste Configs** (mutually exclusive)
   - Subscription URL: Enter `https://example.com/sub?token=...`, click **Fetch**
   - Paste Configs: Paste raw `vless://` and `trojan://` lines, click **Parse**. Non-config lines are ignored
2. **Review unique configs** — Deduplicated templates shown with checkboxes. Select/deselect as needed
3. **Paste endpoints** — One `ip:port` per line, from Clean IP results or anywhere
4. **Generate Configs** — Produces every config × endpoint combination. Remark gets ` @ endpoint` appended
5. **Copy All** or **Download** the output

**Parse behavior:** Configs can be separated by newlines, spaces, commas, semicolons, or pipes — all work. Valid `vless://` and `trojan://` URLs are extracted; everything else is ignored.

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
