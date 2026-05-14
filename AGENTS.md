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

## Versioning (single source of truth)

The top entry of `debian/changelog` is the only place the version
lives. It is consumed by:

- `Makefile` — `awk` extracts it, passed as `-ldflags '-X main.version=$(VERSION)'`.
- `debian/rules` — `dpkg-parsechangelog -SVersion` does the same for the
  packaged binary, and also sets the `.deb` version.

`var version = "dev"` in `server.go` is the fallback when built without
ldflags. **To bump the version, edit `debian/changelog` only.**

**Release commits stand alone.** Never bundle a `debian/changelog`
version bump with any other change. A release commit edits *only*
`debian/changelog` (a new top entry summarizing what shipped since
the last release) and touches nothing else. Equally: **never cut a
release as part of doing other work.** Releases are an explicit,
separate decision made by a human — if you've just finished a feature
or fix, stop at the feature/fix commit and let the user decide when
(and whether) to release. Don't preemptively add a changelog entry
"to go with" the change.

## Configuration

Env vars only. No CLI flags except `-v` / `--version` / `version`.

| Var | Default | Notes |
|---|---|---|
| `STORE` | `bolt` | `bolt` or `mem` (also accepts `memory`). |
| `DB_PATH` | `wclip.db` | bolt only. |
| `BIND` | unset | Listen address(es). Empty → all interfaces (`:PORT`). Comma-separated for multiple listeners sharing one `PORT` (e.g. `127.0.0.1,[::1]`). IPv6 literals must be bracketed. Combined with `PORT` as `BIND:PORT` per entry. |
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
  `net.Listener` (when `BIND` is a comma-separated list) and run a
  `srv.Serve(ln)` goroutine per listener, all sharing one
  `http.Server` and the gin router. All listeners are pre-opened
  before any goroutine starts, so an `EADDRINUSE` on any address
  fails the whole process cleanly. The first listener that errors
  terminates the process via `log.Fatal`.
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
