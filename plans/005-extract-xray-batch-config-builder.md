# Plan 005: Extract the shared xray batch-config builder (one implementation, two callers)

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
- **Risk**: MED (both callers feed live xray processes; the generated JSON must stay byte-equivalent)
- **Depends on**: none (do BEFORE 010/011 file-splits so those move a single builder, not two)
- **Category**: tech-debt
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

Two functions build a batch xray config and are ~90% identical:
`GenerateConfigBatch` (`xray.go:92`, WARP noise fallback) and `BuildXrayJSONBatch`
(`proxy.go:783`, clean-IP Phase 2). Both produce: a `Log` block pointing at a
per-batch `xray.log`; a SOCKS inbound per endpoint (`in-%d`, `noauth`, `udp:true`);
one outbound per endpoint (`out-%d`); a routing rule wiring each inbound to its
outbound; trailing `direct` (freedom) + `block` (blackhole) outbounds; then
`MarshalIndent` + `WriteFile(config.json, 0600)` under a per-batch temp dir. The
**only** differences are (1) the outbound settings (WireGuard vs
`buildOutboundSettings`/`buildStreamSettings`) and (2) the temp-dir name
(`_xray_work/wgbatch_<port>` vs `_xray_clean/batch_<port>`). Any change to the
config shape (a new default outbound, a log-level tweak, the SOCKS `udp` flag, the
port/tag scheme) must be made in both and can silently drift. Collapsing to one
builder with a per-endpoint outbound closure removes that drift risk.

## Current state

- `xray.go:92-195` — `func (xm *XrayManager) GenerateConfigBatch(endpoints []string, basePort int) (configPath string, ports []int, err error)`.
  - dir: `filepath.Join(os.TempDir(), "_xray_work", fmt.Sprintf("wgbatch_%d", basePort))`
  - per-endpoint outbound: `OutboundConfig{Tag: outTag, Protocol: "wireguard", Settings: wgJSON}`
    where `wgJSON` marshals a `WireGuardSettings` built from `xm.Config` + `xm.Noise`.
- `proxy.go:783-872` — `func (c *ProxyConfig) BuildXrayJSONBatch(endpoints []string, basePort int) (configPath string, ports []int, err error)`.
  - extra guard: returns error if `c.UUID == ""`.
  - dir: `filepath.Join(os.TempDir(), "_xray_clean", fmt.Sprintf("batch_%d", basePort))`
  - per-endpoint outbound: `epCfg := c.WithEndpoint(ep)`, then
    `OutboundConfig{Tag: outTag, Protocol: epCfg.Protocol, Settings: epCfg.buildOutboundSettings()}`
    plus `if ss := epCfg.buildStreamSettings(); ss != nil { ob.StreamSettings = ss }`.
- Shared types live in `xray.go`: `XrayConfig`, `LogConfig`, `InboundConfig`,
  `OutboundConfig`, `RoutingConfig`, `RoutingRule` (lines 16-54).
- Both write `logFile` first (empty), then `config.json`, both `0600`; dirs `0700`.
- Callers:
  - `scanner.go:198-208` — `scanBatchNoise` calls `xm.GenerateConfigBatch(...)` then `defer os.RemoveAll(filepath.Dir(configPath))`.
  - `cleanip.go` (`validateBatchWithXray`, ~line 911) calls `cfg.BuildXrayJSONBatch(...)` then `defer os.RemoveAll(filepath.Dir(configPath))`.

Convention: this is a single Go package `main`; helpers are plain package-level
funcs. Comments are dense and explain "why" (see the port-band comments) — match that.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Tests | `go test ./...` | ok |
| Byte-equivalence check | see Step 2 | identical JSON before/after |

## Scope

**In scope**:
- `xray.go` — add the shared builder; reduce `GenerateConfigBatch` to a thin caller
- `proxy.go` — reduce `BuildXrayJSONBatch` to a thin caller
- A new test file (e.g. `xray_batch_test.go`) for the golden-JSON guard

