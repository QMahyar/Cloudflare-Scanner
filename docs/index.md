# Cloudflare Scanner — Documentation

Welcome! This documentation covers everything you need to know about the Cloudflare Scanner app.

> [نسخه فارسی](fa/index.md)

---

## Getting Started

- [Getting Started Guide](getting-started.md) — Download, extract, run, and first steps on any platform

## Tool Guides

Each guide walks through one tab step by step, with screenshots of every field and button:

| Guide | What it covers |
|---|---|
| [Endpoint Scanner](endpoint-scanner.md) | Finding working Warp endpoints with your WireGuard config |
| [IP Scanner (Clean IP)](ip-scanner.md) | Scanning Cloudflare IP ranges for clean proxies |
| [IP Replacer](ip-replacer.md) | Replacing IP:port in subscription/VLESS configs with endpoints |

## How the Tools Work Together

```
Endpoint Scanner ──>  Pick a fast Warp endpoint
                           │
IP Scanner       ──>  Find clean Cloudflare proxy IPs
                           │
IP Replacer      ──>  Replace IP:port in configs with clean IPs
```

## FAQ

- [Frequently Asked Questions](faq.md)

## Build & Development

See [BUILD.md](../BUILD.md) for building from source, project structure, and architecture.
