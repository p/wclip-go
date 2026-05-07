package main

import (
  "sync"
  "testing"
)

func TestMemStore_GetMissing(t *testing.T) {
  s := NewMemStore()
  if _, _, ok := s.Get("/nope"); ok {
    t.Fatal("expected ok=false for missing key")
  }
}

func TestMemStore_SetGetRoundtrip(t *testing.T) {
  s := NewMemStore()
  if err := s.Set("/a", []byte("hello"), "text/plain"); err != nil {
    t.Fatal(err)
  }
  c, ct, ok := s.Get("/a")
  if !ok {
    t.Fatal("expected ok=true")
  }
  if string(c) != "hello" {
    t.Fatalf("content = %q", c)
  }
  if ct != "text/plain" {
    t.Fatalf("content-type = %q", ct)
  }
}

func TestMemStore_Overwrite(t *testing.T) {
  s := NewMemStore()
  _ = s.Set("/k", []byte("v1"), "text/plain")
  _ = s.Set("/k", []byte("v2"), "application/json")
  c, ct, _ := s.Get("/k")
  if string(c) != "v2" || ct != "application/json" {
    t.Fatalf("got %q / %q", c, ct)
  }
}

func TestMemStore_EmptyValue(t *testing.T) {
  s := NewMemStore()
  if err := s.Set("/k", []byte{}, ""); err != nil {
    t.Fatal(err)
  }
  c, ct, ok := s.Get("/k")
  if !ok {
    t.Fatal("expected ok=true for empty value")
  }
  if len(c) != 0 || ct != "" {
    t.Fatalf("got %q / %q", c, ct)
  }
}

func TestMemStore_Close(t *testing.T) {
  s := NewMemStore()
  if err := s.Close(); err != nil {
    t.Fatal(err)
  }
}

func TestMemStore_Concurrent(t *testing.T) {
  s := NewMemStore()
  var wg sync.WaitGroup
  for i := 0; i < 50; i++ {
    wg.Add(2)
    go func() {
      defer wg.Done()
      _ = s.Set("/k", []byte("x"), "text/plain")
    }()
    go func() {
      defer wg.Done()
      s.Get("/k")
    }()
  }
  wg.Wait()
}