**Out of scope**:
- `scanner.go` / `cleanip.go` callers — their signatures stay the same; do NOT change them.
- `BatchProbe` and the process lifecycle (`xray.go:213`) — untouched.
- The port-window math and temp-dir *locations* — keep `_xray_work/wgbatch_` and `_xray_clean/batch_` exactly.

## Git workflow

- Branch: `advisor/005-extract-xray-batch-builder`
- Commit style: conventional commits, e.g. `refactor: unify xray batch-config builder`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add the shared builder in `xray.go`

Add a package-level function that owns the entire scaffold and takes the two
things that vary as parameters — a directory name and a per-endpoint outbound
factory:
```go
// buildBatchXrayConfig writes ONE xray config probing every endpoint in the batch:
// a SOCKS inbound + one outbound (from makeOutbound) + a routing rule per endpoint,
// plus trailing direct/block outbounds. dirName is the subdir under os.TempDir()
// (e.g. "_xray_work/wgbatch_<port>"). Returns the config path and per-endpoint
// SOCKS ports aligned to endpoints. Caller owns dir cleanup.
func buildBatchXrayConfig(endpoints []string, basePort int, dirName string,
    makeOutbound func(ep, outTag string) (OutboundConfig, error)) (string, []int, error) {

    if len(endpoints) == 0 {
        return "", nil, fmt.Errorf("no endpoints in batch")
    }
    configDir := filepath.Join(os.TempDir(), dirName)
    if err := os.MkdirAll(configDir, 0700); err != nil {
        return "", nil, fmt.Errorf("cannot create work dir: %w", err)
    }
    logFile := filepath.Join(configDir, "xray.log")
    _ = os.WriteFile(logFile, []byte{}, 0600)

    socksSettings, _ := json.Marshal(map[string]interface{}{"auth": "noauth", "udp": true})

    cfg := XrayConfig{
        Log:       &LogConfig{Access: logFile, Error: logFile, Loglevel: "warning"},
        Inbounds:  make([]InboundConfig, 0, len(endpoints)),
        Outbounds: make([]OutboundConfig, 0, len(endpoints)+2),
        Routing:   &RoutingConfig{DomainStrategy: "AsIs", Rules: make([]RoutingRule, 0, len(endpoints))},
    }
    ports := make([]int, len(endpoints))
    for i, ep := range endpoints {
        port := basePort + i
        ports[i] = port
        inTag := fmt.Sprintf("in-%d", i)
        outTag := fmt.Sprintf("out-%d", i)
        ob, err := makeOutbound(ep, outTag)
        if err != nil {
            return "", nil, err
        }
        cfg.Inbounds = append(cfg.Inbounds, InboundConfig{
            Tag: inTag, Port: port, Listen: "127.0.0.1", Protocol: "socks", Settings: socksSettings,
        })
        cfg.Outbounds = append(cfg.Outbounds, ob)
        cfg.Routing.Rules = append(cfg.Routing.Rules, RoutingRule{
            Type: "field", InboundTag: []string{inTag}, OutboundTag: outTag,
        })
    }
    cfg.Outbounds = append(cfg.Outbounds,
        OutboundConfig{Tag: "direct", Protocol: "freedom"},
        OutboundConfig{Tag: "block", Protocol: "blackhole"})

    configJSON, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return "", nil, fmt.Errorf("marshal config: %w", err)
    }
    configPath := filepath.Join(configDir, "config.json")
    if err := os.WriteFile(configPath, configJSON, 0600); err != nil {
        return "", nil, fmt.Errorf("write config: %w", err)
    }
    return configPath, ports, nil
}
```
Note: the factory returns `error` so `BuildXrayJSONBatch`'s `UUID==""` guard can be
preserved (return the error from the closure on the first endpoint). Confirm the
field ordering in the produced JSON matches the originals — the struct field order
in `XrayConfig`/`OutboundConfig` is what determines JSON key order, and it's shared,
so it will.

