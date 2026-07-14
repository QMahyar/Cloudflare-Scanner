# Plan 015: Replace the real-looking private key in sample.conf with an obvious placeholder

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open `sample.conf` and confirm it still contains an
> `[Interface] PrivateKey =` line with a base64-looking value. On mismatch, STOP.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: security
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

`sample.conf` (committed) contains a WireGuard `[Interface] PrivateKey` and a
`[Peer] PublicKey` that look like real base64 key material. If that private key is (or
ever was) a live WARP/WireGuard key, committing it burns it — a committed secret is
compromised even after deletion, so rotation, not just removal, is the correct
remediation. Even if it's a throwaway demo key, a committed key-shaped value trains
readers (and scanners) to expect secrets in the repo and can trip secret-scanners.
Replace it with an unmistakable placeholder so the sample stays useful as a format
example without shipping key-shaped bytes.

**Do not reproduce the key value anywhere** — in this plan, the commit message, or any
report. Refer to it as "the PrivateKey at `sample.conf:2`".

## Current state

- `sample.conf` — a WireGuard config used as a format example. It has:
  - `[Interface] PrivateKey = <base64 value>` at line 2 (the sensitive one).
  - `[Peer] PublicKey = <base64 value>` (public keys are not secret, but replace for consistency).
  - Various `S1/S2/S3`, `Jc`, `H1..H4` "Hogwarts"-style masking keys (community
    convention; leave their structure, they're not secrets).
- The parser that reads such files is `config.go:ParseWarpConfig`; it requires a
  non-empty `PrivateKey`, `PublicKey`, and at least one `Address`. So the placeholder
  must be non-empty and keep the `[Interface]/[Peer]` structure, or `ParseWarpConfig`
  will reject it — but note this file is a *doc sample*, not loaded at runtime by default.
- Check whether any test loads `sample.conf`: `grep -rn "sample.conf" *.go *_test.go`.
  If a test parses it, the placeholder must still satisfy `ParseWarpConfig`'s minimal checks.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Find references | `grep -rn "sample.conf" . --include=*.go --include=*.md` | list of places that mention it |
| Tests | `go test ./...` | ok (unchanged) |
| Confirm no key-shaped value remains | see Step 3 | placeholder text present |

## Scope

**In scope**:
- `sample.conf` — replace the key values with placeholders
- Possibly a one-line note in the sample or `docs/` telling users to paste their own key (optional)

**Out of scope**:
- `config.go` parser — do NOT change parsing.
- Any real user config (e.g. `WARPGermany.conf` is git-ignored — never touch user configs).
- The `S1/S2/S3`/`Jc`/`H1..H4` masking fields' structure.

## Git workflow

- Branch: `advisor/015-sanitize-sample-conf`
- Commit style: conventional commits, e.g. `chore: replace key-shaped values in sample.conf with placeholders`.
- **Commit message must NOT contain the old key.** Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Confirm nothing runtime-critical parses the real key

Run `grep -rn "sample.conf" . --include=*.go --include=*.md`. If a **test** loads and
parses `sample.conf`, note it — the placeholder must keep the file parseable
(non-empty PrivateKey/PublicKey, ≥1 Address). If nothing parses it, you have full freedom.

**Verify**: you know whether a test depends on `sample.conf` parsing.

### Step 2: Replace the key-shaped values with placeholders

Edit `sample.conf`:
- Set the `[Interface] PrivateKey` to an obvious placeholder, e.g.
  `PrivateKey = YOUR_PRIVATE_KEY_HERE` (or `<paste-your-wireguard-private-key>`).
- Set the `[Peer] PublicKey` similarly, e.g. `PublicKey = PEER_PUBLIC_KEY_HERE`.
- Leave `Address`, `DNS`, `MTU`, the `S*/Jc/H*` fields, and `Endpoint` structure intact
  so the file still documents the format.

If Step 1 found a test that parses this file and asserts specific values, either update
that test to the placeholder OR (better) point the test at an inline fixture instead of
the committed sample — keep the change minimal and note which you did.

**Verify**: `sample.conf` no longer contains a base64-key-shaped value on the PrivateKey line.

### Step 3: Verify

- `grep -nE "PrivateKey|PublicKey" sample.conf` shows placeholder text, not base64.
- `go test ./...` → ok.

**Verify**: both hold.

### Step 4: Recommend rotation (report, do not perform)

In your completion report (NOT in the repo), advise the maintainer: **if the removed
private key was ever a live WARP/WireGuard key, rotate it** — deregister that key with
Cloudflare/WARP and generate a new one. A committed key is compromised regardless of
this cleanup. Do not attempt rotation yourself; it's an out-of-band operational action.

**Verify**: report includes the rotation recommendation.

## Test plan

No new tests. If a test parsed `sample.conf`, it must still pass (adjusted to the
placeholder or repointed to a fixture). `go test ./...` green is the gate.

## Done criteria

- [ ] `sample.conf` PrivateKey/PublicKey are obvious placeholders, not key-shaped base64
- [ ] The file still illustrates the WireGuard `.conf` format (structure intact)
- [ ] `go test ./...` passes
- [ ] The commit message contains no key material
- [ ] Completion report recommends rotating the key if it was ever live
- [ ] Only `sample.conf` (and possibly one test) modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- `sample.conf` has already been sanitized (no key-shaped value) — nothing to do; report and close.
- A runtime (non-test) code path loads `sample.conf` and requires the specific key —
  unlikely (it's a doc sample), but if so, STOP and report before changing it.

## Maintenance notes

- Keep sample/fixture configs placeholder-only going forward; never commit a real key.
- Consider adding a secret-scan (e.g. gitleaks) in CI as a follow-up (out of scope here).
- Reviewer: confirm the diff removed the key and added a placeholder, and that no test
  now asserts the old value.
