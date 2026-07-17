package store

import (
	"sync"
)

type Store struct {
	mu sync.RWMutex
	kv map[string]any
}

func New() *Store {
	return &Store{kv: make(map[string]any)}
}

func (s *Store) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[key] = value
}

func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if v, ok := s.kv[key]; ok {
		return v, ok
	}
	return nil, false
}