**Verify**: `go vet ./...` → exit 0.

### Step 2: Capture golden JSON BEFORE rewiring callers

Before you change the two callers, add a test that snapshots the exact JSON each
currently produces, so you can prove byte-equivalence after. In `xray_batch_test.go`:
- Build a fixed `XrayManager` (synthetic `WarpConfig`) and call `GenerateConfigBatch`
  with a fixed endpoint list + basePort; read the written `config.json`; store the bytes.
- Build a fixed `ProxyConfig` (vless with ws+tls, and a second reality case) and call
  `BuildXrayJSONBatch` similarly; store the bytes.
- Assert the JSON parses and contains the expected inbound/outbound/rule counts
  (`len(endpoints)` inbounds, `len(endpoints)+2` outbounds).

Run it, copy the produced JSON into the test as golden strings (or assert structural
invariants if exact bytes are unwieldy). This test must PASS against the current code first.

**Verify**: `go test -run TestBatchConfig ./...` → ok (against unmodified builders).

### Step 3: Rewire `GenerateConfigBatch` to the shared builder

Replace the body of `GenerateConfigBatch` with a call to `buildBatchXrayConfig`,
supplying `dirName = fmt.Sprintf("_xray_work/wgbatch_%d", basePort)` and a closure
that builds the WireGuard `OutboundConfig` (moving the existing `WireGuardSettings`
+ noise construction into the closure). Keep the method signature identical.

**Verify**: `go test -run TestBatchConfig ./...` → still ok (golden unchanged).

### Step 4: Rewire `BuildXrayJSONBatch` to the shared builder

Replace its body similarly: keep the `c.UUID == ""` guard up front (or return it from
the closure), `dirName = fmt.Sprintf("_xray_clean/batch_%d", basePort)`, and a closure
doing `epCfg := c.WithEndpoint(ep)` → `OutboundConfig` with `buildOutboundSettings()`
and optional `buildStreamSettings()`.

**Verify**: `go test ./...` → ok; golden JSON identical to Step 2.

### Step 5: Full build + vet

**Verify**: `go build -ldflags="-s -w" -o /dev/null .` → exit 0; `go vet ./...` → exit 0.

## Test plan

- `xray_batch_test.go`: golden/structural assertions on both builders' output
  (inbound/outbound/rule counts; SOCKS settings; presence of `direct`/`block`;
  temp-dir path prefix). The key guarantee: output is unchanged before vs after the refactor.
- Pattern: table/golden style like `parsers_test.go`.
- Verification: `go test ./...` all pass; the golden test proves byte/structural equivalence.

## Done criteria

- [ ] `buildBatchXrayConfig` exists; both `GenerateConfigBatch` and `BuildXrayJSONBatch` delegate to it
- [ ] Method signatures of both callers are unchanged (`scanner.go`/`cleanip.go` untouched)
- [ ] Golden/structural test passes and the JSON is equivalent pre/post refactor
- [ ] `_xray_work/wgbatch_` and `_xray_clean/batch_` dir names preserved exactly
- [ ] `go vet` + `go build` + `go test ./...` all green
- [ ] Only in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- The two builders differ from the excerpts in more than the two documented ways
  (outbound + dir name) — e.g. one has a different SOCKS setting or extra outbound.
  Reconcile before merging; if the difference is load-bearing, STOP and report.
- The golden JSON differs after rewiring (key order, whitespace, or content) — do
  NOT ship; STOP. Byte-equivalence is the safety contract here.
- `WithEndpoint`, `buildOutboundSettings`, or `buildStreamSettings` are missing/renamed (drift).

## Maintenance notes

- New config-shape changes now go in `buildBatchXrayConfig` once — call that out in review.
- Plans 010/011 (file splits) should run AFTER this so they relocate one builder.
- Reviewer: scrutinize that the WARP path's noise entries and the clean path's
  stream settings still land in the outbound exactly as before.
