# Plan 009: Reject control characters in the apply-endpoint value so it can't inject lines into a .conf

> **Executor instructions**: Follow step by step. Run every verification command
> and confirm the expected result before the next step. On any "STOP condition",
> stop and report. When done, update the row in `plans/README.md`.
>
> **Drift check (run first)**: Written against a WORKING TREE with uncommitted
> changes on commit `6f7a19c`. Open the files in "Current state" and confirm the
> quoted lines match live. On mismatch, STOP.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: security
- **Planned at**: commit `6f7a19c`, 2026-07-14 (working tree dirty)

## Why this matters

`handleApplyEndpoint` validates the user-supplied `endpoint` only with
`net.SplitHostPort`, which does NOT reject newlines or other control characters in
the host portion (e.g. `"a\nInjected = x:1"` splits cleanly into host `"a\nInjected = x"`,
port `"1"`). The value is then written verbatim as `"Endpoint = " + endpoint` into a
WireGuard `.conf` (`server.go:1109`), so a crafted value injects arbitrary extra
config lines into the file. The write is already confined to the app directory and
CSRF-protected (local origin), so this is a low-severity input-validation gap rather
than RCE — but it writes attacker-influenced structure into a config the user later
feeds to a VPN client, and the fix is a two-line guard. Tighten the endpoint contract
to `host:port` with a sane host and a numeric in-range port.

## Current state

- `server.go:1027-1035` — current validation:
  ```go
  endpoint := r.FormValue("endpoint")
  if endpoint == "" {
      jsonError(w, "endpoint required", 400)
      return
  }
  if _, _, err := net.SplitHostPort(endpoint); err != nil {
      jsonError(w, fmt.Sprintf("invalid endpoint format (expected host:port): %v", err), 400)
      return
  }
  ```
- `server.go:1108-1113` — the write that trusts it:
  ```go
  if inPeer && strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "endpoint") {
      newLines = append(newLines, "Endpoint = "+endpoint)
      replaced = true
      continue
  }
  ```
- There is an existing `security_test.go` (table-driven) — add the endpoint-validation
  test there or in a sibling test file.
- `net` is already imported in `server.go`.

Convention: validation failures return `jsonError(w, msg, 400)`. Keep messages
user-facing and non-leaky.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Vet | `go vet ./...` | exit 0 |
| Build | `go build -ldflags="-s -w" -o /dev/null .` | exit 0 |
| Target test | `go test -run TestValidateEndpoint ./...` | ok |
| Full tests | `go test ./...` | ok |

## Scope

**In scope**:
- `server.go` — add a strict endpoint validator; call it in `handleApplyEndpoint`
- `security_test.go` (or a new `endpoint_validation_test.go`) — unit-test the validator

**Out of scope**:
- The path-traversal guards (`filepath.Rel`/`filepath.Base`) already in
  `handleApplyEndpoint` — they are correct; do NOT change them.
- The `.conf` rewrite logic beyond swapping in the validated value.
- Any other handler.

## Git workflow

- Branch: `advisor/009-validate-endpoint-control-chars`
- Commit style: conventional commits, e.g. `fix: reject control chars in apply-endpoint value`.
- Do NOT push or open a PR unless instructed.

## Steps

### Step 1: Add a strict validator

Add a small package-level function in `server.go`:
```go
// validateEndpointHostPort enforces a strict host:port for values written into a
// .conf file: a numeric in-range port and a host with no control characters,
// whitespace, or newlines (which could otherwise inject extra config lines).
func validateEndpointHostPort(endpoint string) error {
    host, portStr, err := net.SplitHostPort(endpoint)
    if err != nil {
        return fmt.Errorf("expected host:port: %w", err)
    }
    port, err := strconv.Atoi(portStr)
    if err != nil || port < 1 || port > 65535 {
        return fmt.Errorf("port must be 1-65535")
    }
    if host == "" {
        return fmt.Errorf("host required")
    }
    for _, r := range host {
        if r < 0x20 || r == 0x7f || r == ' ' {
            return fmt.Errorf("host contains an invalid character")
        }
    }
    return nil
}
```
Confirm `strconv` is imported in `server.go` (add it if not).

**Verify**: `go vet ./...` → exit 0.

### Step 2: Use it in `handleApplyEndpoint`

Replace the `net.SplitHostPort`-only check at `server.go:1032` with:
```go
if err := validateEndpointHostPort(endpoint); err != nil {
    jsonError(w, fmt.Sprintf("invalid endpoint: %v", err), 400)
    return
}
```

**Verify**: `go build -ldflags="-s -w" -o /dev/null .` → exit 0.

### Step 3: Unit-test the validator

Add `TestValidateEndpoint` (table-driven) covering:
- valid: `1.2.3.4:8886`, `[2606:4700::1]:443`, `example.com:2053` → nil
- invalid: `"a\nInjected = x:1"`, `"host\t:443"`, `"host :443"` (space),
  `"1.2.3.4:0"`, `"1.2.3.4:70000"`, `"1.2.3.4"` (no port), `":443"` (empty host) → error

Model on the table style in `security_test.go`.

**Verify**: `go test -run TestValidateEndpoint ./...` → ok.

## Test plan

- `TestValidateEndpoint`: valid IPv4/IPv6/hostname host:port pass; newline/tab/space
  in host, out-of-range/non-numeric port, missing host/port all rejected. The newline
  case is the security regression guard.
- Pattern: `security_test.go`.
- Verification: `go test ./...` → all pass.

## Done criteria

- [ ] `validateEndpointHostPort` exists and rejects control chars / bad ports
- [ ] `handleApplyEndpoint` uses it instead of the bare `SplitHostPort` check
- [ ] `TestValidateEndpoint` covers the newline-injection case and passes
- [ ] `go vet` + `go build` + `go test ./...` green
- [ ] Path-traversal guards unchanged
- [ ] Only in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

- The validation block or the `.conf` write differ materially from the excerpts (drift).
- Rejecting spaces/controls breaks a legitimate existing test or documented input
  format (e.g. if hostnames with unusual chars are expected) — report before loosening.

## Maintenance notes

- If the app ever accepts SNI/host values written into configs elsewhere, apply the
  same control-char guard there.
- Reviewer: confirm IPv6 bracket form (`[::1]:443`) still validates (SplitHostPort
  handles the brackets; the host becomes `::1`, which passes the char check).
