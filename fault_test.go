package gx

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/shayanderson/gx/test"
)

func TestFault_ZeroValue(t *testing.T) {
	t.Parallel()

	var fault Fault

	select {
	case <-fault.Done():
		t.Fatal("Done closed before Set")
	default:
	}

	test.Nil(t, fault.Err())

	err := errors.New("fault")
	test.True(t, fault.Set(err))

	select {
	case <-fault.Done():
	case <-t.Context().Done():
		t.Fatal("Done did not close after Set")
	}

	test.ErrorIs(t, fault.Err(), err)
}

func TestFault_SetNil(t *testing.T) {
	t.Parallel()

	fault := NewFault()
	test.False(t, fault.Set(nil))

	select {
	case <-fault.Done():
		t.Fatal("Done closed after Set(nil)")
	default:
	}

	err := errors.New("fault")
	test.True(t, fault.Set(err))
	test.ErrorIs(t, fault.Err(), err)
}

func TestFault_SetFirstError(t *testing.T) {
	t.Parallel()

	fault := NewFault()
	first := errors.New("first")
	second := errors.New("second")

	test.True(t, fault.Set(first))
	test.False(t, fault.Set(second))
	test.ErrorIs(t, fault.Err(), first)
}

func TestFault_ConcurrentSet(t *testing.T) {
	t.Parallel()

	const goroutines = 50

	fault := NewFault()
	errs := make([]error, goroutines)
	for i := range errs {
		errs[i] = errors.New("fault")
	}

	var set atomic.Int32
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			if fault.Set(errs[i]) {
				set.Add(1)
			}
		})
	}
	wg.Wait()

	test.Equal(t, int32(1), set.Load())
	test.True(t, slices.Contains(errs, fault.Err()))
}
