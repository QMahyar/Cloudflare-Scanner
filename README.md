# Cloudflare Scanner

> **Find working Cloudflare Warp endpoints and clean proxy IPs — fast, free, no setup.**

[![Latest Release](https://img.shields.io/github/v/release/QMahyar/Cloudflare-Scanner?style=flat-square&label=Download)](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/QMahyar/Cloudflare-Scanner/ci.yml?branch=master&style=flat-square&label=CI)](https://github.com/QMahyar/Cloudflare-Scanner/actions/workflows/ci.yml)
[![Downloads](https://img.shields.io/github/downloads/QMahyar/Cloudflare-Scanner/total?style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases)
[![License](https://img.shields.io/github/license/QMahyar/Cloudflare-Scanner?style=flat-square)](LICENSE)

---

> **فارسی** → [README.fa.md](README.fa.md)

---

## Overview

Cloudflare Scanner is a **cross-platform desktop tool** for finding healthy Cloudflare WARP endpoints and clean proxy IPs, plus batch-applying them to your config files. It bundles [xray-core](https://github.com/XTLS/Xray-core) for realistic validation — no separate proxy setup needed.

**Start here if you're new.** Download, extract, run, and a browser tab opens. Three tabs do the whole job.

### Who needs this?

Anyone who uses **Cloudflare WARP**, **v2ray/v2rayN**, **Nekobox**, **Sing-box**, **Clash**, or any proxy client running on Cloudflare's edge. ISPs frequently block specific IPs and ports. This tool finds the ones that still work from **your** network and ranks them by real-world performance.

---

## Quick Start

### 1. Download

| Platform | Architecture | Download |
|----------|-------------|---------|
| 🪟 Windows | x86-64 | [`windows-amd64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🪟 Windows | ARM64 | [`windows-arm64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 Linux | x86-64 | [`linux-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 Linux | ARM64 / Raspberry Pi | [`linux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 macOS | Intel | [`darwin-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 macOS | Apple Silicon | [`darwin-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 📱 Android (Termux) | ARM64 | [`termux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |

Each archive contains the app **and** xray-core — nothing extra to install.

### 2. Extract and Run

<details>
<summary><strong>Windows</strong></summary>

1. Right-click the `.zip` → **Extract All** (or use [7-Zip](https://7-zip.org))
2. Double-click `Cloudflare-Scanner.exe`
3. A browser tab opens at `http://127.0.0.1:PORT`

*Troubleshooting:* Antivirus may flag `xray.exe` — add an exclusion for the extracted folder.
</details>

<details>
<summary><strong>Linux</strong></summary>

```bash
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

ARM64: use the `*-linux-arm64.tar.gz` archive.
</details>

<details>
<summary><strong>macOS</strong></summary>

```bash
tar -xzf Cloudflare-Scanner-*-darwin-arm64.tar.gz   # Apple Silicon
chmod +x Cloudflare-Scanner xray
xattr -dr com.apple.quarantine xray  # remove Gatekeeper flag
./Cloudflare-Scanner
```

If blocked: **System Settings → Privacy & Security → Open Anyway**.
</details>

<details>
<summary><strong>Termux / Android</strong></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
scan
```

*Update:* re-run the same one-liner. *Remove:* `rm -rf ~/.local/share/cloudflare-scanner && rm $PREFIX/bin/scan`
</details>

### 3. Use the App

After launching, a browser opens with three tabs:

| Tab | What it does | When to use |
|-----|-------------|-------------|
| **Endpoint Scanner** | Finds working WARP WireGuard endpoints | Your WARP/WireGuard client is slow or disconnected |
| **IP Scanner** | Finds clean Cloudflare proxy IPs (VLESS/Trojan) | You need fresh IPs for a proxy config |
| **IP Replacer** | Batch-replaces IP:port across configs | You have working endpoints and want them injected into subscriptions |

Each tab has built-in guidance and tooltips — start with **Endpoint Scanner** if you just need a working WARP endpoint.

---

## Features

| Feature | Details |
|---------|---------|
| **WARP endpoint scanning** | Tests WireGuard endpoints via SOCKS5 through real xray-core outbound |
| **Clean IP scanning** | Two-phase: fast TCP probe → Xray validation (VLESS/Trojan) |
| **Port selection** | All 13 official CF CDN ports (HTTP + HTTPS) |
| **UDP noise** | Random padding + jitter to bypass DPI-based blocking |
| **Nearby scan** | Expands around working clean IPs to find adjacent targets |
| **Subscription replacer** | Fetch → deduplicate → inject fresh IP:port into VLESS/Trojan configs |
| **Batch apply** | Update many `.conf` files at once with a single endpoint |
| **Multi-attempt validation** | Each endpoint tested multiple times, median latency reported |
| **Cloudflare colo detection** | Identifies which data center responds (FRA, AMS, IAD, etc.) |
| **Live streaming results** | Working endpoints appear as they're confirmed |
| **Bilingual UI** | English and Persian/Farsi, switchable instantly |
| **Mobile-friendly** | Responsive to 360 px, touch-optimized |
| **QR codes** | Generate QR for any config — scan with a phone |
| **Self-contained** | Bundles xray-core — no runtime dependencies |
| **Cancellable** | Stop, rescan, or reset at any time |

---

## Workflows

### Beginner — get a working WARP endpoint

```
1. Get a Warp .conf        ─>  see "Getting Warp Configs" inside the app
2. Upload → Start Scan     ─>  wait for results
3. Click best result       ─>  apply to your .conf files
```

### Advanced — clean IPs + bulk replace

```
1. Paste vless:// URL      ─>  Tab: IP Scanner
2. Start Clean Scan        ─>  wait for Phase 1 + Phase 2
3. Export working IPs
4. Paste subscription URL  ─>  Tab: IP Replacer
5. Select configs + paste endpoints → Generate
```

---

## Security & Privacy

- The web server binds to **127.0.0.1** only — no external access.
- Configs / subscriptions are processed **entirely on your machine**.
- Subscription fetching sends one HTTP request to the URL you type.
- The app contains no telemetry, analytics, or network calls beyond scan traffic.

---

## Documentation

| Guide | Audience | Description |
|-------|----------|-------------|
| [Getting Started](docs/getting-started.md) | Users | First-use walkthrough after launching |
| [Endpoint Scanner](docs/endpoint-scanner.md) | Users | Deep dive into WARP endpoint scanning |
| [IP Scanner](docs/ip-scanner.md) | Users | Clean IP scanning explained |
| [IP Replacer](docs/ip-replacer.md) | Users | Subscription batch-replacement |
| [FAQ](docs/faq.md) | Users | Troubleshooting and common questions |
| [BUILD.md](BUILD.md) | Developers | Build, architecture, API reference |
| [docs/index.md](docs/index.md) | All | Documentation table of contents |

Persian docs: [`docs/fa/`](docs/fa/index.md)

---

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

---

## License

[MIT](LICENSE) — free to use, modify, and distribute.
