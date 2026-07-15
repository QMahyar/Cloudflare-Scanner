# Roadmap notes (non-binding)

> **Not a commitment.** These are advisor notes from a 2026-07-15 improve audit
> of v3.7.0 (`5945765` / follow-on plan set). They record grounded product
> options so the same ideas are not re-litigated cold. Nothing here is scheduled.

---

## 1. CLI / headless mode

### Evidence

- FAQ already tells users they can run on a headless server by SSH-tunneling the
  random loopback port (`docs/faq.md` — “Can I run it on a headless server?”).
- `startServer` binds `127.0.0.1:0` only (`httpserver.go`); there are no CLI flags
  in `main.go` (`flag` package unused).
- Browser auto-open is hard-wired after listen (`main.go` → `openBrowser`).

### Who benefits

Power users and people who already automate scans over SSH or want a fixed port
for a reverse proxy without scraping the console banner.

### Effort

**S–M** for a thin first cut: `--port N`, `--no-browser` / `--no-open`, maybe
print-only mode. Full “one-shot `scan` subcommand with JSON stdout” is **M–L**.

### Main risk

Growing a second UX surface (CLI + web) that must stay in sync with the HTTP API;
headless users still need the CSRF cookie dance unless state-changing routes get
a local-only exception carefully designed.

### Recommendation

**Do (small).** Ship `--port` and `--no-browser` first so the FAQ path is
first-class. Defer one-shot CLI scan until someone asks with a concrete workflow.

---

## 2. Clash / Sing-box export

### Evidence

- README markets Clash, Sing-box, Nekobox, v2rayN users as the audience.
- Export paths today are share-URL oriented: `ProxyConfig.GenerateShareURL`,
  clean-IP `GenerateExport` (share URLs per endpoint), and Replacer output as
  lines of `vless://` / `trojan://` / `vmess://`.
- No Clash YAML or Sing-box JSON builder exists in-tree.

### Who benefits

Users who paste working IPs into clients that prefer native config formats over
subscription share links.

### Effort

**M** for one well-tested format (e.g. Clash proxy list or a minimal Sing-box
outbound array) next to existing export helpers. **L** if multi-format parity and
every transport (WS/gRPC/REALITY/…) must round-trip perfectly.

### Main risk

Client schema drift (Clash Meta vs open Clash, Sing-box version tags). Incomplete
export that “looks fine” but drops SNI/Host/flow produces dead configs and support
noise.

### Recommendation

**Defer** until a clear request names the target format. Prefer extending
share-URL export quality first (already protocol-neutral as of v3.7.0) over a
half-baked YAML emitter.

---

## 3. Auto-tune concurrency

### Evidence

- `IMPROVEMENTS.md` follow-up status still lists auto-tuning as **not implemented**.
- UI already exposes concurrent workers / phase probe counts; backend clamps
  (`maxEndpointConcurrency`, `maxCleanPhase1Probes`, `maxCleanPhase2Probes`).
- Defaults exist (`DefaultConcurrency` for native vs noise paths).

### Who benefits

Users on slow/mobile links who leave defaults too high, or on fast links who
never open Advanced settings.

### Effort

**M** for a simple heuristic (sample a few dials, scale workers). **L** for
robust cross-platform tuning that does not thrash mid-scan.

### Main risk

Surprising behavior when “Auto” is worse than a known-good manual value; hard to
explain in bilingual UI; flaky on networks where early samples misrepresent later
load.

### Recommendation

**Defer** unless support burden shows defaults are routinely wrong. Manual
controls already shipped; document recommended ranges in FAQ if needed instead.

---

## 4. Persistent / resumable scans

### Evidence

- `frontend/src/lib/stores.js` keeps a rolling **summary** history
  (`cfscanner_history`, max 25) — not full endpoint lists.
- Results can persist in `localStorage` (`cfscanner_results`) for the last run,
  but the process-side job maps are in-memory with a 10-minute TTL after
  completion; restart drops server jobs.
- `IMPROVEMENTS.md` lists “Persistent scan state (resume after restart)” as not
  implemented.

### Who benefits

Users running very large clean-IP scans who lose the window mid-run, or who want
to continue after reboot.

### Effort

**L** — durable job store, cancel/resume semantics, disk growth policy, and UI
for partial progress. This is a different product shape than a one-shot desktop
tool.

### Main risk

Complexity and disk/privacy surface (configs and IPs on disk) for a tool whose
strength is “download, run, done.”

### Recommendation

**Skip** unless multiple users explicitly ask. Prefer stop/partial-results (already
supported) and optional export of intermediate tables over true resume.

---

## Summary table

| Option | Effort | Recommendation |
|--------|--------|----------------|
| CLI flags (`--port`, `--no-browser`) | S–M | Do (small) |
| Clash / Sing-box export | M–L | Defer |
| Auto-tune concurrency | M–L | Defer |
| Resumable scans | L | Skip |

When a future implementer picks one up, write a full build plan with tests; do not
treat this file as a green light to start coding without product confirmation.
