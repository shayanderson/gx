package gx_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/shayanderson/gx"
	"github.com/shayanderson/gx/test"
)

func TestDispatcherDispatch(t *testing.T) {
	d := gx.NewDispatcher()
	var values []string

	d.Register(func(ctx context.Context, value string) error {
		test.Equal(t, t.Context(), ctx)
		values = append(values, "first:"+value)
		return nil
	})
	d.Register(func(ctx context.Context, value string) error {
		test.Equal(t, t.Context(), ctx)
		values = append(values, "second:"+value)
		return nil
	})
	d.Register(func(ctx context.Context, value int) error {
		values = append(values, "int")
		return nil
	})

	test.NoError(t, d.Dispatch(t.Context(), "value"))
	test.Equal(t, []string{"first:value", "second:value"}, values)
}

func TestDispatcherDispatchError(t *testing.T) {
	d := gx.NewDispatcher()
	errStop := errors.New("stop")
	var values []int

	d.Register(func(context.Context, int) error {
		values = append(values, 1)
		return nil
	})
	d.Register(func(context.Context, int) error {
		values = append(values, 2)
		return errStop
	})
	d.Register(func(context.Context, int) error {
		values = append(values, 3)
		return nil
	})

	test.Error(t, d.Dispatch(t.Context(), 123), errStop)
	test.Equal(t, []int{1, 2}, values)
}

func TestDispatcherDispatchNoHandler(t *testing.T) {
	d := gx.NewDispatcher()

	err := d.Dispatch(t.Context(), "missing")

	test.NotNil(t, err)
	test.Equal(t, "dispatcher: no handler registered for type string", err.Error())
}

func TestDispatcherUnregister(t *testing.T) {
	d := gx.NewDispatcher()
	var values []string

	d.Register(func(context.Context, string) error {
		values = append(values, "string")
		return nil
	})
	d.Register(func(context.Context, int) error {
		values = append(values, "int")
		return nil
	})

	d.Unregister[string]()

	test.NoError(t, d.Dispatch(t.Context(), 1))
	test.Equal(t, []string{"int"}, values)

	err := d.Dispatch(t.Context(), "missing")
	test.NotNil(t, err)
	test.Equal(t, "dispatcher: no handler registered for type string", err.Error())
}

func TestDispatcherUnregisterAll(t *testing.T) {
	d := gx.NewDispatcher()

	d.Register(func(context.Context, string) error { return nil })
	d.Register(func(context.Context, string) error { return nil })

	d.Unregister[string]()

	err := d.Dispatch(t.Context(), "value")
	test.NotNil(t, err)
}

func TestDispatcherRegisterNilPanics(t *testing.T) {
	d := gx.NewDispatcher()

	test.Panics(t, func() {
		d.Register[string](nil)
	})
}

func TestDispatcherConcurrent(t *testing.T) {
	d := gx.NewDispatcher()

	var total atomic.Int32
	d.Register(func(context.Context, int) error {
		total.Add(1)
		return nil
	})

	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			_ = d.Dispatch(t.Context(), 1)
		})
	}

	wg.Wait()
	test.Equal(t, 100, total.Load())
}
