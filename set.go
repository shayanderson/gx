package gx

import (
	"maps"
	"sync"
)

// Set is a concurrency-safe collection of unique values.
type Set[T comparable] struct {
	m  map[T]struct{}
	mu sync.RWMutex
}

// NewSet returns an empty set.
func NewSet[T comparable](values ...T) *Set[T] {
	s := &Set[T]{
		m: make(map[T]struct{}, len(values)),
	}

	for _, v := range values {
		s.m[v] = struct{}{}
	}

	return s
}

// Add inserts v into the set.
// It returns true if v was not already present.
func (s *Set[T]) Add(v T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.m[v]; ok {
		return false
	}

	s.m[v] = struct{}{}
	return true
}

// Clear removes all values from the set.
func (s *Set[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.m)
}

// Clone returns a shallow copy of the set.
func (s *Set[T]) Clone() *Set[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &Set[T]{
		m: maps.Clone(s.m),
	}
}

// Has returns true if v is in the set.
func (s *Set[T]) Has(v T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.m[v]
	return ok
}

// Len returns the number of values in the set.
func (s *Set[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.m)
}

// Remove removes v from the set.
// It returns true if v was present.
func (s *Set[T]) Remove(v T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.m[v]; !ok {
		return false
	}

	delete(s.m, v)
	return true
}

// Values returns the values in the set.
// The order is unspecified.
func (s *Set[T]) Values() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	values := make([]T, 0, len(s.m))
	for v := range s.m {
		values = append(values, v)
	}

	return values
}
