package assert

import (
	"errors"
	"runtime"
	"testing"
)

// helper for run assertion that should not panic
func noPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			_, file, line, _ := runtime.Caller(2) // caller of noPanic
			t.Fatalf("%s:%d: expected no panic, but got: %v", file, line, r)
		}
	}()
	f()
}

// helper for run assertion that should panic
func panics(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			_, file, line, _ := runtime.Caller(2) // caller of panics
			t.Fatalf("%s:%d: expected panic, but did not panic", file, line)
		}
	}()
	f()
}

func TestEmpty(t *testing.T) {
	noPanic(t, func() { Empty("") })
	panics(t, func() { Empty("not empty") })
}

func TestEqual(t *testing.T) {
	noPanic(t, func() { Equal(5, 5) })
	panics(t, func() { Equal(5, 6) })
}

func TestError(t *testing.T) {
	e1 := errors.New("a")
	e2 := errors.New("a")
	e3 := errors.New("b")

	noPanic(t, func() { Error(nil, nil) }) // both nil ok
	noPanic(t, func() { Error(e1, e1) })   // same instance
	noPanic(t, func() { Error(e1, e2) })   // same message
	panics(t, func() { Error(nil, e1) })   // nil mismatch
	panics(t, func() { Error(e1, e3) })    // different error
	noPanic(t, func() { Error(nil, nil) }) // both nil ok
}

func TestFalse(t *testing.T) {
	noPanic(t, func() { False(false) })
	panics(t, func() { False(true) })
}

func TestGreaterAndGreaterOrEqual(t *testing.T) {
	noPanic(t, func() { Greater(5, 1) })
	panics(t, func() { Greater(1, 5) })
	noPanic(t, func() { GreaterOrEqual(5, 5) })
	panics(t, func() { GreaterOrEqual(4, 5) })
}

func TestLessAndLessOrEqual(t *testing.T) {
	noPanic(t, func() { Less(1, 5) })
	panics(t, func() { Less(5, 1) })
	noPanic(t, func() { LessOrEqual(5, 5) })
	panics(t, func() { LessOrEqual(6, 5) })
}

func TestLen(t *testing.T) {
	noPanic(t, func() { Len([]int{1, 2, 3}, 3) })
	panics(t, func() { Len([]int{1}, 2) })
	panics(t, func() { Len(123, 1) }) // invalid type
}

func TestNilAndNotNil(t *testing.T) {
	var ptr *int
	var m map[string]int
	var s []string

	noPanic(t, func() { Nil(nil) })
	noPanic(t, func() { Nil(ptr) })
	noPanic(t, func() { Nil(m) })
	noPanic(t, func() { Nil(s) })
	panics(t, func() { Nil(1) })

	var x int
	noPanic(t, func() { NotNil(&x) })
	panics(t, func() { NotNil(nil) })
	panics(t, func() { NotNil(ptr) })
}

func TestNoError(t *testing.T) {
	noPanic(t, func() { NoError(nil) })
	panics(t, func() { NoError(errors.New("boom")) })
}

func TestNotEmpty(t *testing.T) {
	noPanic(t, func() { NotEmpty("ok") })
	panics(t, func() { NotEmpty("") })
}

func TestNotEqual(t *testing.T) {
	noPanic(t, func() { NotEqual(1, 2) })
	panics(t, func() { NotEqual(5, 5) })
}

func TestTrue(t *testing.T) {
	noPanic(t, func() { True(true) })
	panics(t, func() { True(false) })
}

func TestType(t *testing.T) {
	noPanic(t, func() { Type(1, 2) })
	panics(t, func() { Type(1, "string") })
}

func TestFormatMsg(t *testing.T) {
	expect := ": custom message: details here: 23"
	result := formatMsg("custom message: %s: %d", "details here", 23)
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	expect = ": no details"
	result = formatMsg("no details")
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}
}
