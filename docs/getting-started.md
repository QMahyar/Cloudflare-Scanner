# Getting Started

> **Step 0 — Install the app:** See the [README](../README.md#platform-guides) for OS-specific download, extract, and run instructions.

---

## First-Use Workflow

These steps assume the app is running and you see the web UI at `http://127.0.0.1:XXXXX`.

### Step 1 — Open the Web UI

A browser tab should open automatically at `http://127.0.0.1:XXXXX` (port changes each run).

If the browser doesn't open, look at the terminal output for a line like:

```
Web UI: http://127.0.0.1:53671
```

Copy that address and paste it into your browser manually.

### Step 2 — Switch Language

Click the **فارسی** button in the top-right corner to switch between English and Persian/Farsi. The entire UI switches instantly.

### Step 3 — Get a Warp Config

Scroll down on the **Endpoint Scanner** tab. The app includes links to online generators, Telegram bots, CLI tools, and client apps.

### Step 4 — Scan Warp Endpoints

1. Upload your `.conf` file
2. Pick a scan depth (Quick = 100 is fine for a first try)
3. Click **Start Scan**
4. Results appear live — pick the fastest one

### Step 5 — Apply the Endpoint

1. Click the endpoint in the results table
2. Select your `.conf` file(s)
3. Click **Generate Configs**
4. Use the modified config with your Warp client

---

## What's in the Archive

| File | Description |
|---|---|
| `Cloudflare-Scanner` (or `.exe`) | The main app — web server + scanning engine |
| `xray` (or `xray.exe`) | xray-core v1.8.24 — Warp validation + proxy connections |

Both files are required. Keep them in the same folder.

---

## Stop the App

Close the terminal window, or press **Ctrl+C** in the terminal.

---

## Next Steps

- [Endpoint Scanner](endpoint-scanner.md) — full walkthrough with all options
- [IP Scanner](ip-scanner.md) — find clean Cloudflare proxy IPs
- [IP Replacer](ip-replacer.md) — batch-replace IPs in configs
- [FAQ](faq.md) — troubleshooting and common questions
