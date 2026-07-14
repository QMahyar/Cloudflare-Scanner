# Plan 007: Make the native WARP probe honor context so Stop is responsive

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
- **Risk**: LOW-MED (touches the hot probe path; normal timing must be unchanged)
- **Depends on**: none
- **Category**: bug (correctness)
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

The default WARP endpoint-scan path uses a native WireGuard handshake probe
(`warpProber.Probe`) — no xray. `Probe` takes only a `timeout`, not a
`context.Context`, and its UDP read loop runs until the timeout elapses regardless
of cancellation. The scan workers only check `ctx.Done()` *between* endpoints
(`server.go:569`) and `testEndpointAttempts` only *between* attempts
(`scanner.go:83`). So after the user hits Stop, every one of up to `concurrency`
(256) in-flight probes runs to its full timeout first. The default timeout is 6s,
but the UI allows `TimeoutMs` up to 60000 — meaning Stop can hang up to 60 seconds
on the most common scan path. CLAUDE.md requires scan loops to honor `ctx.Done()`;
this path doesn't, deep in the probe. Threading `ctx` into `Probe` makes Stop
near-immediate without changing normal (non-cancelled) timing.

## Current state

- `warp_probe.go:151` — signature:
  ```go
  func (p *warpProber) Probe(endpoint string, timeout time.Duration) (time.Duration, error) {
  ```
- `warp_probe.go:213-269` — the UDP dial + retransmit/read loop:
  ```go
  conn, err := net.DialTimeout("udp", endpoint, timeout)
  // ...
  deadline := time.Now().Add(timeout)
  retransmit := timeout / 3   // clamped to [100ms, 700ms]
  // ...
  buf := make([]byte, 256)
  for {
      now := time.Now()
      if !now.Before(deadline) { return 0, fmt.Errorf("no handshake response within %v", timeout) }
      readUntil := now.Add(retransmit)
      if readUntil.After(deadline) { readUntil = deadline }
      conn.SetReadDeadline(readUntil)
      n, err := conn.Read(buf)
      if err != nil {
          if ne, ok := err.(net.Error); ok && ne.Timeout() && time.Now().Before(deadline) {
              // retransmit + continue
          }
          return 0, err
      }
      // match handshake response / cookie reply → return time.Since(sendTime), nil
  }
  ```
  The loop already wakes every `retransmit` (≤700ms) to resend — that is the natural
  place to also check `ctx`.
- Caller: `scanner.go:142` — `rtt, err := prober.Probe(endpoint, s.Timeout)` inside
  `testEndpointOnce(ctx, endpoint)`. `ctx` is already in scope there.
- Only caller of `Probe` in production is `scanner.go`. There's also
  `warp_probe_test.go` — check whether it calls `Probe` directly (it likely does);
  its calls must be updated to pass a context.

Convention: contexts are threaded as the first parameter (`ctx context.Context`),
matching `testEndpointOnce(ctx, ...)`, `socks204Probe(ctx, ...)`, etc.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Probe tests | `go test -run TestWarp ./...` | ok |
| Full tests | `go test ./...` | ok |

## Scope

**In scope**:
- `warp_probe.go` — add `ctx` to `Probe`, use it to bound dial + read loop
- `scanner.go` — pass `ctx` at the call site
- `warp_probe_test.go` — update `Probe` calls; add a cancellation test

**Out of scope**:
- The handshake crypto (message construction, MAC, key derivation) — do NOT touch.
- Retransmit timing constants (`timeout/3`, 100ms/700ms clamps) — keep as-is for the
  non-cancelled path.
- Worker-pool / attempts logic in `scanner.go` / `server.go` — unchanged.

## Git workflow

- Branch: `advisor/007-ctx-warp-probe`
- Commit style: conventional commits, e.g. `fix: honor context in native WARP probe so Stop is responsive`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Change the signature and dial

Change to:
```go
func (p *warpProber) Probe(ctx context.Context, endpoint string, timeout time.Duration) (time.Duration, error) {
```
Replace `net.DialTimeout("udp", endpoint, timeout)` with a context-aware dial:
```go
var d net.Dialer
dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
defer dialCancel()
conn, err := d.DialContext(dialCtx, "udp", endpoint)
```
(Keep `defer conn.Close()`.)

