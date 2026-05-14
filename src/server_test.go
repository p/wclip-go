package main

import (
  "encoding/base64"
  "io/ioutil"
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"

  "github.com/gin-gonic/gin"
)

func init() {
  gin.SetMode(gin.TestMode)
}

func do(t *testing.T, r http.Handler, method, path, body, ct, auth string) *httptest.ResponseRecorder {
  t.Helper()
  var rdr *strings.Reader
  if body != "" {
    rdr = strings.NewReader(body)
  } else {
    rdr = strings.NewReader("")
  }
  req := httptest.NewRequest(method, path, rdr)
  if ct != "" {
    req.Header.Set("Content-Type", ct)
  }
  if auth != "" {
    req.Header.Set("Authorization", auth)
  }
  w := httptest.NewRecorder()
  r.ServeHTTP(w, req)
  return w
}

func basic(user, pass string) string {
  return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestGet_Missing_Returns404(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  w := do(t, r, "GET", "/nope", "", "", "")
  if w.Code != 404 {
    t.Fatalf("code = %d", w.Code)
  }
}

func TestPutThenGet_Roundtrip(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")

  w := do(t, r, "PUT", "/foo", "hello", "text/plain", "")
  if w.Code != http.StatusCreated {
    t.Fatalf("PUT code = %d, body = %s", w.Code, w.Body.String())
  }

  w = do(t, r, "GET", "/foo", "", "", "")
  if w.Code != 200 {
    t.Fatalf("GET code = %d", w.Code)
  }
  if w.Body.String() != "hello" {
    t.Fatalf("body = %q", w.Body.String())
  }
  if got := w.Header().Get("Content-Type"); got != "text/plain" {
    t.Fatalf("content-type = %q", got)
  }
}

func TestPost_AlsoStores(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  w := do(t, r, "POST", "/foo", "via post", "application/json", "")
  if w.Code != http.StatusCreated {
    t.Fatalf("POST code = %d", w.Code)
  }
  w = do(t, r, "GET", "/foo", "", "", "")
  if w.Body.String() != "via post" {
    t.Fatalf("body = %q", w.Body.String())
  }
  if got := w.Header().Get("Content-Type"); got != "application/json" {
    t.Fatalf("content-type = %q", got)
  }
}

func TestPut_NestedPath(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  do(t, r, "PUT", "/a/b/c", "deep", "text/plain", "")
  w := do(t, r, "GET", "/a/b/c", "", "", "")
  if w.Body.String() != "deep" {
    t.Fatalf("body = %q", w.Body.String())
  }
}

func TestPut_EmptyContentType(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  // gin/net-http may auto-set a content-type if we don't; explicitly clear
  req := httptest.NewRequest("PUT", "/x", strings.NewReader("data"))
  req.Header.Del("Content-Type")
  w := httptest.NewRecorder()
  r.ServeHTTP(w, req)
  if w.Code != http.StatusCreated {
    t.Fatalf("code = %d", w.Code)
  }

  w = do(t, r, "GET", "/x", "", "", "")
  if w.Body.String() != "data" {
    t.Fatalf("body = %q", w.Body.String())
  }
}

func TestRobotsTxt_Fallback(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  w := do(t, r, "GET", "/robots.txt", "", "", "")
  if w.Code != 200 {
    t.Fatalf("code = %d", w.Code)
  }
  body := w.Body.String()
  if !strings.Contains(body, "User-agent: *") || !strings.Contains(body, "Disallow: /") {
    t.Fatalf("unexpected body: %q", body)
  }
  if got := w.Header().Get("Content-Type"); got != "text/plain" {
    t.Fatalf("content-type = %q", got)
  }
}

func TestRobotsTxt_UserOverride(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  do(t, r, "PUT", "/robots.txt", "custom", "text/plain", "")
  w := do(t, r, "GET", "/robots.txt", "", "", "")
  if w.Body.String() != "custom" {
    t.Fatalf("expected user override, got %q", w.Body.String())
  }
}

func TestAuth_NotConfigured_Open(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  w := do(t, r, "GET", "/anything", "", "", "")
  if w.Code != 404 {
    t.Fatalf("expected open access (404 from store), got %d", w.Code)
  }
}

func TestAuth_MissingCredentials_401(t *testing.T) {
  r := newRouter(NewMemStore(), "user", "pw")
  w := do(t, r, "GET", "/x", "", "", "")
  if w.Code != http.StatusUnauthorized {
    t.Fatalf("code = %d", w.Code)
  }
}

func TestAuth_WrongCredentials_401(t *testing.T) {
  r := newRouter(NewMemStore(), "user", "pw")
  w := do(t, r, "GET", "/x", "", "", basic("user", "WRONG"))
  if w.Code != http.StatusUnauthorized {
    t.Fatalf("code = %d", w.Code)
  }
}

func TestAuth_CorrectCredentials_OK(t *testing.T) {
  r := newRouter(NewMemStore(), "user", "pw")

  w := do(t, r, "PUT", "/x", "ok", "text/plain", basic("user", "pw"))
  if w.Code != http.StatusCreated {
    t.Fatalf("PUT code = %d", w.Code)
  }
  w = do(t, r, "GET", "/x", "", "", basic("user", "pw"))
  if w.Code != 200 || w.Body.String() != "ok" {
    t.Fatalf("GET code=%d body=%q", w.Code, w.Body.String())
  }
}

func TestAuth_AppliesToWrites(t *testing.T) {
  r := newRouter(NewMemStore(), "user", "pw")
  w := do(t, r, "PUT", "/x", "nope", "text/plain", "")
  if w.Code != http.StatusUnauthorized {
    t.Fatalf("PUT without auth code = %d", w.Code)
  }
  w = do(t, r, "POST", "/x", "nope", "text/plain", "")
  if w.Code != http.StatusUnauthorized {
    t.Fatalf("POST without auth code = %d", w.Code)
  }
}

func TestCORS_HeadersPresent(t *testing.T) {
  r := newRouter(NewMemStore(), "", "")
  w := do(t, r, "GET", "/anything", "", "", "")
  if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
    t.Fatalf("Allow-Origin = %q", got)
  }
  if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET" {
    t.Fatalf("Allow-Methods = %q", got)
  }
}

