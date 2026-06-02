# Cloudflare Scanner — Documentation

> **Start here** → [README.md](../README.md) for Quick Start, per-OS install guides, and the full 1-to-100 workflow.
>
> [نسخه فارسی](fa/index.md)

---

## Reading Path

For the best experience, follow this path in order:

| # | Document | What you'll learn |
|---|----------|-------------------|
| 0 | **[README.md](../README.md)** | Quick Start, per-OS setup, TOC, 1-to-100 workflow |
| 1 | [Getting Started](getting-started.md) | First-use workflow after the app is running |
| 2 | [Endpoint Scanner](endpoint-scanner.md) | Full walkthrough of scanning Warp endpoints |
| 3 | [IP Scanner](ip-scanner.md) | Two-phase clean IP scanning with VLESS validation |
| 4 | [IP Replacer](ip-replacer.md) | Batch IP replacement in subscription configs |
| 5 | [FAQ](faq.md) | Troubleshooting and common questions |

## Tool Overview

```
┌────────────────────────────────────────────────────────────┐
│                     Cloudflare Scanner                     │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  Endpoint Scanner    ──>   Pick a fast Warp endpoint       │
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

## Also See

- [BUILD.md](../BUILD.md) — Build from source, project structure, architecture

---

**Tip:** If you're new, just follow steps 0→1→2. That's all you need for a working Warp endpoint.
