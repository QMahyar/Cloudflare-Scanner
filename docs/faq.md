# Frequently Asked Questions

## General

### What does this app do?

It has three tools:
1. **Endpoint Scanner** — Finds Cloudflare Warp endpoints where your WireGuard config works
2. **IP Scanner** — Finds Cloudflare IPs that work as proxies with your VLESS/Trojan URL
3. **IP Replacer** — Generates all combinations of your subscription configs with a list of clean endpoints

### Which platforms are supported?

Windows (Intel and ARM), Linux (Intel and ARM), macOS (Intel and Apple Silicon), and Termux on Android (ARM64).

### Do I need to install anything else?

No. The app bundles xray-core v1.8.24. Everything is self-contained in the archive.

### Does it work on phones?

Yes. The web UI is fully responsive down to 360 px. Open `http://127.0.0.1:PORT` in your phone's browser after launching the app on a desktop/server, or run it directly in Termux on Android.

## Running

### The browser doesn't open automatically

Look at the terminal output. You'll see a banner like:

```
  ➜  http://127.0.0.1:53671
```

Copy that address and paste it into your browser manually.

### Linux says "xdg-open not found"

Install `xdg-utils`:

- Debian/Ubuntu: `sudo apt install xdg-utils`
- Fedora: `sudo dnf install xdg-utils`
- Arch: `sudo pacman -S xdg-utils`

### Can I run it on a headless server?

Yes. The web UI runs on `127.0.0.1` by default. You can port-forward with SSH (`ssh -L 8080:127.0.0.1:XXXXX server`) or use a reverse proxy.

## Endpoint Scanner

### What is UDP Noise?

Some ISPs block or throttle WireGuard traffic on port 2408 (Cloudflare Warp's default port). UDP Noise sends a random 50–100 byte packet followed by a 1–5 ms delay before the actual WireGuard handshake, making the traffic look non-standard to Deep Packet Inspection (DPI) systems.

### What config formats are supported?

Standard WireGuard `.conf` files and Hogwarts-style configs with fields like S1, S2, S3, S4, Jc, Jmin, H1–H4, I1, I2.

### Can I scan without a config file?

Yes. Disable **Use Real Config**. The scan degrades to a plain TCP dial — no xray validation. Good for quickly finding reachable endpoints.

### How do I get a Warp config?

Scroll down on the Endpoint Scanner tab. The **Getting Warp Configs** panel has links to online generators, Telegram bots, CLI tools, and client apps. Popular options:

- **warp-generator.github.io/warp/** — browser-based config generator
- **@warp_generator_bot** — Telegram bot with 54k+ users
- **ViRb3/wgcf** — classic CLI generator

## IP Scanner

### What's the difference between Phase 1 and Phase 2?

- **Phase 1** — TCP dial test. Checks if the IP:port accepts TCP connections. Fast, large scale (configurable up to 2,000 concurrent workers, default 500).
- **Phase 2** — xray validation. Runs xray-core with your VLESS config against the endpoint and sends an HTTP request through it. Confirms the IP actually proxies traffic.

### What's the difference between "Phase 2 count" and "Phase 2 probes"?

- **Phase 2 count** — how many of the top Phase 1 results to validate (e.g. top 20)
- **Phase 2 probes** — how many concurrent xray validations run simultaneously (e.g. 12 at a time)

### Can I use IP Scanner without a VLESS URL?

Yes. Disable **Use Real Config** — only Phase 1 (TCP probe) will run. Choose your ports manually in the port selection grid.

### What does "Push to Replacer" do?

After a scan, **Push to Replacer** sends all working `ip:port` pairs directly to the IP Replacer tab's endpoints field, so you can generate new configs immediately without copying/pasting.

### How are IPs generated?

From **25 IPv4 CIDR ranges** and **91 IPv6 CIDR ranges** covering Cloudflare's official AS13335 address space. IPv4 uses weighted random selection (larger ranges get proportionally more hits). IPv6 uses uniform random selection.

## IP Replacer

### What config formats does it accept?

`vless://`, `trojan://`, and `vmess://` share URLs. Other protocols are ignored.

### What separators work when pasting?

Newlines, spaces, commas, semicolons, and pipes: `, ; |`. You can mix them.

### It says "no valid configs found"

Check that:
- Your subscription URL is accessible (not behind a login page or expired)
- Your pasted text contains valid `vless://`, `trojan://`, or `vmess://` URLs
- The subscription response is not empty or HTML (some providers return a login page)

## Troubleshooting

### xray failed to start

Make sure `xray` (or `xray.exe` on Windows) is in the same folder as the app. Check your antivirus — it may block xray-core.

### Scan shows 0 results

- Your config may be invalid or expired
- Your ISP may be blocking the connection
- Try increasing scan depth
- Enable UDP Noise if you suspect DPI blocking
- In IP Scanner, try "HTTPS (6)" or "All (13)" port presets instead of 443 only

### App port keeps changing

This is normal. The app binds to a random available port on `127.0.0.1` each time it starts.
