# Cloudflare Scanner Documentation

> **Start here** → [README.md](../README.md) for Quick Start, download, per-OS install.
>
> [نسخه فارسی](fa/index.md)

---

## Reading Path

| # | Document | What you'll learn |
|---|----------|-------------------|
| 1 | [Getting Started](getting-started.md) | First-use workflow after the app is running |
| 2 | [Endpoint Scanner](endpoint-scanner.md) | Full walkthrough of scanning WARP endpoints |
| 3 | [IP Scanner](ip-scanner.md) | Two-phase clean IP scanning with VLESS validation |
| 4 | [IP Replacer](ip-replacer.md) | Batch IP replacement in subscription configs |
| 5 | [FAQ](faq.md) | Troubleshooting and common questions |
| 6 | [BUILD.md](../BUILD.md) | Build from source, project structure, architecture |

## Tool Overview

```
┌────────────────────────────────────────────────────────────┐
│                     Cloudflare Scanner                     │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Endpoint Scanner    ──>   Pick a fast WARP endpoint       │
│       │                                                    │
│       ▼                                                    │
│  Apply endpoint to your .conf files                        │
│       │                                                    │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─                    │
│       │                                                    │
│  IP Scanner          ──>   Find clean Cloudflare proxy IPs │
│       │                                                    │
│       ▼                                                    │
│  IP Replacer         ──>   Replace IP:port in configs      │
│                            with clean IPs                  │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

**Tip:** New? Start with [Getting Started](getting-started.md) for a working WARP endpoint.
