# Cloudflare Scanner

**Find working Cloudflare Warp endpoints and clean proxy IPs — fast, free, and no setup required.**

[![Latest Release](https://img.shields.io/github/v/release/QMahyar/Cloudflare-Scanner?style=flat-square&label=Download)](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/QMahyar/Cloudflare-Scanner/ci.yml?branch=master&style=flat-square&label=CI)](https://github.com/QMahyar/Cloudflare-Scanner/actions/workflows/ci.yml)
[![Downloads](https://img.shields.io/github/downloads/QMahyar/Cloudflare-Scanner/total?style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases)
[![License](https://img.shields.io/github/license/QMahyar/Cloudflare-Scanner?style=flat-square)](LICENSE)

> ### 🌐 Persian / فارسی
> [**مشاهده نسخه فارسی ← README.fa.md**](README.fa.md)

---

## What is this?

**Cloudflare Scanner** is a cross-platform desktop tool that helps you:

- **Find working Warp endpoints** — scan hundreds of Cloudflare WARP/WireGuard IPs:ports and rank them by latency so you can pick the fastest working one.
- **Find clean Cloudflare IPs** — probe Cloudflare's global IP ranges via TCP and then validate through xray-core (VLESS/Trojan), so you know exactly which IPs your config will actually connect through.
- **Batch-update configs** — replace the IP:port in any number of subscription configs at once with the freshly discovered endpoints.

It runs a tiny local web server and opens a browser tab — no installation, no dependencies. Just download, extract, and run.

### Who needs this?

If you use **Cloudflare Warp**, **v2ray**, **Nekobox**, **Sing-box**, or any proxy client built on Cloudflare's network, your performance depends entirely on which IP:port the client connects to. ISPs frequently block specific endpoints. This tool finds the ones that still work, ranked by speed.

---

## Download

| Platform | Architecture | Download |
|----------|-------------|---------|
| 🪟 Windows | x86-64 | [`windows-amd64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🪟 Windows | ARM64 | [`windows-arm64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 Linux | x86-64 | [`linux-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 Linux | ARM64 / Raspberry Pi | [`linux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 macOS | Intel | [`darwin-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 macOS | Apple Silicon | [`darwin-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 📱 Android (Termux) | ARM64 | [`termux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |

Each archive contains the app **and** xray-core — nothing else to install.

---

## One-Liner Install

```powershell
# Windows — run PowerShell as Administrator
irm https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-windows.ps1 | iex
```

```sh
# Linux
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-linux.sh | sh

# macOS
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-macos.sh | sh

# Termux / Android
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
```

The scripts auto-detect your CPU architecture, download the correct release, and add `cloudflare-scanner` (or `scan` on Termux) to your PATH.

---

## Manual Setup

<details>
<summary><strong>Windows</strong></summary>

1. Download `Cloudflare-Scanner-*-windows-amd64.zip` from [Releases](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest)
2. Right-click → **Extract All** (or use [7-Zip](https://7-zip.org))
3. Double-click `Cloudflare-Scanner.exe`
4. A browser tab opens automatically

**Troubleshooting:** Antivirus may flag `xray.exe` — add an exclusion for the extracted folder. Run as Administrator if the app fails to start.
</details>

<details>
<summary><strong>Linux</strong></summary>

```bash
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

ARM64 (Raspberry Pi, etc.): use the `*-linux-arm64.tar.gz` archive.
</details>

<details>
<summary><strong>macOS</strong></summary>

```bash
tar -xzf Cloudflare-Scanner-*-darwin-arm64.tar.gz   # Apple Silicon
# or darwin-amd64 for Intel
chmod +x Cloudflare-Scanner xray
xattr -dr com.apple.quarantine xray  # remove Gatekeeper flag
./Cloudflare-Scanner
```

If macOS still blocks the app: **System Settings → Privacy & Security → Open Anyway**.
</details>

<details>
<summary><strong>Termux / Android</strong></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
scan   # run the app
```

- Updates: re-run the same one-liner
- Remove: `rm -rf ~/.local/share/cloudflare-scanner && rm $PREFIX/bin/scan`
</details>

---

## How to Use

After launching, a browser tab opens at `http://127.0.0.1:PORT`. The app has three tabs:

### Tab 1 — Endpoint Scanner

Use this to find a fast Warp WireGuard endpoint.

1. Get a Warp `.conf` file — see [Getting Warp Configs](#getting-warp-configs)
2. Toggle **Use Real Config** and upload your `.conf`
3. Choose scan depth (Quick=100 / Normal=500 / Deep=1K+)
4. Click **Start Scan**
5. Results appear in real-time, sorted by latency
6. Click a result → **Apply** it to one or more `.conf` files

> **Without a config:** disable "Use Real Config" for a fast TCP-only scan — no xray needed.

### Tab 2 — IP Scanner

Use this to find clean Cloudflare IPs for VLESS/Trojan configs.

1. Paste your `vless://` or `trojan://` URL
2. Choose which **ports to scan**: 443 only, all HTTPS, all HTTP+HTTPS, or a custom selection
3. Set scan depth and click **Start Clean Scan**
4. **Phase 1** — fast TCP probe across Cloudflare IP ranges
5. **Phase 2** — xray-core validates the top Phase 1 candidates
6. Export working IPs → use with IP Replacer

> **Port selection:** The scanner supports all official Cloudflare CDN ports (HTTP: 80, 8080, 8880, 2052, 2082, 2086, 2095 · HTTPS: 443, 8443, 2053, 2083, 2087, 2096).

### Tab 3 — IP Replacer

Use this to inject fresh IPs into existing configs in bulk.

1. Paste a subscription URL **or** raw config text
2. Select which configs to update
3. Paste the `IP:port` endpoints discovered in the IP Scanner
4. Click **Generate Configs** → Copy All or Download

---

## Full Workflow (Beginner to Advanced)

| Step | Action | Tab |
|------|--------|-----|
| 1 | Get a Warp `.conf` from a generator (see links in the app) | — |
| 2 | Upload `.conf` → Start Scan → pick the fastest result | Endpoint Scanner |
| 3 | Apply the endpoint to your `.conf` files | Endpoint Scanner |
| 4 | Paste your `vless://` URL → scan clean IPs → export | IP Scanner |
| 5 | Paste subscription URL → select configs → inject endpoints | IP Replacer |

> **First time?** Steps 1–3 alone give you a working Warp endpoint.

---

## Features

| Feature | Details |
|---------|---------|
| Endpoint scanning | Tests Warp WireGuard endpoints with optional xray validation |
| IP scanning | CIDR-based generation from 25 IPv4 + 91 IPv6 Cloudflare ranges |
| Port selection | Choose from all 13 official Cloudflare CDN ports (HTTP + HTTPS) |
| Nearby scan | After Phase 1, expands around working IPs to find adjacent clean IPs |
| UDP noise | Random padding + jitter to evade DPI-based Warp blocking |
| Batch apply | Update many `.conf` files at once with a single endpoint |
| Subscription | Fetch, deduplicate, and batch-replace IP:port in VLESS/Trojan configs |
| Live results | Endpoints stream in as they are confirmed, sorted by latency |
| Bilingual UI | English and Persian/Farsi, switch instantly |
| Mobile UI | Responsive down to 360 px, touch-optimised |
| QR codes | Generate QR for any config — scan with your phone |
| Folder picker | Native OS dialog to choose output directory |
| Self-contained | Ships with xray-core, nothing else to install |
| Cancellable | Stop, rescan, or reset at any time |

---

## Getting Warp Configs

The **Endpoint Scanner** tab contains a built-in help section with links to:
- Online Warp config generators
- Telegram bots that generate configs on demand
- Open-source CLI tools
- WireGuard client apps

Scroll to the bottom of the Endpoint Scanner tab to find them.

---

## Build from Source

Requires **Go 1.26+** — no C compiler needed.

```bash
git clone https://github.com/QMahyar/Cloudflare-Scanner.git
cd Cloudflare-Scanner
go build -ldflags="-s -w -X 'main.Version=dev'" -o Cloudflare-Scanner .
```

See [BUILD.md](BUILD.md) for cross-platform build commands, project architecture, and the HTTP API reference.

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | First-time setup and end-to-end workflow |
| [Endpoint Scanner](docs/endpoint-scanner.md) | Deep dive into Warp endpoint scanning |
| [IP Scanner](docs/ip-scanner.md) | Two-phase clean IP scanning |
| [IP Replacer](docs/ip-replacer.md) | Batch IP replacement in configs |
| [FAQ](docs/faq.md) | Troubleshooting and common questions |
| [BUILD.md](BUILD.md) | Build from source, architecture, API |

Persian docs: [`docs/fa/`](docs/fa/index.md)

---

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for the full release history.

---

## License

[MIT](LICENSE) — free to use, modify, and distribute.
