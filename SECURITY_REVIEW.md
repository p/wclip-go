# Security review — 2026-05-14

Snapshot review of `src/`, `debian/`, and `Dockerfile` at commit
`91edb64`. Severity = practical impact for a typical deployment
(deb-installed service or behind a reverse proxy), not paranoid
worst-case. Treat this file as a working TODO list; tick items as
they are fixed and link to the commit that fixed each.

Suggested priority order to tackle: **C1, C2, H2, M2, M1, L8, M4.**

---

## Critical

### [ ] C1. Unbounded request body — trivial OOM

**Where:** `src/server.go` in `handler.set`:

```go
content, err := ioutil.ReadAll(c.Request.Body)
```

No size cap. Any client able to reach a writable endpoint can PUT a
body larger than RAM and crash the process. With auth disabled (the
default), this is one `curl --data-binary @/dev/zero` away.

**Fix sketch:**

```go
const defaultMaxBody = 16 << 20 // 16 MiB
c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBody)
content, err := io.ReadAll(c.Request.Body)
if err != nil {
    // distinguish *http.MaxBytesError -> 413, other -> 500
}
```

Expose a `MAX_BODY` env var (bytes, or with a `K`/`M`/`G` suffix) with
a sensible default. Add a 413 test case.

---

### [ ] C2. `/etc/default/wclip` is world-readable but may contain `HTTP_PASSWORD`

**Where:** `debian/wclip.default` (installed by debhelper at mode
0644). The same file documents and invites `HTTP_PASSWORD=...`. Any
local user can `cat` it.

**Fix sketch:** install the conffile mode 0640 root:wclip. Either
override perms in `debian/rules`:

```make
override_dh_fixperms:
	dh_fixperms
	chown root:wclip debian/wclip/etc/default/wclip
	chmod 0640       debian/wclip/etc/default/wclip
```

…or chown/chmod in `postinst configure` (the `wclip` group is created
earlier in the same hook, so by-name chown is safe). Add a comment to
the file itself: "may contain a password — do not loosen perms."

---

## High

### [ ] H1. No rate / connection / key-count limits = easy disk-fill

With the bolt backend and no auth, an attacker can spam thousands of
PUTs to arbitrary paths until the partition under `/var/lib/wclip` is
full. bbolt files only grow; deletions don't shrink the file.

**Fixes (pick any combination):**

- Document loudly in README that a rate-limiting reverse proxy is
  required for any non-loopback, non-auth deployment.
- Optional: server-side cap on number of keys, or total stored bytes,
  with a clear "store full" 507 response.
- Optional: simple per-IP token bucket (a 50-line dependency-free
  middleware would suffice).

---

### [ ] H2. Stored XSS via `Content-Type` passthrough

`set()` stores the client's `Content-Type` as-is; `get()` echoes it.
A client can PUT `text/html` containing `<script>` and have it served
back from a wclip origin. With CORS `Allow-Origin: *` and no auth (the
default), anyone reachable can plant payloads.

This is somewhat intentional ("PUT a body, GET it back"), so a hard
fix changes the product. Defense-in-depth instead:

**Fix sketch (cheap, no behavior change for documented use cases):**

- Always set `X-Content-Type-Options: nosniff` on responses.
- Consider an env-gated `Content-Security-Policy: default-src 'none'; sandbox`
  on stored-content responses (would break legitimate HTML viewing, so
  make it opt-in/out).
- README: "never host wclip on the same site / eTLD+1 as anything that
  shares cookies with users you care about."

---

### [ ] H3. Auth middleware vs unregistered methods

`gin.BasicAuth` runs before route handlers, but un-registered methods
(`OPTIONS`, `DELETE`, `HEAD`) currently 404 *before* middleware fires
(gin default `HandleMethodNotAllowed=false`). Not exploitable today,
but a trap if someone later adds `DELETE`.

**Fix sketch:** add `router.NoRoute` / `router.NoMethod` handlers that
`c.AbortWithStatus(404)` *after* `gin.BasicAuth` has had a chance to
challenge. Low priority — mainly a future-proofing note for
contributors. Worth a one-line comment near `newRouter`.

---

## Medium

### [ ] M1. 5xx responses leak operator-controlled error strings

**Where:** `src/server.go`:

```go
c.String(500, "Error saving: "+err.Error())
c.String(500, "Error reading request: "+err.Error())
```

bolt errors can include `DB_PATH`. Operator-controlled, so low real
risk, but the convention is to log internally and return generic to
clients:

```go
log.Printf("save %q: %v", path, err)
c.String(500, "internal error")
```

---

### [ ] M2. `DEBUG=` truthy parsing is too permissive

`DEBUG=0`, `DEBUG=false`, `DEBUG=no` currently enable gin debug
logging (any non-empty value triggers it). In debug mode gin logs full
request lines including paths, which can contain secrets (e.g.
`PUT /api-key-abc123`).

**Fix sketch:**

```go
if v, _ := strconv.ParseBool(os.Getenv("DEBUG")); !v {
    gin.SetMode(gin.ReleaseMode)
}
```

