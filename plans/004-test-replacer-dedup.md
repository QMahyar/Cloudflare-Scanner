# Plan 004: Lock the replacer dedup path with tests so a new ProxyConfig field can't silently drop configs

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open the files in "Current state" and confirm the
> quoted lines match live. On mismatch, STOP.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

The IP-Replacer pipeline dedupes parsed configs by a hand-maintained 21-field
fingerprint string (`ConfigFingerprint`). `DeduplicateConfigs` drops any config
whose fingerprint collides. Both functions have **0% test coverage** — no test
references them. The failure mode is silent and nasty: if someone adds a field to
`ProxyConfig` (the app does this regularly — REALITY/flow fields were added) and
forgets to include it in `ConfigFingerprint`, two configs that differ only in that
field collide, and one is silently discarded from replacer output with no error and
no test signal. This plan adds characterization tests that (a) prove distinct
configs survive dedup, (b) prove true duplicates collapse, and (c) include a pair
differing only in a rarely-set field, so the fingerprint's completeness is guarded.

## Current state

- `replacer.go:129-137` — the fingerprint (21 fields):
  ```go
  func ConfigFingerprint(c *ProxyConfig) string {
      return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%t|%s|%s|%s|%s|%d|%s",
          c.Protocol, c.UUID, c.Encryption, c.Security, c.SNI,
          c.Fingerprint, c.Network, c.Host, c.Path, c.PacketEncoding,
          c.Flow, c.PublicKey, c.ShortId, c.SpiderX, c.AllowInsecure,
          c.ALPN, c.HeaderType, c.Mode, c.ServiceName,
          c.MaxEarlyData, c.EarlyDataHeaderName,
      )
  }
  ```
- `replacer.go:139-153` — `DeduplicateConfigs([]*ProxyConfig) []*ProxyConfig`:
  keeps first occurrence per fingerprint, preserves order.
- `replacer.go:15-18` — `ParseRawConfigs(rawText string) []*ProxyConfig` (parses
  whitespace/`,`/`;`/`|`-separated share URLs; useful for building test inputs from URLs).
- No test file references `ConfigFingerprint` / `DeduplicateConfigs` / `ParseRawConfigs`
  (verified: `grep -rn` across `*_test.go` returns nothing).
- The `ProxyConfig` struct is defined in `proxy.go` — read it to see every field
  the fingerprint should (and does) cover.

Convention: tests are table-driven, in `parsers_test.go` / `replacer_name_test.go`,
using standard `testing` (no framework). See `TestShareURLRoundTrip`
(`parsers_test.go:514`) and `replacer_name_test.go` for the local style. Build
`*ProxyConfig` values either as struct literals or by parsing a share URL via
`ParseProxyURL`.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Target test | `go test -run 'TestConfigFingerprint\|TestDeduplicate' ./...` | ok, new tests pass |
| Coverage of replacer.go | `go test -coverprofile=/tmp/c.out ./... && go tool cover -func=/tmp/c.out \| grep replacer.go` | `ConfigFingerprint`/`DeduplicateConfigs` > 0% |
| Full tests | `go test ./...` | ok |

## Scope

**In scope**:
- `replacer_name_test.go` (add the new tests here — it already tests replacer helpers), OR a new `replacer_dedup_test.go` if you prefer isolation. Pick one.

