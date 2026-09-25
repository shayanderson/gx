package test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

// fakeT simulates *testing.T for internal assert testing
type fakeT struct {
	failed  bool
	helperN int
	msg     string
}

func (f *fakeT) Fatal(args ...any) {
	f.failed = true
	f.msg = fmt.Sprint(args...)
	runtime.Goexit()
}

func (f *fakeT) Helper() { f.helperN++ }

// runAssertion executes an assertion in its own goroutine so fakeT.Fatal can
// match testing.T.Fatal's Goexit behavior.
func runAssertion(fn func(f *fakeT)) *fakeT {
	result := make(chan *fakeT, 1)
	go func() {
		f := &fakeT{}
		defer func() { result <- f }()
		fn(f)
	}()
	return <-result
}

// expectFail ensures the assert triggers a failure.
func expectFail(t *testing.T, fn func(f *fakeT)) *fakeT {
	t.Helper()
	f := runAssertion(fn)
	if !f.failed {
		t.Fatalf("expected failure but got none")
	}
	return f
}

// expectPass ensures the assert does not trigger a failure.
func expectPass(t *testing.T, fn func(f *fakeT)) {
	t.Helper()
	f := runAssertion(fn)
	if f.failed {
		t.Fatalf("expected pass but it failed: %v", f.msg)
	}
}

func TestContains(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { Contains(f, "hello world", "world") })
	expectFail(t, func(f *fakeT) { Contains(f, "hello", "nope") })
	expectFail(t, func(f *fakeT) { Contains(f, "abc", 'a') })

	expectPass(t, func(f *fakeT) { Contains(f, []int{1, 2, 3}, 2) })
	expectFail(t, func(f *fakeT) { Contains(f, []int{1, 2, 3}, 4) })

	expectPass(t, func(f *fakeT) { Contains(f, map[string]int{"a": 1}, "a") })
	expectFail(t, func(f *fakeT) { Contains(f, map[string]int{"a": 1}, "b") })

	expectFail(t, func(f *fakeT) { Contains(f, 123, 1) })
}

func TestEmpty(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { Empty(f, "") })
	expectFail(t, func(f *fakeT) { Empty(f, "x") })
}

func TestEqual(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { Equal(f, 5, 5) })
	expectFail(t, func(f *fakeT) { Equal(f, 5, 6) })

	f := expectFail(t, func(f *fakeT) { Equal(f, []int(nil), []int{}) })
	if !strings.Contains(f.msg, "[]int(nil)") || !strings.Contains(f.msg, "[]int{}") {
		t.Fatalf("equality failure does not distinguish nil and empty slices: %s", f.msg)
	}

	f = expectFail(t, func(f *fakeT) { Equal(f, 1, 2) })
	if strings.Contains(f.msg, "(int)") {
		t.Fatalf("equality failure repeats the shared type: %s", f.msg)
	}

	f = expectFail(t, func(f *fakeT) { Equal[any](f, 1, "2") })
	if !strings.Contains(f.msg, "(int)") || !strings.Contains(f.msg, "(string)") {
		t.Fatalf("equality failure does not include differing types: %s", f.msg)
	}
}

func TestErrorIs(t *testing.T) {
	t.Parallel()

	e1 := errors.New("a")
	e2 := errors.New("a")
	e3 := errors.New("b")

	expectPass(t, func(f *fakeT) { ErrorIs(f, e1, e1) })
	expectPass(t, func(f *fakeT) { ErrorIs(f, fmt.Errorf("wrapped: %w", e1), e1) })
	expectFail(t, func(f *fakeT) { ErrorIs(f, e1, e2) })
	f := expectFail(t, func(f *fakeT) { ErrorIs(f, e1, e3) })
	if !strings.Contains(f.msg, "expected error 'b', got 'a'") {
		t.Fatalf("unexpected error assertion message: %s", f.msg)
	}
	expectFail(t, func(f *fakeT) { ErrorIs(f, e1, nil) })
	expectFail(t, func(f *fakeT) { ErrorIs(f, nil, e1) })
	expectPass(t, func(f *fakeT) { ErrorIs(f, nil, nil) })
}

