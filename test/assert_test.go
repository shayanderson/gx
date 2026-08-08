package test

import (
	"errors"
	"fmt"
	"testing"
)

// fakeT simulates *testing.T for internal assert testing
type fakeT struct {
	failed bool
	msg    string
}

func (f *fakeT) Fatal(args ...any) {
	f.failed = true
	f.msg = fmt.Sprint(args...)
}

func (f *fakeT) Helper() {}

// expectFail ensures the assert triggers a failure
func expectFail(t *testing.T, fn func(f *fakeT)) {
	t.Helper()
	f := &fakeT{}
	fn(f)
	if !f.failed {
		t.Fatalf("expected failure but got none")
	}
}

// expectPass ensures the assert does not trigger a failure
func expectPass(t *testing.T, fn func(f *fakeT)) {
	t.Helper()
	f := &fakeT{}
	fn(f)
	if f.failed {
		t.Fatalf("expected pass but it failed: %v", f.msg)
	}
}

func TestContains(t *testing.T) {
	expectPass(t, func(f *fakeT) { Contains(f, "hello world", "world") })
	expectFail(t, func(f *fakeT) { Contains(f, "hello", "nope") })

	expectPass(t, func(f *fakeT) { Contains(f, []int{1, 2, 3}, 2) })
	expectFail(t, func(f *fakeT) { Contains(f, []int{1, 2, 3}, 4) })

	expectPass(t, func(f *fakeT) { Contains(f, map[string]int{"a": 1}, "a") })
	expectFail(t, func(f *fakeT) { Contains(f, map[string]int{"a": 1}, "b") })

	expectFail(t, func(f *fakeT) { Contains(f, 123, 1) })
}

func TestEmpty(t *testing.T) {
	expectPass(t, func(f *fakeT) { Empty(f, "") })
	expectFail(t, func(f *fakeT) { Empty(f, "x") })
}

func TestEqual(t *testing.T) {
	expectPass(t, func(f *fakeT) { Equal(f, 5, 5) })
	expectFail(t, func(f *fakeT) { Equal(f, 5, 6) })
}

func TestError(t *testing.T) {
	e1 := errors.New("a")
	e2 := errors.New("a")
	e3 := errors.New("b")

	expectPass(t, func(f *fakeT) { Error(f, e1, e1) })
	expectPass(t, func(f *fakeT) { Error(f, e1, e2) })
	expectFail(t, func(f *fakeT) { Error(f, e1, e3) })
	expectFail(t, func(f *fakeT) { Error(f, e1, nil) })
	expectFail(t, func(f *fakeT) { Error(f, nil, e1) })
	expectPass(t, func(f *fakeT) { Error(f, nil, nil) })
}

func TestFalse(t *testing.T) {
	expectPass(t, func(f *fakeT) { False(f, false) })
	expectFail(t, func(f *fakeT) { False(f, true) })
}

func TestGreater(t *testing.T) {
	expectPass(t, func(f *fakeT) { Greater(f, 5, 3) })
	expectFail(t, func(f *fakeT) { Greater(f, 3, 5) })
}

func TestGreaterOrEqual(t *testing.T) {
	expectPass(t, func(f *fakeT) { GreaterOrEqual(f, 5, 5) })
	expectFail(t, func(f *fakeT) { GreaterOrEqual(f, 3, 5) })
}

func TestLen(t *testing.T) {
	expectPass(t, func(f *fakeT) { Len(f, []int{1, 2, 3}, 3) })
	expectFail(t, func(f *fakeT) { Len(f, []int{1}, 2) })
	expectFail(t, func(f *fakeT) { Len(f, 123, 1) })
}

func TestLess(t *testing.T) {
	expectPass(t, func(f *fakeT) { Less(f, 3, 5) })
	expectFail(t, func(f *fakeT) { Less(f, 5, 3) })
}

func TestLessOrEqual(t *testing.T) {
	expectPass(t, func(f *fakeT) { LessOrEqual(f, 3, 5) })
	expectPass(t, func(f *fakeT) { LessOrEqual(f, 5, 5) })
	expectFail(t, func(f *fakeT) { LessOrEqual(f, 6, 5) })
}

func TestNil(t *testing.T) {
	var p *int
	var m map[string]int
	var s []string
	var f func()

	expectPass(t, func(fa *fakeT) { Nil(fa, nil) })
	expectPass(t, func(fa *fakeT) { Nil(fa, p) })
	expectPass(t, func(fa *fakeT) { Nil(fa, m) })
	expectPass(t, func(fa *fakeT) { Nil(fa, s) })
	expectPass(t, func(fa *fakeT) { Nil(fa, f) })
	expectFail(t, func(fa *fakeT) { Nil(fa, 1) })
}

func TestNoError(t *testing.T) {
	expectPass(t, func(f *fakeT) { NoError(f, nil) })
	expectFail(t, func(f *fakeT) { NoError(f, errors.New("fail")) })
}

func TestNotEmpty(t *testing.T) {
	expectPass(t, func(f *fakeT) { NotEmpty(f, "x") })
	expectFail(t, func(f *fakeT) { NotEmpty(f, "") })
}

func TestNotEqual(t *testing.T) {
	expectPass(t, func(f *fakeT) { NotEqual(f, 5, 6) })
	expectFail(t, func(f *fakeT) { NotEqual(f, 5, 5) })
}

func TestNotNil(t *testing.T) {
	var p *int
	var m map[string]int
	expectPass(t, func(f *fakeT) { NotNil(f, 1) })
	expectFail(t, func(f *fakeT) { NotNil(f, nil) })
	expectFail(t, func(f *fakeT) { NotNil(f, p) })
	expectFail(t, func(f *fakeT) { NotNil(f, m) })
}

func TestPanics(t *testing.T) {
	expectPass(t, func(f *fakeT) { Panics(f, func() { panic("ok") }) })
	expectFail(t, func(f *fakeT) { Panics(f, func() {}) })
}

func TestSame(t *testing.T) {
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
	expectPass(t, func(f *fakeT) { True(f, true) })
	expectFail(t, func(f *fakeT) { True(f, false) })
}

func TestType(t *testing.T) {
	expectPass(t, func(f *fakeT) { Type(f, 1, 2) })
	expectFail(t, func(f *fakeT) { Type(f, 1, "s") })
}

func TestFormatMsg(t *testing.T) {
	msg := formatMsg("expected %v but got %v", 5, 6)
	expected := ": expected 5 but got 6"
	if msg != expected {
		t.Fatalf("expected '%v' but got '%v'", expected, msg)
	}

	msg = formatMsg("no details")
	expected = ": no details"
	if msg != expected {
		t.Fatalf("expected '%v' but got '%v'", expected, msg)
	}
}
