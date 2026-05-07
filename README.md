# wclip-go

A tiny HTTP "web clipboard": `PUT`/`POST` a body to any path, `GET` it back with the same content-type.

## Build

    make

Produces `tmp/wclip`.

## Run

    ./tmp/wclip

Listens on `:8093` by default.

## Usage

    curl -X PUT -d 'hello' http://localhost:8093/foo
    curl http://localhost:8093/foo
    # -> hello

## Configuration

All configuration is via environment variables.

| Var             | Default    | Description                                                                 |
|-----------------|------------|-----------------------------------------------------------------------------|
| `STORE`         | `bolt`     | Storage backend: `bolt` (persistent, on-disk) or `mem` (in-memory, volatile). |
| `DB_PATH`       | `wclip.db` | Path to the bbolt database file. Only used when `STORE=bolt`.               |
| `PORT`          | `8093`     | TCP port to listen on.                                                      |
| `DEBUG`         | unset      | If set to any value, gin runs in debug mode (verbose logs).                 |
| `HTTP_USER`     | unset      | If set (together with `HTTP_PASSWORD`), enables HTTP Basic Auth on all routes. |
| `HTTP_PASSWORD` | unset      | Password for Basic Auth. Must be set together with `HTTP_USER`.             |

Setting only one of `HTTP_USER` / `HTTP_PASSWORD` is a startup error.
With neither set, the server is open (no auth).

Responses include `Access-Control-Allow-Origin: *` and
`Access-Control-Allow-Methods: GET`, so browser-based clients on other
origins can read clips but cannot write them via cross-origin requests.

Basic Auth is sent in cleartext — put the server behind a TLS-terminating
reverse proxy if you care.

### In-memory store

    STORE=mem ./tmp/wclip

All data is lost on restart. No file is created.

## Docker

    make docker
