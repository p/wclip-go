package main

import (
  "fmt"
  "io/ioutil"
  "log"
  "net"
  "net/http"
  "os"
  "strconv"
  "strings"

  "github.com/gin-gonic/gin"
)

// version, commit, and date live in version.go; they are set via
// -ldflags at build time and fall back to runtime/debug.ReadBuildInfo().

var http_user, http_password string

type handler struct {
  store Store
}

func (h *handler) get(c *gin.Context) {
  path := c.Param("path")
  content, ct, ok := h.store.Get(path)
  if !ok {
    if path == "/robots.txt" {
      c.Header("content-type", "text/plain")
      c.String(http.StatusOK, "User-agent: *\nDisallow: /\n")
      return
    }
    c.String(404, "Not found")
    return
  }
  c.Header("content-type", ct)
  c.String(200, string(content[:]))
}

func (h *handler) set(c *gin.Context) {
  path := c.Param("path")
  content, err := ioutil.ReadAll(c.Request.Body)
  if err != nil {
    c.String(500, "Error reading request: "+err.Error())
    return
  }
  ct := c.GetHeader("content-type")

  if err := h.store.Set(path, content, ct); err != nil {
    c.String(500, "Error saving: "+err.Error())
    return
  }
  c.String(http.StatusCreated, "Created")
}

func set_cors_headers(c *gin.Context) {
  c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
  c.Writer.Header().Set("Access-Control-Allow-Methods", "GET")
}

func main() {
  if len(os.Args) > 1 {
    switch os.Args[1] {
    case "-v", "--version", "version":
      fmt.Println(versionString())
      return
    }
  }
  log.Printf("wclip %s starting", shortVersion())

  http_user = os.Getenv("HTTP_USER")
  http_password = os.Getenv("HTTP_PASSWORD")
  if http_user == "" && http_password != "" {
    log.Fatal("HTTP_PASSWORD was specified but HTTP_USER was not, they need to be given together")
  }
  if http_user != "" && http_password == "" {
    log.Fatal("HTTP_USER was specified but HTTP_PASSWORD was not, they need to be given together")
  }

  var store Store
  backend := strings.ToLower(os.Getenv("STORE"))
  if backend == "" {
    backend = "bolt"
  }
  switch backend {
  case "mem", "memory":
    store = NewMemStore()
    log.Printf("using in-memory store")
  case "bolt":
    db_path := os.Getenv("DB_PATH")
    if db_path == "" {
      db_path = "wclip.db"
    }
    bs, err := NewBoltStore(db_path)
    if err != nil {
      log.Fatalf("Error opening database: %v", err)
    }
    store = bs
    log.Printf("using bolt store at %s", db_path)
  default:
    log.Fatalf("unknown STORE backend: %q (expected 'bolt' or 'mem')", backend)
  }
  defer store.Close()

  debug := os.Getenv("DEBUG")
  if debug == "" {
    gin.SetMode(gin.ReleaseMode)
  }

  router := newRouter(store, http_user, http_password)

  port := os.Getenv("PORT")
  var iport int
  var err error
  if port == "" {
    iport = 8093
  } else {
    iport, err = strconv.Atoi(port)
    if err != nil {
      log.Fatal(err)
    }
  }
  bindEnv := os.Getenv("BIND")
  var (
    binds      []string
    bestEffort bool
  )
  if strings.TrimSpace(bindEnv) == "" {
    // Default: listen on all available loopback addresses. IPv6
    // loopback may be absent (e.g. some containers) — skip it with a
    // log message rather than refusing to start.
    binds = defaultBinds()
    bestEffort = true
  } else {
    binds = parseBindList(bindEnv)
    if len(binds) == 0 {
      log.Fatalf("BIND=%q yielded no listen addresses", bindEnv)
    }
  }

  // Pre-create listeners up front. Explicit BIND is strict: any
  // failure aborts. The default (loopback) mode is best-effort: skip
  // an address that fails to bind as long as at least one succeeds.
  var lns []net.Listener
  for _, b := range binds {
    addr := listenAddr(b, iport)
    ln, err := net.Listen("tcp", addr)
    if err != nil {
      if bestEffort {
        log.Printf("skipping %s: %v", addr, err)
        continue
      }
      for _, l := range lns {
        l.Close()
      }
      log.Fatalf("listen on %s: %v", addr, err)
    }
    log.Printf("listening on %s", ln.Addr())
    lns = append(lns, ln)
  }
  if len(lns) == 0 {
    log.Fatalf("no listeners could be opened (tried %v)", binds)
  }

  // Share one http.Server across all listeners; first listener error
  // terminates the process.
  srv := &http.Server{Handler: router}
  errCh := make(chan error, len(lns))
  for _, ln := range lns {
    ln := ln
    go func() { errCh <- srv.Serve(ln) }()
  }
  log.Fatal(<-errCh)
}

// listenAddr builds a single address string passed to net.Listen.
// An empty bind preserves the historical behavior of listening on all
// interfaces (":<port>"). IPv6 literals must be bracketed by the caller,
// e.g. BIND="[::1]".
func listenAddr(bind string, port int) string {
  return fmt.Sprintf("%s:%d", bind, port)
}

// defaultBinds is the list of addresses tried when BIND is unset:
// IPv4 + IPv6 loopback. Binding is best-effort — an address that
// fails (e.g. [::1] in a container without IPv6) is skipped, and the
// server starts as long as at least one succeeds.
func defaultBinds() []string {
  return []string{"127.0.0.1", "[::1]"}
}

// parseBindList parses a comma-separated BIND value into a deduped
// list of listen addresses (each suitable as the bind argument to
// listenAddr). The token "*" is the wildcard alias for "all
// interfaces" and expands to "" (net.Listen on ":<port>", dual-stack
// on platforms that support it).
//
// Input is split on commas, each entry is trimmed, empty entries are
// dropped, and duplicates are removed (preserving order). An
// empty/whitespace-only input, or one yielding no usable entries
// (e.g. "," or ", ,"), returns nil; the caller decides whether that
// is the "no BIND set" default case or a configuration error.
func parseBindList(s string) []string {
  if strings.TrimSpace(s) == "" {
    return nil
  }
  seen := map[string]bool{}
  var out []string
  for _, p := range strings.Split(s, ",") {
    p = strings.TrimSpace(p)
    if p == "" {
      continue
    }
    if p == "*" {
      p = "" // wildcard alias → all interfaces
    }
    if seen[p] {
      continue
    }
    seen[p] = true
    out = append(out, p)
  }
  return out
}

func newRouter(store Store, user, pass string) *gin.Engine {
  h := &handler{store: store}
  router := gin.New()
  router.Use(gin.Recovery())
  router.Use(set_cors_headers)
  if user != "" {
    router.Use(gin.BasicAuth(gin.Accounts{user: pass}))
  }
  router.GET("/*path", h.get)
  router.POST("/*path", h.set)
  router.PUT("/*path", h.set)
  return router
}
