# Plan 008: Extend share-URL round-trip tests to REALITY/xTLS and gRPC/kcp transports

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open the files in "Current state" and confirm the
> quoted lines match live. On mismatch, STOP.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW (tests only)
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

`GenerateShareURL` must stay round-trip-consistent with `ParseProxyURL` (CLAUDE.md
states this explicitly). The existing `TestShareURLRoundTrip` only covers `ws`+`tls`
vless, trojan, and vmess. The most fragile fields — REALITY (`pbk`/`sid`/`spx`),
`flow` (xTLS Vision), and gRPC `serviceName` — have **no** round-trip case, and it
shows in coverage (`buildOutboundSettings` 25%, `buildStreamSettings` 47%). A
regression that drops or mangles a REALITY/flow/gRPC field (the same class of bug
that commit `b49c345` fixed for the bare-`/` path) would ship untested. Adding these
cases locks the round-trip for the transports real users rely on.

## Current state

- `parsers_test.go:509-567` — `TestShareURLRoundTrip`. It builds a `cases []string`
  of share URLs, then for each: `ParseProxyURL` → `GenerateShareURL` →
  `ParseProxyURL`, and asserts `Protocol/Address/Port/UUID/Security/Network/SNI/Path/Encryption`
  match, plus vless carries `encryption=none`. Current cases (excerpt):
  ```go
  cases := []string{
      "vless://uuid-1234@1.2.3.4:443?security=tls&sni=example.com&fp=chrome&type=ws&host=example.com&path=/ws#remark",
      "vless://5391f0cd-...@172.67.155.31:443?encryption=none&security=tls&sni=...&type=ws&host=...&path=/&packetEncoding=xudp#Mahyar-2-443",
      "trojan://password123@192.168.1.1:8443?security=tls&sni=host.example",
      "vmess://" + base64.RawURLEncoding.EncodeToString([]byte(vmessPayload)),
  }
  ```
- `proxy.go` — `ProxyConfig` fields relevant here: `Security` (`reality`/`xtls`/`tls`),
  `PublicKey` (REALITY `pbk`), `ShortId` (`sid`), `SpiderX` (`spx`), `Flow`,
  `Network` (`grpc`/`kcp`/`ws`/...), `ServiceName` (gRPC), `HeaderType`, `Mode`.
  Read `ParseProxyURL` (`proxy.go:197`) to confirm the exact query-param names it
  parses for these (e.g. `pbk`, `sid`, `spx`, `serviceName`/`servicename`, `flow`).
  Use the SAME param spellings the parser expects when writing test URLs.

Convention: pure table-driven tests, standard `testing`. `base64` is already imported
in `parsers_test.go` (the vmess case uses it).

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Target test | `go test -run TestShareURLRoundTrip ./...` | ok |
| Coverage | `go test -coverprofile=/tmp/c.out ./... && go tool cover -func=/tmp/c.out \| grep -E 'buildOutboundSettings\|buildStreamSettings'` | both higher than 25%/47% |
| Full tests | `go test ./...` | ok |

## Scope

**In scope**:
- `parsers_test.go` — add REALITY and gRPC (and optionally kcp) cases + field assertions

**Out of scope**:
- `proxy.go` — do NOT modify the parser or generator here. If a round-trip case
  FAILS, that's a real bug → STOP and report (do not adjust the test to hide it).

## Git workflow

- Branch: `advisor/008-test-reality-grpc-roundtrip`
- Commit style: conventional commits, e.g. `test: round-trip REALITY/gRPC share URLs`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Confirm the parser's param names

Read `ParseProxyURL` (`proxy.go:197+`) and note the exact query keys it reads for:
REALITY public key, short id, spiderX, flow, gRPC service name, network/type. Write
your test URLs using those exact keys. (Common xray convention: `security=reality`,
`pbk=`, `sid=`, `spx=`, `flow=`, `type=grpc`, `serviceName=`.)

**Verify**: you can point to the parse site for each field.

### Step 2: Add a REALITY (xTLS Vision) case + assertions

Add to the `cases` slice a vless REALITY URL, e.g.:
```
vless://<uuid>@1.2.3.4:443?encryption=none&security=reality&pbk=<base64pub>&sid=abcd&spx=%2F&flow=xtls-rprx-vision&type=grpc&serviceName=grpcSvc&sni=example.com#reality-case
```
Because the existing loop only compares a fixed field set, add REALITY/gRPC-specific
assertions. Simplest: after the existing per-case comparisons, add targeted checks
guarded by protocol/security so they only run for the reality case:
```go
if c1.Security == "reality" {
    if c1.PublicKey != c2.PublicKey { t.Errorf("reality PublicKey %q -> %q", c1.PublicKey, c2.PublicKey) }
    if c1.ShortId   != c2.ShortId   { t.Errorf("reality ShortId %q -> %q", c1.ShortId, c2.ShortId) }
    if c1.Flow      != c2.Flow      { t.Errorf("Flow %q -> %q", c1.Flow, c2.Flow) }
}
if c1.Network == "grpc" && c1.ServiceName != c2.ServiceName {
    t.Errorf("grpc ServiceName %q -> %q", c1.ServiceName, c2.ServiceName)
}
```
(If adding fields to a fixed comparison list is cleaner in the actual code, do that
instead — the goal is that these fields are asserted for the reality/grpc case.)

**Verify**: `go test -run TestShareURLRoundTrip ./...` → ok. If it fails, STOP (real bug).

### Step 3: Add a gRPC (non-reality) and optionally a kcp case

Add a `security=tls&type=grpc&serviceName=...` vless URL and (optional) a
`type=kcp&headerType=...` case, with assertions for `ServiceName` / `HeaderType`
respectively. Keep them in the same `cases` slice + guarded-assertion pattern.

**Verify**: `go test -run TestShareURLRoundTrip ./...` → ok.

### Step 4: Confirm coverage improved

Run the coverage command and confirm `buildOutboundSettings`/`buildStreamSettings`
percentages rose above their current 25%/47%.

**Verify**: coverage grep shows increases.

## Test plan

- New round-trip cases: REALITY+flow+grpc (all the fragile fields at once), a plain
  gRPC/tls case, optionally kcp. Assertions cover `PublicKey`/`ShortId`/`Flow`/`ServiceName`
  (and `HeaderType` for kcp) surviving struct→URL→struct.
- Pattern: extend the existing `TestShareURLRoundTrip` table + add guarded assertions.
- Verification: `go test ./...` all pass; coverage of the two builders rises.

## Done criteria

- [ ] `cases` includes a REALITY (with `pbk`/`sid`/`spx`/`flow`/`serviceName`) URL and a gRPC URL
- [ ] Assertions verify `PublicKey`/`ShortId`/`Flow`/`ServiceName` round-trip for those cases
- [ ] `go test -run TestShareURLRoundTrip ./...` passes
- [ ] Coverage of `buildOutboundSettings`/`buildStreamSettings` increased
- [ ] `proxy.go` unchanged (`git status` shows only the test file)
- [ ] `plans/README.md` status row updated

## STOP conditions

- A new round-trip case FAILS — that is a genuine parse/generate bug in `proxy.go`.
  STOP and report the exact field that didn't survive; do NOT modify the test to pass
  or touch `proxy.go` in this plan (a fix is a separate plan).
- The parser uses different query-param names than assumed and no combination
  round-trips — report the actual names you found.

## Maintenance notes

- When a new transport or security mode is added to `proxy.go`, add a round-trip case here.
- Reviewer: verify the assertions are actually reached (guarded by the right
  protocol/security/network) and not silently skipped.
