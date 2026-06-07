# IP Scanner (Clean IP)

Finds Cloudflare proxy IPs that respond to TCP connections and optionally validates them against your VLESS URL through xray-core. Useful for finding clean Cloudflare IPs for v2ray, Nekobox, Sing-box, and similar tools.

---

## How It Works

The scan runs in two phases:

```
Phase 1: TCP Probe            Phase 2: xray Validation
─────────────────────         ────────────────────────
Generate IPs from         ──> Take top N from
Cloudflare CIDRs               Phase 1 results
      │                               │
TCP dial to each              Start xray with your
IP:port (configurable         VLESS config, connect
concurrent workers)           via SOCKS5, send HTTP
      │                       request to gstatic.com
Collect successful            Keep endpoints that
responses, sort by            return HTTP 204/200
latency, detect colo          (configurable workers)
```

---

## Step-by-Step

### Step 1 — Enter a VLESS URL (or leave empty)

Paste a `vless://...` or `trojan://...` share URL. This URL is used in Phase 2 to validate endpoints through xray-core.

- **If you enter a URL:** Phase 2 runs after Phase 1 completes. Each endpoint is tested by running xray-core with this config and checking if a proxy connection succeeds.
- **If you leave it empty (and disable "Use Real Config"):** Only Phase 1 (TCP probe) runs.

The port from the URL is also used as the default port for the "Config port" preset in Port Selection.

### Step 2 — Set Scan Depth

Choose how many IPs to probe:

| Option | IPs to test | When to use |
|--------|-------------|-------------|
| Quick | 100 | Fast check |
| Normal (default) | 500 | Good balance |
| Deep | 1,000 | Thorough |
| Insane | 5,000 | Very thorough |
| Massive | 10,000 | Comprehensive |
| Custom | (you enter) | Any number |

### Step 3 — Set Phase 1 Probes

Number of **concurrent TCP workers** during Phase 1:

| Option | Concurrent workers |
|--------|--------------------|
| 100 | Light |
| 250 | Moderate |
| 500 (default) | Standard |
| 1,000 | Aggressive |
| 2,000 | Maximum |

Higher values finish faster but use more CPU/memory.

### Step 4 — Set Phase 2 Count and Probes

**Phase 2 count** — how many of the top Phase 1 results to validate:

| Option | Count |
|--------|-------|
| 10 | Quick validation |
| 20 (default) | Good balance |
| 30 | More thorough |
| 50 | Comprehensive |

**Phase 2 probes** — concurrent xray validations:

| Option | Concurrent |
|--------|-----------|
| 5 | Light |
| 12 (default) | Standard |
| 25 | Fast |
| 50 | Aggressive |
| 100 | Maximum |

### Step 5 — Choose Ports to Scan

Select which Cloudflare CDN ports to probe. Use the quick-select presets:

| Preset | Ports |
|--------|-------|
| 443 only | HTTPS 443 |
| HTTPS (6) | 443, 8443, 2053, 2083, 2087, 2096 |
| HTTP (7) | 80, 8080, 8880, 2052, 2082, 2086, 2095 |
| All (13) | All 13 Cloudflare CDN ports |
| Config port | Port extracted from your VLESS URL |

You can also check/uncheck individual ports in the grid below the presets.

### Step 6 — Choose IP Version

| Option | What it scans |
|--------|--------------|
| IPv4 only (default) | Cloudflare IPv4 ranges (25 CIDRs) |
| IPv6 only | Cloudflare IPv6 ranges (91 CIDRs) |
| IPv4 + IPv6 | Both, mixed |

### Step 7 — Enable Nearby Scan (optional)

Toggle **Scan nearby ranges** to expand around any working Phase 1 IPs. For each working IPv4 address, the full `/24` subnet is probed; for IPv6 the `/64`. Results from the nearby scan appear in a separate **Nearby** list.

### Step 8 — Start Clean Scan

Click **Start Clean Scan**.

During **Phase 1**, you see live progress (`Phase 1: TCP probe — X / Y`). Endpoints that respond appear in the results area with their latency and Cloudflare colo (e.g. `FRA`, `AMS`, `IAD`).

During **Phase 2**, you see `Phase 2: Proxy validation — X / Y`. Results update as each endpoint is validated.

Click **Stop** at any time to cancel — partial results are kept.

### Step 9 — Export or Push Results

Once the scan completes:

- **Copy All** — copies all `ip:port` pairs to your clipboard
- **Copy Selected** — copies only the checked rows
- **Export Configs** — generates VLESS/Trojan share URLs with working endpoints substituted in
- **Push to Replacer** — sends all working endpoints directly to the IP Replacer tab

---

## IP Generation Details

IPs are generated from **25 IPv4 CIDR ranges** and **91 IPv6 CIDR ranges** sourced from Cloudflare's official AS13335 ranges.

IPv4 uses **weighted random selection** — larger ranges (e.g. `/12`) get proportionally more hits than smaller ranges (e.g. `/24`). This ensures even coverage across Cloudflare's address space.

IPv6 uses **uniform random selection** from all 91 CIDRs.

Each generated IP is unique — duplicates are skipped.

---

## Technical Notes

- **Phase 1:** Uses `net.DialTimeout` with a 3-second timeout, configurable concurrent goroutines (default 500), semaphore-limited
- **Phase 2:** Each endpoint gets a dedicated xray process with a unique SOCKS5 port. Configurable concurrent validations (default 12). 5-second test timeout.
- **Validation:** xray connects to the endpoint, then the app sends an HTTP request through the SOCKS5 proxy to `www.gstatic.com/generate_204`. HTTP 204 or 200 counts as success.
- **Colo detection:** After Phase 1, each responsive IP is probed at `/cdn-cgi/trace` to identify the Cloudflare data center.
- **TCP-only:** Phase 1 only checks TCP connectivity. It does not send any data — just establishes and closes the connection.
