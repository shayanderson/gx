package gx

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestNewDebouncer(t *testing.T) {
	d := NewDebouncer(25 * time.Millisecond)

	test.Equal(t, 25*time.Millisecond, d.delay)
}

func TestNewDebouncerDefaultDelay(t *testing.T) {
	d := NewDebouncer(0)

	test.Equal(t, 10*time.Millisecond, d.delay)
}

func TestDebouncerDo(t *testing.T) {
	d := NewDebouncer(10 * time.Millisecond)
	done := make(chan struct{}, 1)

	d.Do(func() {
		done <- struct{}{}
	})

	select {
	case <-done:
		t.Fatal("expected function to be delayed")
	case <-time.After(5 * time.Millisecond):
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for function")
	}
}

func TestDebouncerDoResetsTimer(t *testing.T) {
	d := NewDebouncer(20 * time.Millisecond)
	var calls atomic.Int32
	done := make(chan struct{}, 1)

	d.Do(func() {
		calls.Add(1)
		done <- struct{}{}
	})
	time.Sleep(10 * time.Millisecond)
	d.Do(func() {
		calls.Add(1)
		done <- struct{}{}
	})

	select {
	case <-done:
		t.Fatal("expected first function to be canceled")
	case <-time.After(15 * time.Millisecond):
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for function")
	}

	test.Equal(t, int32(1), calls.Load())
}

func TestDebouncerCancel(t *testing.T) {
	d := NewDebouncer(10 * time.Millisecond)
	var calls atomic.Int32

	d.Do(func() {
		calls.Add(1)
	})
	d.Cancel()
	time.Sleep(20 * time.Millisecond)

	test.Equal(t, int32(0), calls.Load())
}

func TestDebouncerCancelWithoutTimer(t *testing.T) {
	d := NewDebouncer(10 * time.Millisecond)

	d.Cancel()

	test.Nil(t, d.timer)
}

func TestDebouncerCancelAfterRun(t *testing.T) {
	d := NewDebouncer(10 * time.Millisecond)
	done := make(chan struct{}, 1)

	d.Do(func() {
		done <- struct{}{}
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for function")
	}

	d.Cancel()

	test.Nil(t, d.timer)
}
