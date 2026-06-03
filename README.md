# Cloudflare Scanner

**Three-in-one cross-platform tool** — scan Warp endpoints, find clean Cloudflare proxy IPs, and replace IP:port in subscription configs.

[![Latest Release](https://img.shields.io/github/v/release/QMahyar/Cloudflare-Scanner?label=version&style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/QMahyar/Cloudflare-Scanner/total?style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases)
[![License](https://img.shields.io/github/license/QMahyar/Cloudflare-Scanner?style=flat-square)](LICENSE)

> ## 🌐 Persian / فارسی
>
> [**مشاهده نسخه فارسی README.fa.md**](README.fa.md) — راهنمای کامل به زبان فارسی.
>
> [همه مستندات فارسی](docs/fa/index.md)

---

## Download

| Platform | Arch | Download |
|----------|------|----------|
| 🪟 Windows | amd64 | [`Cloudflare-Scanner-*-windows-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🪟 Windows | arm64 | [`Cloudflare-Scanner-*-windows-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 Linux | amd64 | [`Cloudflare-Scanner-*-linux-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 Linux | arm64 | [`Cloudflare-Scanner-*-linux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 macOS (Intel) | amd64 | [`Cloudflare-Scanner-*-darwin-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 macOS (Apple Silicon) | arm64 | [`Cloudflare-Scanner-*-darwin-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 📱 Termux / Android | arm64 | [`Cloudflare-Scanner-*-termux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |

Each archive includes the app binary + **xray-core v1.8.24** — nothing else to download.

---

## Quick Start

| Platform | Command |
|----------|---------|
| **Windows** | Extract `.tar.gz`, double-click `Cloudflare-Scanner.exe` |
| **Linux / macOS** | `tar -xzf *.tar.gz && chmod +x Cloudflare-Scanner xray && ./Cloudflare-Scanner` |
| **Termux** | `curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh \| sh` then `scan` |

1. A browser tab opens at `http://127.0.0.1:XXXXX`
2. Upload a Warp `.conf` → click **Start Scan** → pick the fastest endpoint
3. Press **Ctrl+C** in the terminal to stop

---

## Platform Guides

<details>
<summary><strong>Windows</strong> — extract, run, firewall</summary>

- **Extract**: Right-click `.tar.gz` → **Extract All** (7-Zip) or `tar.exe -xzf Cloudflare-Scanner-*-windows-amd64.tar.gz`
- **Run**: Double-click `Cloudflare-Scanner.exe`
- **Firewall**: Click **Allow** if prompted
- **Troubleshooting**: Antivirus may flag `xray.exe` — add exclusion. Run as Admin if port is blocked.
</details>

<details>
<summary><strong>Linux</strong> — dependencies, extract, run</summary>

```bash
# Dependencies
sudo apt install xdg-utils tar   # Debian/Ubuntu
sudo dnf install xdg-utils tar   # Fedora
sudo pacman -S xdg-utils tar     # Arch

# Extract & run
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
cd Cloudflare-Scanner-*-linux-amd64
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

**ARM/ARM64** (Raspberry Pi, etc.): use `*-linux-arm64.tar.gz`.
</details>

<details>
<summary><strong>macOS</strong> — extract, run, Gatekeeper bypass</summary>

```bash
tar -xzf Cloudflare-Scanner-*-darwin-amd64.tar.gz
cd Cloudflare-Scanner-*-darwin-amd64
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

**Apple Silicon**: use `*-darwin-arm64.tar.gz`.

**Gatekeeper**: if blocked, go to **System Settings → Privacy & Security → Open Anyway** or run:
```bash
xattr -d com.apple.quarantine xray
```
</details>

<details>
<summary><strong>Termux / Android</strong> — one-liner install</summary>

```bash
curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
scan
```

- No extra packages needed
- `scan` alias persists across sessions
- Update: re-run the one-liner
- Remove: `rm -rf ~/cloudflare-scanner && sed -i '/alias scan=/d' ~/.bashrc`
</details>

---

## The Three Tools

| Tool | What it does | When to use |
|------|-------------|-------------|
| **Endpoint Scanner** | Tests Warp WireGuard endpoints using your `.conf` | You need a working Warp endpoint |
| **IP Scanner (Clean IP)** | Scans Cloudflare IP ranges for clean proxies with VLESS/Trojan validation | You need clean IPs for v2ray, Nekobox, Sing-box |
| **IP Replacer** | Replaces IP:port in subscription configs with discovered endpoints | You have stale configs and fresh IPs to inject |

See [workflow](#workflow-1-to-100) below for the full walkthrough.

---

## Workflow: 1 to 100

### Phase 1: First Scan
1. Get a Warp `.conf` → see [Getting Warp Configs](#getting-warp-configs)
2. Open the app (browser at `http://127.0.0.1:XXXXX`)
3. Go to **Endpoint Scanner** tab → upload `.conf` → **Start Scan**
4. Pick the fastest result

### Phase 2: Apply Your Endpoint
5. Click an endpoint → it fills into **Endpoint to apply**
6. Select one or more `.conf` files → click **Generate Configs**
7. Use the modified config with your Warp client

### Phase 3: Clean IP Scanning
8. Go to **IP Scanner** → paste `vless://` or `trojan://` URL → **Start Clean Scan**
9. Phase 1 (TCP probe) → Phase 2 (xray validation) → export working IPs

### Phase 4: Replace IPs in Configs
10. Go to **IP Replacer** → paste subscription URL or raw configs → **Fetch/Parse**
11. Select configs → paste endpoints → **Generate Configs** → Copy All / Download

### Phase 5: Master
12. Combine all three tools in a loop. Tune scan depths, UDP noise, batch-apply.

> **New user?** Phases 1–2 are all you need for a working Warp endpoint.

---

## Features

- **Three integrated tools** — Endpoint Scanner, IP Scanner, IP Replacer
- **UDP noise** — random padding + delays to evade DPI-based Warp blocking
- **Live results** — endpoints appear as they pass each test
- **Clean IP scanning** — CIDR-based IP generation from 25 IPv4 + 91 IPv6 Cloudflare ranges
- **Subscription support** — fetch, deduplicate, batch-replace IP:port in VLESS/Trojan configs
- **Batch apply** — apply one endpoint to multiple `.conf` files at once
- **Mobile-friendly UI** — responsive layout, touch targets, works on phone browsers
- **Bilingual web GUI** — English and Persian/Farsi with instant switching
- **Folder picker** — native OS folder dialog for output directory (Chromium)
- **Self-contained** — bundles xray-core v1.8.24, nothing to install
- **Cancellable** — stop, resume, or reset scans at any time

---

## Changelog

### v1.8.0 (current)
- Mobile-responsive UI — tabs, tables, buttons scale to 360px+ screens
- Browse button for output folder (native `showDirectoryPicker` on Chromium)
- Per-config output cards with copy button, QR code, and selectable textarea
- Touch targets ≥44px, `overflow-x: auto` on wide tables, stacked flex on narrow viewports
- Persian RTL mobile layout fixes

[Full changelog →](https://github.com/QMahyar/Cloudflare-Scanner/releases)

---

## Getting Warp Configs

The app includes a curated help section with links to online generators, Telegram bots/channels, CLI tools, and client apps. Scroll down on the **Endpoint Scanner** tab.

---

## Build from Source

Requires **Go 1.26+** (no C compiler). See [BUILD.md](BUILD.md) for cross-platform build commands, project structure, and architecture.

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | First-time setup and workflow |
| [Endpoint Scanner](docs/endpoint-scanner.md) | Full walkthrough of Warp endpoint scanning |
| [IP Scanner](docs/ip-scanner.md) | Two-phase clean IP scanning |
| [IP Replacer](docs/ip-replacer.md) | Batch IP replacement in configs |
| [FAQ](docs/faq.md) | Troubleshooting & common questions |
| [BUILD.md](BUILD.md) | Build from source, architecture, API |

Persian versions: [`docs/fa/`](docs/fa/index.md)

---

## License

MIT
