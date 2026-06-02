# Getting Started

## 1. Download

Go to the [releases page](https://github.com/QMahyar/Cloudflare-Scanner/releases) and download the archive for your platform:

| Platform | Archive to download |
|---|---|
| Windows (Intel/AMD) | `Cloudflare-Scanner-*-windows-amd64.tar.gz` |
| Windows (ARM, e.g. Surface Pro X) | `Cloudflare-Scanner-*-windows-arm64.tar.gz` |
| Linux (Intel/AMD) | `Cloudflare-Scanner-*-linux-amd64.tar.gz` |
| Linux (ARM, e.g. Raspberry Pi) | `Cloudflare-Scanner-*-linux-arm64.tar.gz` |
| macOS (Intel) | `Cloudflare-Scanner-*-darwin-amd64.tar.gz` |
| macOS (Apple Silicon M1/M2/M3) | `Cloudflare-Scanner-*-darwin-arm64.tar.gz` |
| Termux / Android (ARM64) | `Cloudflare-Scanner-*-termux-arm64.tar.gz` |

## 2. Extract & Run

### Windows

1. Right-click the `.tar.gz` file and choose **Extract All** (7-Zip), or run `tar.exe -xzf Cloudflare-Scanner-*-windows-amd64.tar.gz`
2. Open the extracted folder
3. Double-click `Cloudflare-Scanner.exe`

### Linux / macOS

```bash
# Open a terminal in the Downloads folder
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
cd Cloudflare-Scanner-*-linux-amd64
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

**Note:** Linux requires `xdg-utils` for the browser to open automatically:

- Debian/Ubuntu: `sudo apt install xdg-utils`
- Fedora: `sudo dnf install xdg-utils`
- Arch: `sudo pacman -S xdg-utils`

### Termux (Android)

```bash
curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
```

This downloads the latest Termux release, extracts it, and creates a `scan` command. Just type `scan` to run later.

No extra packages needed — the app uses `termux-open-url` to open the browser.

## 3. Open the Web UI

A browser tab automatically opens at `http://127.0.0.1:XXXXX` (the port changes each time).

If the browser doesn't open automatically, look at the terminal output for the address and open it manually.

## 4. Switch Language

Click the **فارسی** button in the top-right corner to switch between English and Persian/Farsi. The entire UI, including labels, placeholders, and tooltips, switches instantly.

## 5. Stop the App

Close the terminal window, or press **Ctrl+C** in the terminal.

---

## What's in the Archive

| File | Description |
|---|---|
| `Cloudflare-Scanner` (or `.exe`) | The main app — a web server + scanning engine |
| `xray` (or `xray.exe`) | xray-core v1.8.24 — handles Warp endpoint validation and proxy connections |

Both files are required. Keep them in the same folder.

---

## First Use Workflow

Here's a typical first session:

1. **Get a Warp config** — Use one of the online generators listed in the app's help section (scroll down on the Endpoint Scanner tab)
2. **Scan Warp endpoints** — Upload your `.conf` file, pick a scan depth, and scan
3. **Apply a good endpoint** — Pick the fastest result, select your config files, save modified copies
4. **(Optional) Scan clean IPs** — Paste a VLESS URL, find working Cloudflare proxy IPs
5. **(Optional) Replace IPs in configs** — Paste subscription or configs + endpoints, generate refreshed configs
