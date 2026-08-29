package gx

import "sync"

// Stack is a concurrency-safe, dynamically growing LIFO stack.
type Stack[T any] struct {
	closed bool
	cond   *sync.Cond
	items  []T
	len    int
	mu     sync.Mutex
}

// NewStack creates a new Stack with the specified initial capacity.
// If initialCapacity is zero or negative, it defaults to 1.
func NewStack[T any](initialCapacity int) *Stack[T] {
	if initialCapacity <= 0 {
		initialCapacity = 1
	}

	s := &Stack[T]{
		items: make([]T, initialCapacity),
	}
	s.cond = sync.NewCond(&s.mu)

	return s
}

// Close closes the stack and wakes any goroutines waiting in Pop.
// Values already in the stack remain available until drained.
// Close is safe to call multiple times.
func (s *Stack[T]) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	s.cond.Broadcast()
}

// Len returns the number of values currently in the stack.
func (s *Stack[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.len
}

// Pop blocks until a value is available or the stack is closed.
// It returns false when the stack is closed and empty.
func (s *Stack[T]) Pop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for s.len == 0 && !s.closed {
		s.cond.Wait()
	}

	if s.len == 0 {
		var zero T
		return zero, false
	}

	return s.popLocked(), true
}

// Push adds value to the top of the stack.
// It returns false if the stack is closed.
func (s *Stack[T]) Push(value T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}

	s.growLocked()

	s.items[s.len] = value
	s.len++

	s.cond.Signal()
	return true
}

// TryPop removes and returns the top value without blocking.
// It returns false if the stack is empty.
func (s *Stack[T]) TryPop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.len == 0 {
		var zero T
		return zero, false
	}

	return s.popLocked(), true
}

// growLocked doubles the backing storage when the stack is full.
// Caller must hold s.mu.
func (s *Stack[T]) growLocked() {
	if s.len < len(s.items) {
		return
	}

	items := make([]T, len(s.items)*2)
	copy(items, s.items)

	s.items = items
}

// popLocked removes and returns the top value in the stack.
// Caller must hold s.mu.
func (s *Stack[T]) popLocked() T {
	s.len--

	value := s.items[s.len]

	var zero T
	s.items[s.len] = zero

	return value
}
