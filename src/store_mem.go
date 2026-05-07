package main

import "sync"

type memEntry struct {
  content     []byte
  contentType string
}

type MemStore struct {
  mu   sync.RWMutex
  data map[string]memEntry
}

func NewMemStore() *MemStore {
  return &MemStore{data: make(map[string]memEntry)}
}

func (s *MemStore) Get(path string) ([]byte, string, bool) {
  s.mu.RLock()
  defer s.mu.RUnlock()
  e, ok := s.data[path]
  if !ok {
    return nil, "", false
  }
  return e.content, e.contentType, true
}

func (s *MemStore) Set(path string, content []byte, contentType string) error {
  s.mu.Lock()
  defer s.mu.Unlock()
  s.data[path] = memEntry{content: content, contentType: contentType}
  return nil
}

func (s *MemStore) Close() error { return nil }
