package main

import (
  bolt "go.etcd.io/bbolt"
)

type BoltStore struct {
  db *bolt.DB
}

func NewBoltStore(path string) (*BoltStore, error) {
  db, err := bolt.Open(path, 0600, nil)
  if err != nil {
    return nil, err
  }
  err = db.Update(func(tx *bolt.Tx) error {
    _, err := tx.CreateBucketIfNotExists([]byte("wclip"))
    return err
  })
  if err != nil {
    db.Close()
    return nil, err
  }
  return &BoltStore{db: db}, nil
}

func (s *BoltStore) Get(path string) ([]byte, string, bool) {
  var content []byte
  var ct string
  s.db.View(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("wclip"))
    v := b.Get([]byte("content:" + path))
    if v != nil {
      // bbolt values are only valid during the tx; copy out
      content = make([]byte, len(v))
      copy(content, v)
    }
    ct = string(b.Get([]byte("content-type:" + path)))
    return nil
  })
  if content == nil {
    return nil, "", false
  }
  return content, ct, true
}

func (s *BoltStore) Set(path string, content []byte, contentType string) error {
  return s.db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("wclip"))
    if err := b.Put([]byte("content:"+path), content); err != nil {
      return err
    }
    return b.Put([]byte("content-type:"+path), []byte(contentType))
  })
}

func (s *BoltStore) Close() error {
  return s.db.Close()
}
