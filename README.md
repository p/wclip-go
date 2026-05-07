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
| `PORT`          | `8093`     | TCP port to listen on.                                                          |
| `DEBUG`         | unset      | If set to any value, gin runs in debug mode (verbose logs).                     |
| `HTTP_USER`     | unset      | If set (together with `HTTP_PASSWORD`), enables HTTP Basic Auth on all routes.  |
| `HTTP_PASSWORD` | unset      | Password for Basic Auth. Must be set together with `HTTP_USER`.                 |

Setting only one of `HTTP_USER` / `HTTP_PASSWORD` is a startup error.
With neither set, the server is open (no auth).

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

The service is enabled and started automatically on install.

The package version is taken from the top entry of `debian/changelog`,
which is also the single source of truth for the version baked into
the binary (via `-ldflags -X main.version=...`).

## License

BSD 2-Clause. Copyright (c) 2020, 2026 Oleg Pudeyev. See [LICENSE](LICENSE)
for the full text.