func TestErrorMessage(t *testing.T) {
	t.Parallel()

	expectPass(
		t,
		func(f *fakeT) { ErrorMessage(f, errors.New("agent id is empty"), "agent id is empty") },
	)
	expectFail(t, func(f *fakeT) { ErrorMessage(f, nil, "agent id is empty") })

	f := expectFail(t, func(f *fakeT) {
		ErrorMessage(f, errors.New("different message"), "agent id is empty")
	})
	if !strings.Contains(
		f.msg,
		`expected error message "agent id is empty", got "different message"`,
	) {
		t.Fatalf("unexpected error message assertion failure: %s", f.msg)
	}
}

func TestFalse(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { False(f, false) })
	expectFail(t, func(f *fakeT) { False(f, true) })
}

func TestGreater(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { Greater(f, 5, 3) })
	expectFail(t, func(f *fakeT) { Greater(f, 3, 5) })
}

func TestGreaterOrEqual(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { GreaterOrEqual(f, 5, 5) })
	expectFail(t, func(f *fakeT) { GreaterOrEqual(f, 3, 5) })
}

func TestLen(t *testing.T) {
	t.Parallel()

	array := [3]int{}
	var nilArray *[3]int
	value := 1

	expectPass(t, func(f *fakeT) { Len(f, 3, []int{1, 2, 3}) })
	expectPass(t, func(f *fakeT) { Len(f, 3, &array) })
	expectPass(t, func(f *fakeT) { Len(f, 3, nilArray) })
	expectFail(t, func(f *fakeT) { Len(f, 2, []int{1}) })
	expectFail(t, func(f *fakeT) { Len(f, 1, &value) })
	expectFail(t, func(f *fakeT) { Len(f, 1, 123) })
}

func TestLess(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { Less(f, 3, 5) })
	expectFail(t, func(f *fakeT) { Less(f, 5, 3) })
}

func TestLessOrEqual(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { LessOrEqual(f, 3, 5) })
	expectPass(t, func(f *fakeT) { LessOrEqual(f, 5, 5) })
	expectFail(t, func(f *fakeT) { LessOrEqual(f, 6, 5) })
}

func TestNil(t *testing.T) {
	t.Parallel()

	var p *int
	var m map[string]int
	var s []string
	var f func()
	var unsafePtr unsafe.Pointer

	expectPass(t, func(fa *fakeT) { Nil(fa, nil) })
	expectPass(t, func(fa *fakeT) { Nil(fa, p) })
	expectPass(t, func(fa *fakeT) { Nil(fa, m) })
	expectPass(t, func(fa *fakeT) { Nil(fa, s) })
	expectPass(t, func(fa *fakeT) { Nil(fa, f) })
	expectPass(t, func(fa *fakeT) { Nil(fa, unsafePtr) })
	expectFail(t, func(fa *fakeT) { Nil(fa, 1) })
}

func TestNoError(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { NoError(f, nil) })
	expectFail(t, func(f *fakeT) { NoError(f, errors.New("fail")) })
}

func TestNotEmpty(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { NotEmpty(f, "x") })
	expectFail(t, func(f *fakeT) { NotEmpty(f, "") })
}

func TestNotEqual(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { NotEqual(f, 5, 6) })
	expectFail(t, func(f *fakeT) { NotEqual(f, 5, 5) })
}

func TestNotNil(t *testing.T) {
	t.Parallel()

	var p *int
	var m map[string]int
	var unsafePtr unsafe.Pointer
	expectPass(t, func(f *fakeT) { NotNil(f, 1) })
	expectFail(t, func(f *fakeT) { NotNil(f, nil) })
	expectFail(t, func(f *fakeT) { NotNil(f, p) })
	expectFail(t, func(f *fakeT) { NotNil(f, m) })
	expectFail(t, func(f *fakeT) { NotNil(f, unsafePtr) })
}

func TestPanics(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { Panics(f, func() { panic("ok") }) })
	expectFail(t, func(f *fakeT) { Panics(f, func() {}) })

	f := &fakeT{}
	Panics(f, func() { panic("ok") })
	if f.helperN == 0 {
		t.Fatal("Panics did not mark itself as a test helper")
	}
}

