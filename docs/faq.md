# Frequently Asked Questions

## General

### What does this app do?

It has three tools:
1. **Endpoint Scanner** — Finds Cloudflare Warp endpoints where your WireGuard config works
2. **IP Scanner** — Finds Cloudflare IPs that work as proxies with your VLESS URL
3. **IP Replacer** — Generates all combinations of your subscription configs with a list of clean endpoints

### Which platforms are supported?

Windows (Intel and ARM), Linux (Intel and ARM), macOS (Intel and Apple Silicon), and Termux on Android (ARM64).

### Do I need to install anything else?

No. The app bundles xray-core v1.8.24. Everything is self-contained in the archive.

## Running

### The browser doesn't open automatically

Look at the terminal output. You'll see a line like:

```
Web UI: http://127.0.0.1:53671
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

Some ISPs block or throttle WireGuard traffic on port 2408 (Cloudflare Warp's default port). UDP Noise sends a random 50-100 byte packet followed by a 1-5ms delay before the actual WireGuard handshake, making the traffic look non-standard to Deep Packet Inspection (DPI) systems.

### What config formats are supported?

Standard WireGuard `.conf` files and Hogwarts-style configs with fields like S1, S2, S3, S4, Jc, Jmin, H1-H4, I1, I2.

### How do I get a Warp config?

Scroll down on the Endpoint Scanner tab. The app includes links to online generators, Telegram bots, CLI tools, and client apps. Popular options:

- **warp-generator.github.io/warp/** — browser-based config generator
- **@warp_generator_bot** — Telegram bot with 54k+ users
- **ViRb3/wgcf** — classic CLI generator

## IP Scanner

### What's the difference between Phase 1 and Phase 2?

- **Phase 1** — TCP dial test. Checks if the IP:port accepts TCP connections. Fast, large scale (500 concurrent workers).
- **Phase 2** — xray validation. Runs xray-core with your VLESS config against the endpoint and sends an HTTP request through it. Confirms the IP actually proxies traffic.

### Can I use IP Scanner without a VLESS URL?

Yes. Enable **1-phase mode** and leave the VLESS URL field empty. Only Phase 1 (TCP probe) will run, using port 443.

### How are IPs generated?

From **25 IPv4 CIDR ranges** and **91 IPv6 CIDR ranges** covering Cloudflare's official AS13335 address space. IPv4 uses weighted random selection (larger ranges get proportionally more hits). IPv6 uses uniform random selection.

## IP Replacer

### What config formats does it accept?

`vless://` and `trojan://` share URLs only. Other protocols are ignored.

### What separators work when pasting?

Newlines, spaces, commas, semicolons, and pipes: `, ; |`. You can mix them.

### It says "no valid configs found"

Check that:
- Your subscription URL is accessible (not behind a login page or expired)
- Your pasted text contains valid `vless://` or `trojan://` URLs
- The subscription response is not empty or HTML (some providers return a login page)

## Troubleshooting

### xray failed to start

Make sure `xray` (or `xray.exe` on Windows) is in the same folder as the app. Check your antivirus — it may block xray-core.

### Scan shows 0 results

- Your config may be invalid or expired
- Your ISP may be blocking the connection
- Try increasing scan depth
- Enable UDP Noise if you suspect DPI blocking

### App port keeps changing

This is normal. The app binds to a random available port on `127.0.0.1` each time it starts.
