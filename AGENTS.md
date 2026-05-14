# AGENTS.md

Orientation for coding agents working in this repo. User-facing docs live
in [README.md](README.md); this file covers conventions, invariants, and
gotchas that aren't obvious from the README.

## What this is

A tiny HTTP web-clipboard: `PUT`/`POST` a body to any path, `GET` it
back with the same `Content-Type`. Gin HTTP router, pluggable `Store`
backend (bbolt or in-memory).

## Repo layout

- `src/` — all Go code, single `main` package:
  - `server.go` — handlers, `newRouter()` factory, `main()`, env-var wiring.
  - `store.go` — the `Store` interface (`Get` / `Set` / `Close`).
  - `store_bolt.go` — persistent bbolt-backed store (default).
  - `store_mem.go` — in-memory, mutex-guarded map.
  - `*_test.go` — tests next to each file.
- `Makefile` — `b` (build), `test`, `fmt`, `docker`.
- `Dockerfile` — `scratch`-based image, expects `build/wclip.docker`.
- `debian/` — native Debian package (systemd unit, conffile,
  postinst/postrm, `rules` driven by `dh`).
- `go.mod` — Go 1.21; deps are gin + bbolt.

## Build / test / format

    make            # -> build/wclip
    make test       # go test ./src/...
    make fmt        # go fmt + sed tabs -> 2 spaces
    make docker     # CGO_ENABLED=0 build + docker build

⚠️ Code is intentionally indented with **two spaces**, not tabs.
`make fmt` runs `go fmt` and then `sed -i 's/\t/  /g'`. Don't run
`gofmt` alone — your diff will fight the convention. Match the existing
style in any new `.go` file.

## Versioning

**Canonical source of truth: git tags** of the form `vMAJOR.MINOR.PATCH`
(e.g. `v0.2.1`). The leading `v` is the Go-modules / semver
convention and lets `go install wclip@vX.Y.Z` work if anyone ever
consumes wclip as a module.

`debian/changelog`'s top entry **must match the tag without the `v`**
(e.g. tag `v0.3.0` ↔ changelog `wclip (0.3.0) ...`). This is
required because `debian/rules` uses `dpkg-parsechangelog -SVersion`
as the `.deb`'s package version, and we want the `.deb` and the
tagged Go binary to agree.

### How the version is resolved into the binary

At build time, the `Makefile` derives `VERSION` with a four-step
fallback chain (highest priority first):

1. `git describe --tags --dirty` (strip leading `v`) — used in any
   clone with at least one tag. On the tagged commit returns the
   bare version (e.g. `0.2.1`); between tags returns
   `0.2.1-7-g30ea4f7`; with uncommitted changes appends `-dirty`.
2. `dpkg-parsechangelog -SVersion` — used in Debian source trees
   without git context.
3. `awk` on `debian/changelog` — used in tarballs without `dpkg-dev`.
4. Literal `"dev"` — last-ditch fallback.

`COMMIT` (short SHA) and `DATE` (committer time, not build time —
keeps builds reproducible) come from git. All three are passed via
`-ldflags '-X main.version=... -X main.commit=... -X main.date=...'`.

`debian/rules` continues to use `dpkg-parsechangelog -SVersion` as
authoritative for the `.deb` version and passes the same value into
the binary via ldflags, so deb-packaged and source-built binaries
agree.

### Runtime fallback (no ldflags)

If any of `main.version`, `main.commit`, `main.date` are unset
(e.g. a contributor ran `go build ./src` directly), `src/version.go`
falls back to `runtime/debug.ReadBuildInfo()`:

- `bi.Main.Version` (set by `go install pkg@vX.Y.Z`) fills `version`
  when it's still `"dev"`.
- `vcs.revision` / `vcs.time` / `vcs.modified` from the embedded VCS
  settings fill `commit`, `date`, and the dirty marker.

This means a plain `go build` still produces a binary that knows its
commit and dirty status. Don't bypass this by hardcoding values
elsewhere.

### Release workflow

A release is **two commits and one tag**, all on master, in order:

1. **changelog commit** — adds a new top entry to `debian/changelog`
   with version `X.Y.Z`. **Touches only `debian/changelog`.**
2. **`git tag -a vX.Y.Z -m "Release vX.Y.Z"`** — annotated tag on
   the changelog commit. The tag (sans `v`) must equal the changelog
   version exactly.
3. (Optional, future) push tag to forge; goreleaser or hand-built
   artifacts can key off the tag.

**Release commits and tags stand alone.** Never bundle a
`debian/changelog` version bump with any other change, and never cut
a release as part of doing other work. Releases are an explicit,
separate decision made by a human — if you've just finished a feature
or fix, stop at the feature/fix commit and let the user decide when
(and whether) to release. Don't preemptively add a changelog entry
"to go with" the change.

### Retroactive tags

The pre-existing release commits were tagged retroactively: `v0.2.0`
on `8136f21` and `v0.2.1` on `2a55c7e`. The `0.1.0` changelog entry
represents an initial-packaging state without its own dedicated
release commit and is intentionally not tagged. When tagging a new
release, just follow the workflow above; no further retroactive work
is needed.

## Configuration

Env vars only. No CLI flags except `-v` / `--version` / `version`.