func TestHandler_WithBoltStore(t *testing.T) {
  // Sanity: handlers work with bolt backend too, not just mem.
  s, _ := newTestBoltStore(t)
  defer s.Close()
  r := newRouter(s, "", "")

  w := do(t, r, "PUT", "/foo", "bolt-body", "text/plain", "")
  if w.Code != http.StatusCreated {
    t.Fatalf("PUT code = %d", w.Code)
  }
  w = do(t, r, "GET", "/foo", "", "", "")
  if w.Body.String() != "bolt-body" {
    t.Fatalf("body = %q", w.Body.String())
  }
}

func TestParseBindList(t *testing.T) {
  eq := func(a, b []string) bool {
    if len(a) != len(b) {
      return false
    }
    for i := range a {
      if a[i] != b[i] {
        return false
      }
    }
    return true
  }
  cases := []struct {
    in   string
    want []string
  }{
    {"", nil},                                             // unset → caller uses defaults
    {" ", nil},                                            // whitespace-only → same
    {"127.0.0.1", []string{"127.0.0.1"}},                  // single
    {"[::1]", []string{"[::1]"}},                          // IPv6 literal
    {"127.0.0.1,[::1]", []string{"127.0.0.1", "[::1]"}},   // both loopbacks
    {"127.0.0.1, [::1] ", []string{"127.0.0.1", "[::1]"}}, // whitespace tolerant
    {"127.0.0.1,127.0.0.1", []string{"127.0.0.1"}},        // dedupe
    {"127.0.0.1,,[::1]", []string{"127.0.0.1", "[::1]"}},  // drop empties
    {",", nil},                                            // explicit-but-empty (caller treats as error)
    {", ,", nil},                                          // same
    {"*", []string{""}},                                   // wildcard alias → all interfaces
    {"*,127.0.0.1", []string{"", "127.0.0.1"}},            // wildcard + extra
    {"*,*", []string{""}},                                 // dedupe wildcard
  }
  for _, c := range cases {
    got := parseBindList(c.in)
    if !eq(got, c.want) {
      t.Errorf("parseBindList(%q) = %v (len %d), want %v (len %d)",
        c.in, got, len(got), c.want, len(c.want))
    }
  }
}

func TestListenAddr(t *testing.T) {
  cases := []struct {
    bind string
    port int
    want string
  }{
    {"", 8093, ":8093"},          // backward compat: all interfaces
    {"127.0.0.1", 8093, "127.0.0.1:8093"},
    {"0.0.0.0", 80, "0.0.0.0:80"},
    {"[::1]", 8093, "[::1]:8093"}, // IPv6 literal, brackets supplied by user
    {"localhost", 8093, "localhost:8093"},
  }
  for _, c := range cases {
    got := listenAddr(c.bind, c.port)
    if got != c.want {
      t.Errorf("listenAddr(%q, %d) = %q, want %q", c.bind, c.port, got, c.want)
    }
  }
}

func TestDefaultBinds(t *testing.T) {
  got := defaultBinds()
  want := []string{"127.0.0.1", "[::1]"}
  if len(got) != len(want) {
    t.Fatalf("defaultBinds() = %v, want %v", got, want)
  }
  for i := range got {
    if got[i] != want[i] {
      t.Errorf("defaultBinds()[%d] = %q, want %q", i, got[i], want[i])
    }
  }
}

// silence unused import warnings if we change tests later
var _ = ioutil.Discard
