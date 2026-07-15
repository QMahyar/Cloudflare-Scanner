# Plan 009: Fix Trojan/VMess xray outbound JSON shape

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 380c55e..HEAD -- proxy_xray.go xray_batch_test.go parsers_test.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: bug
- **Planned at**: commit `380c55e`, 2026-07-15

## Why this matters

Clean-IP Phase 2 and any path that builds xray outbounds from `ProxyConfig`
claim Trojan/VMess support (export is protocol-neutral), but
`buildOutboundSettings` emits **flat** Trojan/VMess JSON that xray-core
rejects (`0 Trojan server configured` / `0 VMess receiver configured`).
VLESS works because it already uses `vnext`. Users with Trojan/VMess configs
see Phase 2 fail for every IP even when the edge is fine.

Verified against local xray `v1.8.24` via `xray run -test -c <config>`:
nested `servers` (Trojan) and `vnext` (VMess) load; flat shapes do not.

## Current state

- `proxy_xray.go` — outbound/stream builders + `BuildXrayJSONBatch`
- `xray_batch_test.go` — batch shape tests (VLESS only today)
- `parsers_test.go` — proxy parse + some BuildXrayJSONBatch tests

Current broken types and emission (`proxy_xray.go`):

```go
type TrojanOutboundSettings struct {
 Address  string `json:"address"`
 Port     int    `json:"port"`
 Password string `json:"password"`
 // ...
}

type VMessOutboundSettings struct {
 Address     string `json:"address"`
 Port        int    `json:"port"`
 ID          string `json:"id"`
 Security    string `json:"security"`
 // ...
}

case "trojan":
 settings, _ := json.Marshal(TrojanOutboundSettings{
  Address:  c.Address,
  Port:     c.Port,
  Password: c.UUID,
 })
case "vmess":
 // same flat shape with ID/Security
```

xray expects Trojan:

```json
{"servers":[{"address":"...","port":443,"password":"..."}]}
```

and VMess:

```json
{"vnext":[{"address":"...","port":443,"users":[{"id":"...","security":"auto","alterId":0}]}]}
```

Conventions: table-driven tests in `*_test.go`; package `main`; no new deps.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Tests | `go test ./...` | all pass |
| Vet | `go vet ./...` | no issues |
| Format | `gofmt -l proxy_xray.go xray_batch_test.go` | empty |
| Targeted | `go test -run 'TestBatchConfigProxy\|TestBuildXrayJSON\|TestTrojan\|TestVMess' .` | pass |

Optional if `xray`/`xray.exe` sits next to the repo root: write a temp batch
config via a small test and `xray run -test -c <path>` → exit 0 / "Configuration OK".

## Scope

**In scope**:

- `proxy_xray.go`
- `xray_batch_test.go` and/or `parsers_test.go` (new characterization tests)

**Out of scope**:

- Share-URL generation (`proxy_share.go`) — already correct
- Live Phase-2 network probes
- xray-core version bump
- Frontend

## Git workflow

- Branch: `advisor/009-fix-trojan-vmess-xray-outbound`
- Commit style (from recent log): `fix: use servers/vnext for trojan/vmess xray outbounds`
- Do NOT push or open a PR unless asked.

## Steps

### Step 1: Reshape Go types to match xray outbound settings

In `proxy_xray.go`, replace the flat Trojan/VMess settings structs with nested
shapes used by xray-core:

```go
type TrojanServer struct {
 Address  string `json:"address"`
 Port     int    `json:"port"`
 Password string `json:"password"`
 Email    string `json:"email,omitempty"`
 Level    int    `json:"level,omitempty"`
}

type TrojanOutboundSettings struct {
 Servers []TrojanServer `json:"servers"`
}

type VMessUser struct {
 ID       string `json:"id"`
 Security string `json:"security"`
 AlterId  int    `json:"alterId"`
 Level    int    `json:"level,omitempty"`
}

type VMessServer struct {
 Address string      `json:"address"`
 Port    int         `json:"port"`
 Users   []VMessUser `json:"users"`
}

type VMessOutboundSettings struct {
 VNext []VMessServer `json:"vnext"`
}
```

Update `buildOutboundSettings` cases:

```go
case "trojan":
 settings, _ := json.Marshal(TrojanOutboundSettings{
  Servers: []TrojanServer{{
   Address:  c.Address,
   Port:     c.Port,
   Password: c.UUID,
  }},
 })
 return settings
case "vmess":
 sec := c.Encryption
 if sec == "" || sec == "none" {
  sec = "auto"
 }
 settings, _ := json.Marshal(VMessOutboundSettings{
  VNext: []VMessServer{{
   Address: c.Address,
   Port:    c.Port,
   Users: []VMessUser{{
    ID:       c.UUID,
    Security: sec,
    AlterId:  0,
   }},
  }},
 })
 return settings
```

Leave VLESS unchanged. Do not change stream settings.

**Verify**: `gofmt -w proxy_xray.go` then `go test -c -o NUL .` (or `go test -c -o /dev/null .` on Unix) → builds.

### Step 2: Characterization tests for Trojan and VMess batch configs

Add tests modeled after `TestBatchConfigProxy` in `xray_batch_test.go`:

1. `TestBatchConfigProxyTrojan` — `Protocol: "trojan"`, UUID as password, TLS SNI set; `BuildXrayJSONBatch` one endpoint; unmarshal outbound settings and assert `servers[0].address/port/password`.
2. `TestBatchConfigProxyVMess` — `Protocol: "vmess"`, assert `vnext[0].users[0].id` and `security` (defaulting empty encryption → `auto`).

If unmarshaling into the new structs is awkward from raw JSON, decode into
`map[string]json.RawMessage` / nested maps and assert keys exist.

Optional: if `xray.exe` or `xray` exists beside the binary/repo, add a short
test that skips when missing: write the batch config and run
`exec.Command(xrayPath, "run", "-test", "-c", configPath)` expecting exit 0.
Do **not** fail CI when xray is absent — use `t.Skip`.

**Verify**: `go test -run 'TestBatchConfigProxy' .` → pass (including new cases).

### Step 3: Full suite

**Verify**:

- `go test ./...` → all pass
- `go vet ./...` → clean
- `gofmt -l proxy_xray.go xray_batch_test.go` → empty

## Test plan

- New: Trojan batch settings nested under `servers`
- New: VMess batch settings nested under `vnext` with `alterId: 0`
- Regression: existing `TestBatchConfigProxy` (VLESS) still passes
- Pattern: `xray_batch_test.go` `TestBatchConfigProxy`

## Done criteria

- [ ] `buildOutboundSettings` for trojan/vmess emits nested xray-compatible JSON
- [ ] New tests cover Trojan + VMess batch outbounds and pass
- [ ] `go test ./...` and `go vet ./...` pass
- [ ] No files outside scope modified
- [ ] `plans/README.md` status row updated (if you maintain the index)

## STOP conditions

- Live `proxy_xray.go` no longer has flat Trojan/VMess structs (already fixed).
- Fix appears to require changing `proxy_share.go` or live network code.
- A verification fails twice after a reasonable fix.
- You cannot build because `ui/dist` embed is missing — run
  `cd frontend && npm ci && npm run build` once, or only run `go test`
  (tests do not need embed if packages compile; if embed blocks, build UI).

## Maintenance notes

- Any future protocol (e.g. shadowsocks) must use xray's documented outbound
  settings shape, not a flat invent-your-own struct.
- Reviewers: confirm VLESS path untouched; confirm alterId stays 0 unless
  product later needs legacy VMess.
- Deferred: full `xray run -test` in CI (needs bundled binary in test env).