| Var | Default | Notes |
|---|---|---|
| `STORE` | `bolt` | `bolt` or `mem` (also accepts `memory`). |
| `DB_PATH` | `wclip.db` | bolt only. |
| `BIND` | unset | Listen address(es). **Unset = all available loopbacks** (`127.0.0.1` plus `[::1]` if available; best-effort — missing IPv6 loopback is skipped, not fatal). Explicit value is strict: comma-separated, every entry must bind. Token `*` is the wildcard alias for `""` (all interfaces, dual-stack `[::]`). IPv6 literals must be bracketed. |
| `PORT` | `8093` | |
| `DEBUG` | unset | Any value → gin debug mode. |
| `HTTP_USER` / `HTTP_PASSWORD` | unset | Must be set together; setting only one is a fatal startup error. |

When adding a new env var: document it in `README.md` **and**
`debian/wclip.default` (the conffile shipped to `/etc/default/wclip`).

## Architecture notes / invariants

- **`Store` interface.** Add a new backend by implementing
  `Get/Set/Close` and wiring a new `case` in `main`'s `switch backend`.
- **Bolt schema.** Bucket `wclip`, two keys per path:
  `content:<path>` and `content-type:<path>`. "Exists" is determined
  by `content != nil`, not a separate marker. Preserve this if you
  change the layout, or the `/robots.txt` fallback logic in the
  handler will misbehave.
- **`newRouter(store, user, pass)`** is the factory used by both
  `main` and tests. Keep it pure — don't read env vars or touch globals
  inside it; do that in `main` and pass values in.
- **Multi-listener startup.** `main` may open more than one
  `net.Listener` (when `BIND` is a comma-separated list, or when the
  default loopback set is used) and run a `srv.Serve(ln)` goroutine
  per listener, all sharing one `http.Server` and the gin router.
  All listeners are pre-opened before any goroutine starts, so an
  `EADDRINUSE` on any address fails the whole process cleanly. The
  first listener that errors terminates the process via `log.Fatal`.
- **Strict vs best-effort binding.** When `BIND` is set explicitly,
  binding is **strict** — any failed `net.Listen` aborts startup
  (security: don't silently bind fewer addresses than the operator
  asked for). When `BIND` is unset (default loopback mode), binding
  is **best-effort** — a per-address failure is logged and skipped,
  and startup proceeds as long as at least one listener came up.
  This is what lets the default mode work in containers without IPv6
  loopback. If you add a new "default-list" code path, preserve this
  asymmetry.
- **Routes.** Single wildcard `/*path` for `GET`, `POST`, `PUT`.
  - `GET /robots.txt` returns a disallow-all body **only** when nothing
    is stored at that path; a user-`PUT` value wins.
  - CORS headers (`Access-Control-Allow-Origin: *`,
    `Access-Control-Allow-Methods: GET`) are set globally via
    middleware on every response.
- **Basic auth, when configured, applies to all methods including
  `GET`.** This is asserted by `TestAuth_MissingCredentials_401` and
  friends — don't accidentally narrow it to writes.
- **bbolt tx lifetime.** Values from `bolt.Bucket.Get` are only valid
  during the transaction; `BoltStore.Get` copies `content` out. Keep
  this copy if you refactor.

## Testing patterns

- Standard fixture: `newRouter(NewMemStore(), "", "")`.
- HTTP helper: `do(t, r, method, path, body, ct, auth)` in
  `server_test.go`; `basic(user, pass)` builds an `Authorization`
  header.
- Bolt tests use `newTestBoltStore(t)` which allocates a `t.TempDir()`
  database; `TestHandler_WithBoltStore` cross-checks the handlers
  against the bolt backend.
- `gin.SetMode(gin.TestMode)` is set in a test `init()`.

## Debian packaging quick facts

- Native package, format `3.0 (native)`.
- Installs:
  - `/usr/bin/wclip`
  - `/lib/systemd/system/wclip.service` — hardened
    (`ProtectSystem=strict`, `NoNewPrivileges`, etc.), runs as the
    `wclip` system user, `ReadWritePaths=/var/lib/wclip`.
  - `/etc/default/wclip` — conffile, env vars for the unit.
  - `/var/lib/wclip/` — data dir, `wclip:wclip`, mode 0750
    (`debian/wclip.dirs` + `postinst`).
- `debian/rules` overrides `dh_auto_build/test/clean/install`, uses
  in-tree `.gocache` / `.gomodcache` (both gitignored).
- **Service is not enabled or started on install.**
  `debian/rules` overrides `dh_installsystemd` with
  `--no-enable --no-start`, so a fresh `dpkg -i` / `apt install`
  installs the unit dormant. The operator opts in with
  `systemctl enable --now wclip`. Upgrade behavior is unchanged — a
  service the user has already enabled+started is restarted normally
  on upgrade. Rationale: lets the operator edit
  `/etc/default/wclip` (BIND, HTTP_USER/HTTP_PASSWORD, etc.) before
  the service first runs, and avoids surprising the user with an
  open listener they didn't ask for.
- **If you add a runtime-writable path, also add it to
  `ReadWritePaths=` in `debian/wclip.service`** or the service will
  fail to write to it.
- The `wclip` user/group is created in `postinst` and removed on
  `purge` in `postrm`.

## Common gotchas

- Tabs in `src/*.go` — see formatting note above.
- Forgetting to update `debian/wclip.default` when adding env vars.
- Forgetting `ReadWritePaths=` when adding new on-disk state.
- `ioutil` is still used in `server.go` / `server_test.go`. If you
  modernize to `io` / `os`, do it in one pass across the package,
  don't leave it half-migrated.
- The Dockerfile sets `PORT=80` and `EXPOSE 80` — different from the
  `:8093` default used everywhere else. Keep that in mind if you touch
  default-port logic.
