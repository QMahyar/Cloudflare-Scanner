# Getting Started

> **Step 0 — Install the app:** See the [README](../README.md) for OS-specific download, extract, and run instructions.

---

## First-Use Workflow

These steps assume the app is running and you see the web UI at `http://127.0.0.1:XXXXX`.

### Step 1 — Open the Web UI

A browser tab opens automatically when you launch the app. Look for a banner like this in the terminal:

```
  ╔══════════════════════════════════════════════════════╗
  ║         Cloudflare Scanner v3.0.0                   ║
  ║    Open your browser to the URL below               ║
  ╚══════════════════════════════════════════════════════╝

  ➜  http://127.0.0.1:52824
```

If the browser doesn't open automatically, copy that URL and paste it into your browser manually. The port changes every run — that's normal.

### Step 2 — Switch Language

Click the **فارسی** button in the top-right corner to switch to Persian/Farsi. Click **English** to switch back. The entire UI — labels, tooltips, placeholders — switches instantly.

### Step 3 — Get a Warp Config

Scroll down on the **Endpoint Scanner** tab. The app includes a collapsible **Getting Warp Configs** panel with links to online generators, Telegram bots, CLI tools, and client apps.

### Step 4 — Scan Warp Endpoints

1. Upload your `.conf` file (or disable "Use Real Config" for a quick TCP-only check)
2. Pick a scan depth (Quick = 100 is fine for a first try)
3. Click **Start Scan**
4. Results appear live — pick the fastest one

### Step 5 — Apply the Endpoint

1. Click the endpoint in the results table to auto-fill the **Endpoint to apply** field
2. Click **Choose config(s)...** and select your `.conf` file(s)
3. Click **Generate Configs**
4. Use the modified config with your Warp client

---

## What's in the Archive

| File | Description |
|------|-------------|
| `Cloudflare-Scanner` (or `.exe`) | The main app — web server + scanning engine |
| `xray` (or `xray.exe`) | xray-core v1.8.24 — Warp validation + proxy connections |

Both files are required. Keep them in the same folder.

---

## Stop the App

Close the terminal window, or press **Ctrl+C** in the terminal. There is no graceful shutdown — closing the terminal kills the server immediately.

---

## Next Steps

- [Endpoint Scanner](endpoint-scanner.md) — full walkthrough with all options
- [IP Scanner](ip-scanner.md) — find clean Cloudflare proxy IPs
- [IP Replacer](ip-replacer.md) — batch-replace IPs in configs
- [FAQ](faq.md) — troubleshooting and common questions
