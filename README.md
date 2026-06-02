# Cloudflare Scanner

**Cloudflare Warp endpoint + Clean IP scanner** — a cross-platform desktop tool (Windows, Linux, macOS, Termux/Android) with a web GUI. Three tools in one:

- **Endpoint Scanner** — finds working Warp WireGuard endpoints (bypasses DPI-based ISP blocking with UDP noise)
- **IP Scanner** — scans Cloudflare IP ranges for clean proxies using a VLESS/Trojan URL
- **IP Replacer** — replaces IP:port in subscription configs with discovered clean endpoints

> [نسخه فارسی](README.fa.md)

---

## Quick Start

1. Download the [latest release](https://github.com/QMahyar/Cloudflare-Scanner/releases) for your platform
2. Extract and run:

   | Platform | Run command |
   |---|---|
   | Windows | Extract `.tar.gz`, double-click `Cloudflare-Scanner.exe` |
   | Linux / macOS | `tar -xzf *.tar.gz && chmod +x Cloudflare-Scanner xray && ./Cloudflare-Scanner` |

   For **Termux**, one-liner install:

   ```bash
   curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
   ```
   Then type `scan` to run.

3. A browser tab opens at `http://127.0.0.1:XXXXX`
4. Close the terminal or press Ctrl+C to stop

xray-core v1.8.24 is bundled — no extra downloads.

> **New user?** See the [full getting started guide](docs/getting-started.md) for platform-specific tips, distro setup, and first-use workflow.

---

## Documentation

Detailed step-by-step guides for every feature:

| Guide | What it covers |
|---|---|
| [Getting Started](docs/getting-started.md) | Download, extract, run, first-use workflow on any platform |
| [Endpoint Scanner](docs/endpoint-scanner.md) | Find working Warp endpoints with your WireGuard config |
| [IP Scanner (Clean IP)](docs/ip-scanner.md) | Find clean Cloudflare proxy IPs with VLESS validation |
| [IP Replacer](docs/ip-replacer.md) | Replace IP:port in configs with clean endpoints |
| [FAQ](docs/faq.md) | Frequently asked questions and troubleshooting |

> [مستندات فارسی](docs/fa/index.md)

---

## Features

- **Three integrated tools** — Endpoint Scanner, IP Scanner, IP Replacer
- **UDP noise** — random padding + delays to evade DPI-based Warp blocking
- **Live results** — endpoints appear as they pass each test
- **Clean IP scanning** — CIDR-based IP generation from official Cloudflare ranges (25 IPv4 + 91 IPv6)
- **Subscription support** — fetch, deduplicate, and batch-replace IP:port in VLESS/Trojan configs
- **Batch apply** — pick a working endpoint and apply it to multiple config files at once
- **Bilingual web GUI** — English and Persian/Farsi with instant switching
- **Self-contained** — bundles xray-core, nothing to install
- **Cancellable** — stop, resume, or reset scans at any time

## Getting Warp Configs

The app includes a curated help section with links to online generators, Telegram bots/channels, CLI tools, and client apps. Scroll down on the **Endpoint Scanner** tab.

## Build from Source

Requires **Go 1.26+** (no C compiler). See [BUILD.md](BUILD.md) for cross-platform build commands, project structure, and architecture.

## License

MIT
