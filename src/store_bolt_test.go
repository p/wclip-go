package main

import (
  "os"
  "path/filepath"
  "testing"
)

func newTestBoltStore(t *testing.T) (*BoltStore, string) {
  t.Helper()
  dir := t.TempDir()
  path := filepath.Join(dir, "test.db")
  s, err := NewBoltStore(path)
  if err != nil {
    t.Fatalf("NewBoltStore: %v", err)
  }
  return s, path
}

func TestBoltStore_GetMissing(t *testing.T) {
  s, _ := newTestBoltStore(t)
  defer s.Close()
  if _, _, ok := s.Get("/nope"); ok {
    t.Fatal("expected ok=false for missing key")
  }
}

func TestBoltStore_SetGetRoundtrip(t *testing.T) {
  s, _ := newTestBoltStore(t)
  defer s.Close()
  if err := s.Set("/a", []byte("hello"), "text/plain"); err != nil {
    t.Fatal(err)
  }
  c, ct, ok := s.Get("/a")
  if !ok || string(c) != "hello" || ct != "text/plain" {
    t.Fatalf("got ok=%v c=%q ct=%q", ok, c, ct)
  }
}

func TestBoltStore_Overwrite(t *testing.T) {
  s, _ := newTestBoltStore(t)
  defer s.Close()
  _ = s.Set("/k", []byte("v1"), "text/plain")
  _ = s.Set("/k", []byte("v2"), "application/json")
  c, ct, _ := s.Get("/k")
  if string(c) != "v2" || ct != "application/json" {
    t.Fatalf("got %q / %q", c, ct)
  }
}

func TestBoltStore_Persistence(t *testing.T) {
  dir := t.TempDir()
  path := filepath.Join(dir, "persist.db")

  s1, err := NewBoltStore(path)
  if err != nil {
    t.Fatal(err)
  }
  if err := s1.Set("/k", []byte("durable"), "text/plain"); err != nil {
    t.Fatal(err)
  }
  if err := s1.Close(); err != nil {
    t.Fatal(err)
  }

  s2, err := NewBoltStore(path)
  if err != nil {
    t.Fatalf("reopen: %v", err)
  }
  defer s2.Close()
  c, ct, ok := s2.Get("/k")
  if !ok || string(c) != "durable" || ct != "text/plain" {
    t.Fatalf("got ok=%v c=%q ct=%q", ok, c, ct)
  }
}

func TestBoltStore_OpenError(t *testing.T) {
  // Opening a directory as a bbolt file should fail.
  dir := t.TempDir()
  if _, err := NewBoltStore(dir); err == nil {
    t.Fatal("expected error opening directory as bolt db")
  }
}

func TestBoltStore_FileCreated(t *testing.T) {
  s, path := newTestBoltStore(t)
  defer s.Close()
  if _, err := os.Stat(path); err != nil {
    t.Fatalf("expected db file at %s: %v", path, err)
  }
}

