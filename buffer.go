package gx

import "sync"

// Buffer is a concurrency-safe, dynamically growing FIFO buffer.
type Buffer[T any] struct {
	closed bool
	cond   *sync.Cond
	head   int
	items  []T
	len    int
	mu     sync.Mutex
}

// NewBuffer creates a new Buffer with the specified initial capacity.
// If initialCapacity is zero or negative, it defaults to 1.
func NewBuffer[T any](initialCapacity int) *Buffer[T] {
	if initialCapacity <= 0 {
		initialCapacity = 1
	}

	b := &Buffer[T]{
		items: make([]T, initialCapacity),
	}
	b.cond = sync.NewCond(&b.mu)

	return b
}

// Close closes the buffer and wakes any goroutines waiting in Next.
// Buffered values remain available until drained.
// Close is safe to call multiple times.
func (b *Buffer[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true
	b.cond.Broadcast()
}

// Len returns the number of values currently in the buffer.
func (b *Buffer[T]) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.len
}

// Next blocks until a value is available or the buffer is closed.
// It returns false when the buffer is closed and empty.
func (b *Buffer[T]) Next() (T, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for b.len == 0 && !b.closed {
		b.cond.Wait()
	}

	if b.len == 0 {
		var zero T
		return zero, false
	}

	return b.nextLocked(), true
}

// Push adds value to the end of the buffer.
// It returns false if the buffer is closed.
func (b *Buffer[T]) Push(value T) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return false
	}

	b.growLocked()

	i := (b.head + b.len) % len(b.items)
	b.items[i] = value
	b.len++

	b.cond.Signal()
	return true
}

// TryNext returns the next value without blocking.
// It returns false if the buffer is empty.
func (b *Buffer[T]) TryNext() (T, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.len == 0 {
		var zero T
		return zero, false
	}

	return b.nextLocked(), true
}

// growLocked doubles the backing storage when the buffer is full.
// Caller must hold b.mu.
func (b *Buffer[T]) growLocked() {
	if b.len < len(b.items) {
		return
	}

	items := make([]T, len(b.items)*2)

	if b.head+b.len <= len(b.items) {
		copy(items, b.items[b.head:b.head+b.len])
	} else {
		n := copy(items, b.items[b.head:])
		copy(items[n:], b.items[:b.len-n])
	}

	b.items = items
	b.head = 0
}

// nextLocked removes and returns the first value in the buffer.
// Caller must hold b.mu.
func (b *Buffer[T]) nextLocked() T {
	value := b.items[b.head]

	var zero T
	b.items[b.head] = zero

	b.head = (b.head + 1) % len(b.items)
	b.len--

	if b.len == 0 {
		b.head = 0
	}

	return value
}
