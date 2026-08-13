package gx

import "sync"

// Map is a concurrency-safe map of key-value pairs.
type Map[K comparable, V any] struct {
	m  map[K]V
	mu sync.RWMutex
}

// NewMap returns an empty map.
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		m: make(map[K]V),
	}
}

// Clear removes all key-value pairs from the map.
func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	clear(m.m)
}

// Get returns the value associated with key.
// The returned bool is true if the key was present.
func (m *Map[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.m[key]
	return v, ok
}

// GetAndRemove removes key from the map and returns its value.
// The returned bool is true if the key was present.
func (m *Map[K, V]) GetAndRemove(key K) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.m[key]
	if !ok {
		var zero V
		return zero, false
	}

	delete(m.m, key)
	return v, true
}

// GetOrSet returns the value associated with key if present.
// Otherwise, it associates value with key and returns it.
// The returned bool is true if the key was present.
func (m *Map[K, V]) GetOrSet(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := m.m[key]; ok {
		return v, true
	}

	m.m[key] = value
	return value, false
}

// Has returns true if the key is present in the map.
func (m *Map[K, V]) Has(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.m[key]
	return ok
}

// Keys returns the keys in the map.
// The order is unspecified.
func (m *Map[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]K, 0, len(m.m))
	for k := range m.m {
		keys = append(keys, k)
	}

	return keys
}

// Len returns the number of key-value pairs in the map.
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.m)
}

// Remove removes key from the map.
// Returns true if the key was present.
func (m *Map[K, V]) Remove(key K) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.m[key]; !ok {
		return false
	}

	delete(m.m, key)
	return true
}

// Set associates value with key.
func (m *Map[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.m[key] = value
}

// Values returns the values in the map.
// The order is unspecified.
func (m *Map[K, V]) Values() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()

	values := make([]V, 0, len(m.m))
	for _, v := range m.m {
		values = append(values, v)
	}

	return values
}
