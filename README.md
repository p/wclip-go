# wclip-go

A tiny HTTP "web clipboard": `PUT`/`POST` a body to any path, `GET` it
back later with the same content-type.

## Build

    make

Produces `build/wclip`. Tests:

    make test

## Run

    ./build/wclip

Listens on `:8093` by default. Print the version and exit:

    ./build/wclip --version

## Usage

    curl -X PUT -d 'hello' http://localhost:8093/foo
    curl http://localhost:8093/foo
    # -> hello

`POST` works the same as `PUT`. Nested paths are fine
(`/a/b/c`). The server stores the request body and its `Content-Type`
header, and returns both verbatim on `GET`.

A `GET /robots.txt` with nothing stored at that path returns a
disallow-all body, so crawlers don't index your clips by default. If
you `PUT` your own content to `/robots.txt`, your value wins.

## Configuration

All configuration is via environment variables.

| Var             | Default    | Description                                                                     |
|-----------------|------------|---------------------------------------------------------------------------------|
| `STORE`         | `bolt`     | Storage backend: `bolt` (persistent, on-disk) or `mem` (in-memory, volatile).   |
| `DB_PATH`       | `wclip.db` | Path to the bbolt database file. Only used when `STORE=bolt`.                   |
| `BIND`          | unset      | Address(es) to bind to / listen on. The default (unset) listens on **all available loopback addresses** — `127.0.0.1` and, if available, `[::1]`. To expose the server beyond loopback, set `BIND` explicitly: `BIND=*` (or `BIND=0.0.0.0,[::]`) listens on all interfaces; a comma-separated list like `BIND=127.0.0.1,10.0.0.5` binds those specific addresses. IPv6 literals must be bracketed. All listeners share the same `PORT`. When using `*` from a shell, quote it (`BIND='*'`) to suppress glob expansion. |
| `PORT`          | `8093`     | TCP port to listen on. The server listens on `BIND:PORT`.                       |
| `DEBUG`         | unset      | If set to any value, gin runs in debug mode (verbose logs).                     |
| `HTTP_USER`     | unset      | If set (together with `HTTP_PASSWORD`), enables HTTP Basic Auth on all routes.  |
| `HTTP_PASSWORD` | unset      | Password for Basic Auth. Must be set together with `HTTP_USER`.                 |

Setting only one of `HTTP_USER` / `HTTP_PASSWORD` is a startup error.
With neither set, the server is open (no auth).

**Default bind changed:** previous versions listened on all interfaces
by default. The default is now loopback-only. If you reach wclip from
another host (LAN, separate-host reverse proxy, etc.), set `BIND=*`
(or an explicit list of addresses) to restore the previous behavior.
The shipped Docker image sets `BIND=*` so `docker run -p ...` keeps
working; the Debian package leaves it unset so a default install is
loopback-only until you edit `/etc/default/wclip`.

### Bind failure semantics

- **Default (`BIND` unset).** Best-effort: tries `127.0.0.1` and
  `[::1]`, skips any that fail to bind (e.g. `[::1]` in containers
  without IPv6) with a log message, fails only if **all** loopback
  binds fail.
- **Explicit `BIND`.** Strict: every listed address must bind
  successfully; the first failure aborts startup.
- An explicit but empty value such as `BIND=,` is a configuration
  error.


Basic Auth is sent in cleartext — put the server behind a
TLS-terminating reverse proxy if you care.

### In-memory store

    STORE=mem ./build/wclip

All data is lost on restart. No file is created.

### CORS

Responses include `Access-Control-Allow-Origin: *` and
`Access-Control-Allow-Methods: GET`, so browser-based clients on other
origins can read clips but cannot write them via cross-origin requests.

## Docker

    make docker

Builds a `scratch`-based image tagged `wclip-go`.

## Debian package

A native Debian package is built from the `debian/` directory:

    dpkg-buildpackage -us -uc -b

Produces `../wclip_<version>_<arch>.deb`. The package installs:

- `/usr/bin/wclip`
- `/usr/lib/systemd/system/wclip.service` (hardened, runs as the
  `wclip` system user, sources `/etc/default/wclip` for env vars)
- `/etc/default/wclip` (conffile — your edits survive upgrades)
- `/var/lib/wclip/` (data dir, owned by `wclip:wclip`)

The service is **not** enabled or started on install. After install,
opt in explicitly:

    sudo systemctl enable --now wclip

This lets you edit `/etc/default/wclip` (e.g. to set `BIND`,
`HTTP_USER` / `HTTP_PASSWORD`) before the service first runs. Upgrades
preserve whatever state you've set: an already-enabled-and-running
service is restarted normally on upgrade.

The package version is taken from the top entry of `debian/changelog`,
which is also the single source of truth for the version baked into
the binary (via `-ldflags -X main.version=...`).

## License

BSD 2-Clause. Copyright (c) 2020, 2026 Oleg Pudeyev. See [LICENSE](LICENSE)
for the full text.
