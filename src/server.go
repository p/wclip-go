package main

import (
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "strconv"
  "strings"

  "github.com/gin-gonic/gin"
)

// version is set via -ldflags "-X main.version=..." at build time.
var version = "dev"

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
      fmt.Println(version)
      return
    }
  }
  log.Printf("wclip %s starting", version)

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
  router.Run(fmt.Sprintf(":%d", iport))
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
