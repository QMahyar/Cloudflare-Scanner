# Cloudflare Scanner

**Three-in-one cross-platform tool** — scan Warp endpoints, find clean Cloudflare proxy IPs, and replace IP:port in subscription configs.

> ## 🌐 Persian / فارسی
>
> [**مشاهده نسخه فارسی README.fa.md**](README.fa.md) — راهنمای کامل به زبان فارسی، شامل راهنمای نصب گام‌به‌گام برای هر سیستم‌عامل، فهرست مطالب، و مسیر گام‌به‌گام کار با برنامه.
>
> [همه مستندات فارسی](docs/fa/index.md)

---

## Table of Contents

| # | Section | |
|---|---|---|
| 1 | [Quick Start](#quick-start) | Download, extract, run — 10 seconds |
| 2 | [Platform Guides](#platform-guides) | Step-by-step per OS |
| 3 | [The Three Tools](#the-three-tools) | What each tab does |
| 4 | [Workflow: 1 to 100](#workflow-1-to-100) | Complete walkthrough from zero to advanced |
| 5 | [Documentation](#documentation) | All guides, FAQ, troubleshooting |
| 6 | [Features](#features) | What makes this app different |
| 7 | [Getting Warp Configs](#getting-warp-configs) | Where to get WireGuard configs |
| 8 | [Build from Source](#build-from-source) | Compile it yourself |

---

## Quick Start

1. Download the [latest release](https://github.com/QMahyar/Cloudflare-Scanner/releases) for your platform
2. Extract and run:

| Platform | Command |
|---|---|
| Windows | Extract `.tar.gz`, double-click `Cloudflare-Scanner.exe` |
| Linux / macOS | `tar -xzf *.tar.gz && chmod +x Cloudflare-Scanner xray && ./Cloudflare-Scanner` |
| Termux | `curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh \| sh` then type `scan` |

3. A browser tab opens at `http://127.0.0.1:XXXXX`
4. Press **Ctrl+C** in the terminal to stop

xray-core v1.8.24 is bundled — no extra downloads.

---

## Platform Guides

Detailed, OS-specific setup. Choose your platform below.

### Windows

**Step 1 — Extract**
- Right-click the `.tar.gz` file → **Extract All** (7-Zip) or run: `tar.exe -xzf Cloudflare-Scanner-*-windows-amd64.tar.gz`

**Step 2 — Run**
- Open the extracted folder, double-click `Cloudflare-Scanner.exe`

**Step 3 — Use**
- A browser tab opens at `http://127.0.0.1:XXXXX`. If not, check the terminal output for the address.

**Firewall**: Windows may ask for network access — click **Allow**.

**Troubleshooting**:
- Antivirus may flag `xray.exe` — add an exclusion for the app folder
- Run as Administrator if the scan port is blocked

---

### Linux

**Step 1 — Install dependencies**
```bash
# Debian / Ubuntu
sudo apt install xdg-utils tar

# Fedora
sudo dnf install xdg-utils tar

# Arch
sudo pacman -S xdg-utils tar
```

**Step 2 — Extract**
```bash
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
cd Cloudflare-Scanner-*-linux-amd64
```

**Step 3 — Make executables**
```bash
chmod +x Cloudflare-Scanner xray
```

**Step 4 — Run**
```bash
./Cloudflare-Scanner
```

**Troubleshooting**:
- If the browser doesn't open, install `xdg-utils` (see above) or manually open `http://127.0.0.1:XXXXX`
- On ARM/ARM64 (Raspberry Pi, etc.), use the `*-linux-arm64.tar.gz` archive

---

### macOS

**Step 1 — Extract**
```bash
tar -xzf Cloudflare-Scanner-*-darwin-amd64.tar.gz
cd Cloudflare-Scanner-*-darwin-amd64
```

**Step 2 — Make executables**
```bash
chmod +x Cloudflare-Scanner xray
```

**Step 3 — Run**
```bash
./Cloudflare-Scanner
```

**Apple Silicon (M1/M2/M3/M4)**: Use the `*-darwin-arm64.tar.gz` archive.

**Troubleshooting**:
- **"xray cannot be opened because the developer cannot be verified"** — go to **System Settings → Privacy & Security** and click **Open Anyway**, or run: `xattr -d com.apple.quarantine xray`
- Gatekeeper may block the app — right-click → **Open** to bypass

---

### Termux / Android

**Step 1 — One-liner install**
```bash
curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
```

**Step 2 — Run**
```bash
scan
```

**Step 3 — Browser opens automatically** using `termux-open-url`.

**Notes**:
- No extra packages needed — everything is self-contained
- The `scan` command persists across Termux sessions
- To update: re-run the one-liner install command
- To remove: `rm -rf ~/cloudflare-scanner && sed -i '/alias scan=/d' ~/.bashrc`

**Troubleshooting**:
- **"command not found: scan"** — run `source ~/.bashrc` or restart Termux
- **Browser doesn't open** — check terminal for `Web UI: http://127.0.0.1:XXXXX` and open manually

---

## The Three Tools

| Tool | What it does | When to use |
|---|---|---|
| **Endpoint Scanner** | Tests Warp WireGuard endpoints using your `.conf` file | You need a working Warp endpoint for your VPN |
| **IP Scanner (Clean IP)** | Scans Cloudflare IP ranges for clean proxies with VLESS/Trojan validation | You need clean Cloudflare IPs for v2ray, Nekobox, Sing-box, etc. |
| **IP Replacer** | Replaces IP:port in subscription configs with discovered endpoints | You have stale configs and fresh IPs to inject |

---

## Workflow: 1 to 100

A complete walkthrough for first-time users. Follow these steps in order.

### Phase 1: First Scan

| Step | What to do | Where |
|---|---|---|
| 1 | Get a Warp WireGuard `.conf` file | See [Getting Warp Configs](#getting-warp-configs) below |
| 2 | Open the app (see [Quick Start](#quick-start) or your [Platform Guide](#platform-guides)) | Terminal → browser at `http://127.0.0.1:XXXXX` |
| 3 | Switch language if needed | Click **فارسی** in the top-right corner |
| 4 | Go to **Endpoint Scanner** tab | First tab in the web UI |
| 5 | Upload your `.conf` file | Click **Choose config...** and select the file |
| 6 | Set scan depth to **Quick (100)** | Dropdown menu — keeps it fast |
| 7 | Click **Start Scan** | Wait ~10–30 seconds for results |
| 8 | Pick the fastest result | Click an endpoint in the results table to copy it |

### Phase 2: Apply Your Endpoint

| Step | What to do | Where |
|---|---|---|
| 9 | Click the endpoint in results | It fills into **Endpoint to apply** |
| 10 | Click **Choose config(s)...** | Select one or more `.conf` files |
| 11 | Set **Output folder** (optional) | Leave empty to save next to the app |
| 12 | Click **Generate Configs** | Modified files are saved with `-modified` suffix |
| 13 | Use the modified config with your Warp client | Import into WireGuard, Nekoray, etc. |

### Phase 3: Advanced — Clean IP Scanning

| Step | What to do | Where |
|---|---|---|
| 14 | Go to **IP Scanner** tab | Second tab in the web UI |
| 15 | Paste a `vless://` or `trojan://` URL | From your subscription provider |
| 16 | Set depth to **Normal (1000)** | Good balance |
| 17 | Click **Start Clean Scan** | Phase 1 (TCP probe) runs first, then Phase 2 (xray validation) |
| 18 | Wait for results | Live progress shows in the UI |
| 19 | Export working IPs | Click **Download IP List** for `ip:port` pairs |

### Phase 4: Advanced — Replace IPs in Configs

| Step | What to do | Where |
|---|---|---|
| 20 | Go to **IP Replacer** tab | Third tab in the web UI |
| 21 | Paste subscription URL or raw configs | Toggle between **Subscription URL** and **Paste Configs** |
| 22 | Click **Fetch** or **Parse** | Configs are deduplicated and listed |
| 23 | Select the configs you want | Check the boxes |
| 24 | Paste the endpoints from Phase 3 | One `ip:port` per line |
| 25 | Click **Generate Configs** | Every config × every endpoint is produced |
| 26 | **Copy All** or **Download** | Use the generated URLs with your proxy client |

### Phase 5: Master

| Step | What to do | Where |
|---|---|---|
| 27 | Combine all three tools in a loop | Scan → apply → scan clean IPs → replace → repeat |
| 28 | Tune scan depths and UDP noise | See [Endpoint Scanner](docs/endpoint-scanner.md) and [IP Scanner](docs/ip-scanner.md) guides |
| 29 | Batch-apply across many configs | See [IP Replacer](docs/ip-replacer.md) guide |
| 30 | Read the [FAQ](docs/faq.md) | Troubleshoot common issues |

> **New user?** Start at Step 1 and go through Phase 1 and Phase 2. That's all you need to get a working Warp endpoint.

---

## Documentation

All guides are in `docs/` with Persian translations in `docs/fa/`.

| Guide | English | Persian | What it covers |
|---|---|---|---|
| Documentation Hub | [index.md](docs/index.md) | [fa/index.md](docs/fa/index.md) | Navigation hub — start here |
| Getting Started | [getting-started.md](docs/getting-started.md) | [fa/getting-started.md](docs/fa/getting-started.md) | Download, extract, run, first workflow |
| Endpoint Scanner | [endpoint-scanner.md](docs/endpoint-scanner.md) | [fa/endpoint-scanner.md](docs/fa/endpoint-scanner.md) | Full walkthrough of the first tab |
| IP Scanner | [ip-scanner.md](docs/ip-scanner.md) | [fa/ip-scanner.md](docs/fa/ip-scanner.md) | Two-phase clean IP scanning |
| IP Replacer | [ip-replacer.md](docs/ip-replacer.md) | [fa/ip-replacer.md](docs/fa/ip-replacer.md) | Batch IP replacement in configs |
| FAQ | [faq.md](docs/faq.md) | [fa/faq.md](docs/fa/faq.md) | Troubleshooting and common questions |

**Suggested reading order:**
1. [Getting Started](docs/getting-started.md) — first-time setup
2. [Endpoint Scanner](docs/endpoint-scanner.md) — find your first endpoint
3. [IP Scanner](docs/ip-scanner.md) — find clean proxy IPs
4. [IP Replacer](docs/ip-replacer.md) — batch-replace IPs in configs
5. [FAQ](docs/faq.md) — when something doesn't work

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