Operator footgun, not user-exploitable. Cheap fix.

---

### [ ] M3. `defer store.Close()` is decorative

Every non-trivial exit path is `log.Fatal*` → `os.Exit` → defers
skipped. bbolt is crash-safe so no real damage, but the defer
mis-suggests "always runs". If a future backend buffers (e.g. remote
KV), this becomes a real bug.

**Fix sketch:** refactor `main` into `run() error` + small `main`:

```go
func main() {
    if err := run(); err != nil {
        log.Print(err)
        os.Exit(1)
    }
}
```

…or remove the misleading defer until a backend actually needs it.

---

### [ ] M4. systemd unit could be tighter

`debian/wclip.service` is already strong. Easy additions for "no-cost"
hardening (wclip needs none of these capabilities):

```ini
CapabilityBoundingSet=
AmbientCapabilities=
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
ProtectClock=true
ProtectHostname=true
ProtectKernelLogs=true
ProtectProc=invisible
ProcSubset=pid
UMask=0077
```

If someone later sets `PORT<1024` they'll need
`AmbientCapabilities=CAP_NET_BIND_SERVICE` and a matching bounding
set; flag this in `debian/wclip.default`.

---

### [ ] M5. CORS preflight `OPTIONS` is missing

`OPTIONS` isn't registered, so cross-origin reads with non-simple
content-types are blocked by browser preflight. The advertised CORS
headers imply we welcome cross-origin reads, but only simple ones
actually work.

**Fix options:**

- Document that only simple cross-origin reads work, **or**
- Add a no-op `OPTIONS` handler returning 204 + the existing CORS
  headers.

Cross-origin *writes* should remain effectively blocked (we don't
advertise `Allow-Methods: PUT,POST` and that's intentional).

---

## Low / informational

### [ ] L1. `Allow-Origin: *` + Basic Auth = browser blocks credentialed XHR

Correct browser behavior; document in README so users don't try
`fetch(..., {credentials: 'include'})` and wonder why.

### [ ] L2. bbolt has no at-rest encryption

Relies on filesystem perms (0750 data dir, 0600 db file). Anyone with
root or the `wclip` user reads everything. Standard; mention if it
matters for the threat model.

### [ ] L3. In-memory store can swap

No `mlock`. Sensitive clips could land in swap under memory pressure.
Out of scope for normal use; footnote only.

### [ ] L4. `version` flag opens nothing — ✅ already good

`--version` returns before any port-open or DB-open side effect.
Verified, no action.

### [ ] L5. Auth comparison is constant-time — ✅ already good

`gin.BasicAuth` uses `subtle.ConstantTimeCompare`. Verified, no
action.

### [ ] L6. No symlink-attack window on bbolt open — ✅ already good

Data dir 0750 wclip:wclip, db file created 0600, only writer is the
service itself. Verified, no action.

### [ ] L7. Dependency hygiene

- gin 1.10.0 — current, no open CVEs known.
- bbolt 1.3.10 — current.
- `go 1.21` in `go.mod`; build toolchain is `GOTOOLCHAIN=local` so
  current Go is used. Consider bumping the `go` directive to `1.22`
  to force a newer minimum.
- Add `govulncheck` (and Dependabot when CI exists).

### [ ] L8. `ioutil` is deprecated

Cosmetic. Replace `ioutil.ReadAll` with `io.ReadAll` (Go 1.16+) across
`server.go` and `server_test.go`. Do it in one sweep, not piecemeal
(AGENTS.md rule).

---

## What's already done well (no action needed)

- Auth applied uniformly to all handlers via `newRouter`, including
  reads, and asserted by `TestAuth_AppliesToWrites` +
  `TestAuth_MissingCredentials_401`.
- Safe defaults: loopback-only bind, auth-off with a TLS-proxy
  pointer, deb leaves `BIND` unset so installs are loopback-only.
- Bolt schema minimal and predictable (`content:<path>` /
  `content-type:<path>` in bucket `wclip`), no cross-tenant state.
- No `unsafe`, no reflection-based parsing, no shell-out, no SQL.
- systemd unit hardened with the common `Protect*=`,
  `MemoryDenyWriteExecute=`, `RestrictAddressFamilies=`, and a
  narrow `ReadWritePaths=`.
- Multi-listener startup is fail-fast for explicit `BIND` — operators
  never silently get fewer listeners than they asked for. Default
  loopback mode is best-effort by design.
- `gin.Recovery()` in the middleware chain — handler panics don't
  tear down the listener.
- Conffile lifecycle is correct: `postinst` creates user/group
  idempotently, `postrm purge` cleans up, defaults file survives
  upgrades.

---

## How to use this document

- Each finding has a `[ ]` checkbox. Tick to `[x]` and link the fix
  commit (or PR) inline when addressed.
- Per AGENTS.md, **fixes go in feature/fix commits, not bundled with
  a release.** Don't fold a `debian/changelog` bump into any of these
  fixes.
- When all critical and high items are clear, re-run a review and
  drop a fresh `SECURITY_REVIEW.md` (replace this one or rotate to
  `docs/security-review-YYYY-MM-DD.md`).