**Out of scope**:
- `replacer.go` — do NOT change the production functions in this plan; this is
  characterization only. (If a test reveals an actual missing field in the
  fingerprint, that's a STOP-and-report, not a silent fix here.)
- Any other pipeline.

## Git workflow

- Branch: `advisor/004-test-replacer-dedup`
- Commit style: conventional commits, e.g. `test: cover replacer ConfigFingerprint and DeduplicateConfigs`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Read the ProxyConfig struct

Open `proxy.go` and locate the `ProxyConfig` struct. Note every field. Confirm each
field the fingerprint uses exists and is spelled as in the excerpt. This tells you
which "rarely-set field" to use for the completeness case (e.g. `ShortId`, `Flow`,
`SpiderX`, `ServiceName`).

**Verify**: you can name all 21 fingerprint inputs against real struct fields.

### Step 2: Write `TestConfigFingerprint`

Add a table-driven test asserting:
- Two configs identical in all 21 fingerprint fields → identical fingerprint.
- Two configs differing in exactly one fingerprint field (test several: `UUID`,
  `Flow`, `ShortId`, `ServiceName`, `Path`) → different fingerprints.
- `AllowInsecure` true vs false → different fingerprints (it's the one `%t`).

Shape:
```go
func TestConfigFingerprint(t *testing.T) {
    base := &ProxyConfig{Protocol: "vless", UUID: "u1", Security: "reality",
        PublicKey: "pk", ShortId: "sid", Flow: "xtls-rprx-vision", Network: "grpc",
        ServiceName: "svc"}
    same := *base
    if ConfigFingerprint(base) != ConfigFingerprint(&same) {
        t.Fatal("identical configs must share a fingerprint")
    }
    diff := *base
    diff.ShortId = "sid2"
    if ConfigFingerprint(base) == ConfigFingerprint(&diff) {
        t.Fatal("configs differing in ShortId must not collide")
    }
    // ...repeat for Flow, ServiceName, Path, AllowInsecure
}
```

**Verify**: `go test -run TestConfigFingerprint ./...` → ok.

### Step 3: Write `TestDeduplicateConfigs`

Assert:
- A slice with a true duplicate (two configs with the same fingerprint) collapses
  to one, preserving first-occurrence order.
- A slice of all-distinct configs (differing only in a rarely-set field like `Flow`)
  is returned unchanged in length and order — **this is the regression guard**: it
  fails if a future change drops that field from the fingerprint.
- Empty input → empty output (no panic).

Shape:
```go
func TestDeduplicateConfigs(t *testing.T) {
    a := &ProxyConfig{Protocol: "vless", UUID: "u", Flow: "flow-a"}
    b := &ProxyConfig{Protocol: "vless", UUID: "u", Flow: "flow-b"} // differs only in Flow
    dupOfA := &ProxyConfig{Protocol: "vless", UUID: "u", Flow: "flow-a"}
    got := DeduplicateConfigs([]*ProxyConfig{a, b, dupOfA})
    if len(got) != 2 {
        t.Fatalf("want 2 distinct (a,b), got %d — Flow may be missing from the fingerprint", len(got))
    }
    if got[0] != a || got[1] != b {
        t.Fatal("dedup must preserve first-occurrence order")
    }
}
```

**Verify**: `go test -run TestDeduplicate ./...` → ok.

### Step 4: Confirm coverage moved off zero

Run the coverage command from the table and confirm `ConfigFingerprint` and
`DeduplicateConfigs` are no longer 0.0%.

**Verify**: coverage grep shows both > 0%.

## Test plan

Covered by Steps 2-3. Cases: identical→same fp; single-field-difference→different fp
(≥5 fields incl. the `%t` bool); true duplicate collapses; distinct-by-rare-field
survives (regression guard); empty input safe. Pattern: `TestShareURLRoundTrip`.
Verification: `go test ./...` → all pass, new tests included.

## Done criteria

- [ ] `TestConfigFingerprint` and `TestDeduplicateConfigs` exist and pass
- [ ] At least one case guards a distinct-by-rarely-set-field pair against collapse
- [ ] Coverage of `ConfigFingerprint`/`DeduplicateConfigs` > 0%
- [ ] `go test ./...` passes
- [ ] `replacer.go` unchanged (`git status` shows only the test file)
- [ ] `plans/README.md` status row updated

## STOP conditions

- A test you wrote to assert "distinct configs survive" FAILS — that means the
  fingerprint is already missing a field (a real bug). STOP and report the field;
  do not "fix" it by weakening the test.
- The `ProxyConfig` field names differ materially from the fingerprint excerpt (drift).

## Maintenance notes

- When a field is added to `ProxyConfig`, the maintainer must add it to
  `ConfigFingerprint` AND add a distinct-by-that-field case here. Note this in the PR.
- Reviewer: the point of Step 3's distinct-by-`Flow` case is to fail loudly on that
  omission — confirm it's asserting length 2, not 1.
