# IP Scanner (Clean IP)

Finds Cloudflare proxy IPs that respond to TCP connections and optionally validates them against your VLESS URL through xray-core. Useful for finding clean Cloudflare IPs for v2ray, Nekobox, Sing-box, and similar tools.

---

## How It Works

The scan runs in two phases:

```
Phase 1: TCP Probe       Phase 2: xray Validation
────────────────────     ────────────────────────
Generate IPs from  ──>   Take top N from
Cloudflare CIDRs         Phase 1 results
      │                          │
TCP Dial to each        Start xray with your
IP:port with 500        VLESS config, connect
concurrent workers      via SOCKS5, send HTTP
      │                  request to gstatic.com
Collect successful      Keep endpoints that
responses, sort         return HTTP 204/200
by latency              (12 concurrent workers)
```

---

## Step-by-Step

### Step 1 — Enter a VLESS URL (or leave empty)

Paste a `vless://...` or `trojan://...` share URL. This URL is used in Phase 2 to validate endpoints through xray-core.

- **If you enter a URL:** Phase 2 will run after Phase 1 completes. Each endpoint is tested by running xray-core with this config and checking if a proxy connection succeeds.
- **If you leave it empty:** Only Phase 1 (TCP probe) runs. You should also check **1-phase mode**.

The port from the URL is used for both phases.

### Step 2 — Toggle 1-Phase Mode

- **Unchecked (default):** Full two-phase scan. Phase 1 finds responsive IPs, Phase 2 validates the best ones.
- **Checked:** Only TCP probe runs. Useful if you just want a list of Cloudflare IPs that accept TCP connections. The default port is 443 when no VLESS URL is provided.

### Step 3 — Set Scan Depth

| Option | IPs to test |
|---|---|
| Fast | 500 |
| Normal (default) | 1000 |
| Deep | 2000 |
| Max | 5000 |

### Step 4 — Set Phase 2 Probes

How many of the top Phase 1 results to validate through xray-core:

| Option | Probes | When to use |
|---|---|---|
| 10 | — | Quick validation |
| 20 (default) | — | Good balance |
| 30 | — | More thorough |
| 50 | — | Comprehensive |

### Step 5 — Choose IP Version

| Option | What it scans |
|---|---|
| IPv4 only (default) | Cloudflare IPv4 ranges (25 CIDRs) |
| IPv6 only | Cloudflare IPv6 ranges (91 CIDRs) |
| IPv4 + IPv6 | Both, mixed |

### Step 6 — Start Clean Scan

Click **Start Clean Scan**.

During **Phase 1**, you see live progress (`Phase 1: TCP probe — X / Y`). Endpoints that respond appear in the results area with their latency.

During **Phase 2**, you see `Phase 2: xray validation — X / Y`. Results update as each endpoint is validated.

Click **Stop** at any time to cancel.

### Step 7 — Export Results

Once the scan completes:

1. Click **Download VLESS Configs** to get a text file with each working IP as a share URL
2. Click **Download IP List** to get a plain list of `ip:port` pairs (one per line)

The VLESS export preserves all original config parameters (SNI, path, encryption, etc.) — only the IP and port are replaced with each working endpoint.

---

## IP Generation Details

IPs are generated from **25 IPv4 CIDR ranges** and **91 IPv6 CIDR ranges** sourced from Cloudflare's official AS13335 ranges.

IPv4 uses **weighted random selection** — larger ranges (e.g. `/12`) get proportionally more hits than smaller ranges (e.g. `/24`). This ensures even coverage across Cloudflare's address space.

IPv6 uses **uniform random selection** from all 91 CIDRs.

Each generated IP is unique — duplicates are skipped.

---

## Technical Notes

- **Phase 1:** Uses `net.DialTimeout` with a 3-second timeout, 500 concurrent goroutines, semaphore-limited
- **Phase 2:** Each endpoint gets a dedicated xray process with a unique SOCKS5 port (starting from 20800). 12 concurrent validations. 5-second test timeout.
- **Validation:** xray connects to the endpoint, then the app sends an HTTP request through the SOCKS5 proxy to `www.gstatic.com/generate_204`. HTTP 204 or 200 counts as success.
- **TCP-only:** Phase 1 only checks TCP connectivity. It does not send any data — just establishes and closes the connection.