func TestSame(t *testing.T) {
	t.Parallel()

	t.Run("same_pointer", func(t *testing.T) {
		v := &struct{ X int }{X: 1}

		expectPass(t, func(f *fakeT) {
			Same(f, v, v)
		})
	})

	t.Run("different_pointers", func(t *testing.T) {
		v1 := &struct{ X int }{X: 1}
		v2 := &struct{ X int }{X: 1}

		expectFail(t, func(f *fakeT) {
			Same(f, v1, v2)
		})
	})

	t.Run("same_slice", func(t *testing.T) {
		s := []int{1, 2, 3}

		expectPass(t, func(f *fakeT) {
			Same(f, s, s)
		})
	})

	t.Run("different_slices", func(t *testing.T) {
		s1 := []int{1, 2, 3}
		s2 := []int{1, 2, 3}

		expectFail(t, func(f *fakeT) {
			Same(f, s1, s2)
		})
	})

	t.Run("same_backing_array_different_length", func(t *testing.T) {
		values := []int{1, 2, 3}

		f := expectFail(t, func(f *fakeT) {
			Same(f, values[:2], values[:3])
		})
		if !strings.Contains(f.msg, "expected same slice view") {
			t.Fatalf("unexpected Same failure message: %s", f.msg)
		}
	})

	t.Run("same_backing_array_different_capacity", func(t *testing.T) {
		values := []int{1, 2, 3}

		expectFail(t, func(f *fakeT) {
			Same(f, values[:2:2], values[:2:3])
		})
	})

	t.Run("empty_slices", func(t *testing.T) {
		expectFail(t, func(f *fakeT) {
			Same(f, []int{}, []int{})
		})
	})

	t.Run("typed_nil_values", func(t *testing.T) {
		var slice []int
		var pointer *struct{}

		expectPass(t, func(f *fakeT) {
			Same(f, slice, slice)
		})
		expectPass(t, func(f *fakeT) {
			Same(f, pointer, pointer)
		})
	})

	t.Run("zero_size_values", func(t *testing.T) {
		pointer := &struct{}{}

		expectFail(t, func(f *fakeT) {
			Same(f, pointer, pointer)
		})
		expectFail(t, func(f *fakeT) {
			Same(f, []struct{}{{}}, []struct{}{{}})
		})
	})

	t.Run("same_map", func(t *testing.T) {
		m := map[string]int{"a": 1}

		expectPass(t, func(f *fakeT) {
			Same(f, m, m)
		})
	})

	t.Run("different_maps", func(t *testing.T) {
		m1 := map[string]int{"a": 1}
		m2 := map[string]int{"a": 1}

		expectFail(t, func(f *fakeT) {
			Same(f, m1, m2)
		})
	})

	t.Run("nil_values", func(t *testing.T) {
		expectPass(t, func(f *fakeT) {
			Same(f, nil, nil)
		})
	})

	t.Run("one_nil", func(t *testing.T) {
		v := &struct{}{}

		expectFail(t, func(f *fakeT) {
			Same(f, v, nil)
		})
	})

	t.Run("different_types", func(t *testing.T) {
		v1 := &struct{}{}
		v2 := &struct{ X int }{}

		expectFail(t, func(f *fakeT) {
			Same(f, v1, v2)
		})
	})

	t.Run("unsupported_type", func(t *testing.T) {
		expectFail(t, func(f *fakeT) {
			Same(f, 1, 1)
		})
	})
}

func TestTrue(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { True(f, true) })
	expectFail(t, func(f *fakeT) { True(f, false) })
}

func TestType(t *testing.T) {
	t.Parallel()

	expectPass(t, func(f *fakeT) { Type(f, 1, 2) })
	expectFail(t, func(f *fakeT) { Type(f, 1, "s") })
}

func TestFailMessage(t *testing.T) {
	t.Parallel()

	f := runAssertion(func(f *fakeT) {
		fail(f, "values differ", "expected 42")
	})

	if !strings.Contains(f.msg, "assertion failed: values differ: expected 42") {
		t.Fatalf("unexpected failure message: %s", f.msg)
	}

	if !strings.Contains(f.msg, "stack trace:") {
		t.Fatalf("expected stack trace in failure message: %s", f.msg)
	}
}

func TestFailRejectsMultipleMessages(t *testing.T) {
	t.Parallel()

	defer func() {
		if got := recover(); got != "assertion accepts at most one message" {
			t.Fatalf("unexpected multiple-message panic: %v", got)
		}
	}()
	fail(&fakeT{}, "values differ", "one", "two")
}
