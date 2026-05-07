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

// silence unused import warnings if we change tests later
var _ = ioutil.Discard
