# IP Replacer

Takes VLESS/Trojan configs (from a subscription URL or pasted directly), deduplicates them by fingerprint (ignoring IP:port and remark), then generates every combination with a list of clean endpoints.

Perfect for refreshing a stale subscription with fresh IPs from the IP Scanner.

---

## How It Works

```
Input configs (URL or paste)
       │
       ▼
Deduplicate (fingerprint ignores IP:port + remark)
       │
       ▼
You select which configs to use
       │
       ▼
You paste endpoints (one per line)
       │
       ▼
Generate: every config × every endpoint
Each gets remark " @ endpoint"
       │
       ▼
Copy all or download as text file
```

---

## Step-by-Step

### Step 1 — Choose Input Method

Toggle between **Subscription URL** and **Paste Configs**. Only one input is visible at a time.

#### Option A: Subscription URL

1. Enter a subscription URL like `https://example.com/sub?token=xxxx`
2. Click **Fetch**
3. The app downloads the subscription, decodes any base64 content, extracts all `vless://` and `trojan://` URLs, and deduplicates them

#### Option B: Paste Configs

1. Paste raw share URLs into the textarea, one per line (or separated by commas, semicolons, pipes, spaces)
2. Click **Parse**
3. The app extracts valid `vless://` and `trojan://` URLs and ignores everything else

**Delimiter examples** — all of these work:

```
vless://aaa... trojan://bbb...
vless://aaa...,trojan://bbb...
vless://aaa...;trojan://bbb...
vless://aaa...|trojan://bbb...
```

### Step 2 — Review Unique Configs

After fetching or parsing, you see a list of unique config templates. Configs that differ only by IP:port or remark are grouped into one entry.

Each entry shows:
- Protocol (vless / trojan)
- UUID / password
- Original address and port
- SNI, security, network type
- Fingerprint, host, path
- Remark

Check the boxes next to configs you want to include. Use **Select All** / **Deselect All** to toggle quickly.

### Step 3 — Paste Endpoints

Enter one `ip:port` per line in the endpoints textarea. These come from:

- IP Scanner results (export the working endpoints)
- Any other source of working proxy endpoints
- Manually entered addresses

### Step 4 — Generate Configs

Click **Generate Configs**. The app produces every combination of selected configs × endpoints.

Each generated URL gets the endpoint appended to its remark:

```
Original remark: "US Server"
Generated remark: "US Server @ 162.159.90.249:443"
```

### Step 5 — Copy or Download

- **Copy All** — copies all generated URLs to your clipboard, one per line
- **Download** — saves them as a `.txt` file

---

## Deduplication Details

Configs are considered identical (and deduplicated) when they share the same:

- Protocol (vless / trojan)
- UUID
- Encryption
- Security
- SNI
- Fingerprint
- Network (tcp / ws / grpc)
- Host
- Path
- Packet encoding

IP:port and remark are **not** part of the fingerprint, so a config pointing to `1.1.1.1:443` and the same config pointing to `2.2.2.2:8443` are treated as the same template.

---

## Tips

- **Use with IP Scanner:** Run a Clean IP scan first, export the working IPs, then copy them into the IP Replacer endpoints field
- **Large combinations:** If you have 5 configs and 100 endpoints, you get 500 URLs. The app handles this efficiently but your clipboard may have limits — use Download instead of Copy All for very large outputs
- **Fetch timeout:** Subscription URLs have a 30-second HTTP timeout. If your subscription is slow, ensure it responds within that window
