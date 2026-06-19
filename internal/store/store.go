package store

import (
	"sync"
)

type Store struct {
	mu   sync.RWMutex
	kv   map[string]any // to read/write settings by their names.
	tree *node          // to facilitate marshaling.
}

func New() *Store {
	return &Store{kv: make(map[string]any), tree: &node{}}
}

func (s *Store) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kv[key] = value
	s.tree.add(key)
}

func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if v, ok := s.kv[key]; ok {
		return v, ok
	}
	return nil, false
}