**Verify**: `go build ./...` will fail until Step 3 (caller not updated) — that's expected; instead run `go vet` after Step 3.

### Step 2: Check ctx inside the read loop

At the top of each `for` iteration (before computing `readUntil`), return promptly on
cancellation:
```go
for {
    if err := ctx.Err(); err != nil {
        return 0, err   // context cancelled/expired — stop retransmitting
    }
    now := time.Now()
    // ... existing body ...
}
```
Optionally also cap `readUntil` by the context deadline if one is set, so a read
never blocks past ctx cancellation:
```go
if dl, ok := ctx.Deadline(); ok && readUntil.After(dl) {
    readUntil = dl
}
```
The loop already wakes at most every ~700ms (retransmit), so a ctx check per
iteration bounds Stop latency to ≤~700ms regardless of the user timeout. Do NOT
otherwise change the retransmit/deadline logic.

**Verify**: (after Step 3) `go vet ./...` → exit 0.

### Step 3: Update the caller

In `scanner.go:142`, pass the context already in scope:
```go
rtt, err := prober.Probe(ctx, endpoint, s.Timeout)
```
Also update the defensive one-shot prober path just above it if it calls `Probe`.

**Verify**: `go vet ./...` → exit 0; `go build -ldflags="-s -w" -o /dev/null .` → exit 0.

### Step 4: Update tests + add a cancellation test

Fix any `Probe(...)` calls in `warp_probe_test.go` to pass a context (use
`context.Background()` for existing cases). Add a test that a probe against an
unreachable/black-hole UDP endpoint with a long timeout (e.g. 30s) returns quickly
(well under the timeout) when the passed context is cancelled shortly after start:
```go
func TestWarpProbeHonorsContext(t *testing.T) {
    p, err := newWarpProber(sampleWarpConfig(t)) // reuse the test's existing config builder
    if err != nil { t.Fatal(err) }
    ctx, cancel := context.WithCancel(context.Background())
    go func() { time.Sleep(200 * time.Millisecond); cancel() }()
    start := time.Now()
    _, perr := p.Probe(ctx, "192.0.2.1:65535", 30*time.Second) // TEST-NET-1, unreachable
    if perr == nil { t.Fatal("expected error on cancelled probe") }
    if elapsed := time.Since(start); elapsed > 3*time.Second {
        t.Fatalf("probe ignored cancellation: took %v (want < 3s)", elapsed)
    }
}
```
Adapt the config construction to whatever helper `warp_probe_test.go` already uses.

**Verify**: `go test -run TestWarp ./...` → ok.

## Test plan

- New `TestWarpProbeHonorsContext`: a cancelled context aborts an in-flight probe
  far sooner than its timeout.
- Existing WARP probe tests still pass (updated to pass `context.Background()`).
- Pattern: `warp_probe_test.go`'s existing structure.
- Verification: `go test ./...` → all pass.

## Done criteria

- [ ] `Probe` takes `ctx context.Context` as its first parameter and uses `DialContext`
- [ ] The read loop returns on `ctx.Err()` each iteration (≤~700ms Stop latency)
- [ ] `scanner.go` call site passes `ctx`
- [ ] `TestWarpProbeHonorsContext` exists and asserts < 3s abort on a 30s-timeout probe
- [ ] `go vet` + `go build` + `go test ./...` green
- [ ] Non-cancelled timing constants unchanged
- [ ] Only in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- `Probe`'s loop or signature differs materially from the excerpt (drift).
- Adding the ctx check changes a passing WARP test's timing/behavior in a way that
  isn't just "faster on cancel" — STOP and report (you may have altered the happy path).
- The test environment can't bind UDP at all and the new test can't run meaningfully —
  keep the code change, mark the test skipped with a clear reason, and report.

## Maintenance notes

- The same pattern (ctx-aware read loop) should be applied to any future raw-socket
  probe added to this file.
- Reviewer: confirm the retransmit cadence and the "no handshake response" timeout
  message are unchanged for the normal path.
- Test network note: this repo's environment sometimes blocks WARP UDP entirely
  (documented); the cancellation test uses TEST-NET-1 which fails fast either way, so
  it's environment-independent for the *cancel* assertion.
