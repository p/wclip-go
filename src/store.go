package main

type Store interface {
  Get(path string) (content []byte, contentType string, ok bool)
  Set(path string, content []byte, contentType string) error
  Close() error
}
