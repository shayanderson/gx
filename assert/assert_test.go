package assert

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

// noPanic runs an assertion that should not panic.
func noPanic(t *testing.T, f func()) {
	t.Helper()
	_, file, line, _ := runtime.Caller(1)
	defer func() {
		if r := recover(); r != nil {
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

func panicMessage(t *testing.T, f func()) (message string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, but did not panic")
		}

		var ok bool
		message, ok = r.(string)
		if !ok {
			t.Fatalf("expected string panic, but got %T", r)
		}
	}()
	f()
	return ""
}

func TestEmpty(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { Empty("") })
	panics(t, func() { Empty("not empty") })
}

func TestEqual(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { Equal(5, 5) })
	panics(t, func() { Equal(5, 6) })

	message := panicMessage(t, func() { Equal(1, 2) })
	if !strings.Contains(message, "expected: '1', got: '2'") {
		t.Fatalf("unexpected equality failure message: %q", message)
	}
	if strings.Contains(message, "(int)") {
		t.Fatalf("equality failure repeats the shared type: %q", message)
	}

	message = panicMessage(t, func() { Equal([]int(nil), []int{}) })
	if !strings.Contains(message, "[]int(nil)") || !strings.Contains(message, "[]int{}") {
		t.Fatalf("equality failure does not distinguish nil and empty slices: %q", message)
	}
}

func TestFormatEqualFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected any
		actual   any
		want     string
	}{
		{
			name:     "same type uses readable values without repeated type",
			expected: 1,
			actual:   2,
			want:     "expected: '1', got: '2'",
		},
		{
			name:     "identical rendered values use Go syntax",
			expected: []int(nil),
			actual:   []int{},
			want:     "expected: '[]int(nil)', got: '[]int{}'",
		},
		{
			name:     "different dynamic types include both type labels",
			expected: int(1),
			actual:   int64(1),
			want:     "expected: '1' (int), got: '1' (int64)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatEqualFailure(tt.expected, tt.actual); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestError(t *testing.T) {
	t.Parallel()

	e1 := errors.New("a")
	e2 := errors.New("a")

	noPanic(t, func() { Error(nil, nil) }) // both nil ok
	noPanic(t, func() { Error(e1, e1) })   // same instance
	noPanic(t, func() { Error(e1, fmt.Errorf("wrapped: %w", e1)) })
	panics(t, func() { Error(e1, e2) })  // same message, distinct error
	panics(t, func() { Error(e1, nil) }) // nil mismatch
}

func TestFalse(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { False(false) })
	panics(t, func() { False(true) })
}

func TestGreaterAndGreaterOrEqual(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { Greater(5, 1) })
	panics(t, func() { Greater(1, 5) })
	noPanic(t, func() { GreaterOrEqual(5, 5) })
	panics(t, func() { GreaterOrEqual(4, 5) })
}

func TestLessAndLessOrEqual(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { Less(1, 5) })
	panics(t, func() { Less(5, 1) })
	noPanic(t, func() { LessOrEqual(5, 5) })
	panics(t, func() { LessOrEqual(6, 5) })
}

func TestLen(t *testing.T) {
	t.Parallel()

	var nilSlice []int
	var nilMap map[string]int
	var nilChan chan int
	var nilArray *[3]int
	array := [3]int{1, 2, 3}
	value := 1
	ch := make(chan int, 2)
	ch <- 1

	noPanic(t, func() { Len(3, []int{1, 2, 3}) })
	noPanic(t, func() { Len(1, map[string]int{"one": 1}) })
	noPanic(t, func() { Len(3, "abc") })
	noPanic(t, func() { Len(1, ch) })
	noPanic(t, func() { Len(3, &array) })
	noPanic(t, func() { Len(3, nilArray) })
	noPanic(t, func() { Len(0, nilSlice) })
	noPanic(t, func() { Len(0, nilMap) })
	noPanic(t, func() { Len(0, nilChan) })
	panics(t, func() { Len(2, []int{1}) })
	panics(t, func() { Len(2, &array) })
	panics(t, func() { Len(1, &value) }) // pointer to non-array
	panics(t, func() { Len(1, 123) })    // invalid type
}

func TestNilAndNotNil(t *testing.T) {
	t.Parallel()

	var ptr *int
	var m map[string]int
	var s []string
	var ch chan int
	var fn func()
	var unsafePtr unsafe.Pointer

	noPanic(t, func() { Nil(nil) })
	noPanic(t, func() { Nil(ptr) })
	noPanic(t, func() { Nil(m) })
	noPanic(t, func() { Nil(s) })
	noPanic(t, func() { Nil(ch) })
	noPanic(t, func() { Nil(fn) })
	noPanic(t, func() { Nil(unsafePtr) })
	panics(t, func() { Nil(1) })

	var x int
	noPanic(t, func() { NotNil(&x) })
	noPanic(t, func() { NotNil(map[string]int{}) })
	noPanic(t, func() { NotNil([]string{}) })
	noPanic(t, func() { NotNil(make(chan int)) })
	noPanic(t, func() { NotNil(func() {}) })
	noPanic(t, func() { NotNil(unsafe.Pointer(&x)) })
	panics(t, func() { NotNil(nil) })
	panics(t, func() { NotNil(ptr) })
	panics(t, func() { NotNil(m) })
	panics(t, func() { NotNil(s) })
	panics(t, func() { NotNil(ch) })
	panics(t, func() { NotNil(fn) })
	panics(t, func() { NotNil(unsafePtr) })
}

func TestNoError(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { NoError(nil) })
	panics(t, func() { NoError(errors.New("boom")) })
}

func TestNotEmpty(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { NotEmpty("ok") })
	panics(t, func() { NotEmpty("") })
}

func TestNotEqual(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { NotEqual(1, 2) })
	panics(t, func() { NotEqual(5, 5) })
}

func TestTrue(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { True(true) })
	panics(t, func() { True(false) })
}

func TestType(t *testing.T) {
	t.Parallel()

	noPanic(t, func() { Type(1, 2) })
	panics(t, func() { Type(1, "string") })

	message := panicMessage(t, func() { Type(1, "string") })
	if !strings.Contains(message, "expected type int but got string") {
		t.Fatalf("type mismatch labels are incorrect: %q", message)
	}
}

func TestFormatMsg(t *testing.T) {
	t.Parallel()

	expect := ": custom message: details here: 23"
	format := "custom message: %s: %d"
	result := formatMsg(format, "details here", 23)
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	expect = ": no details"
	result = formatMsg("no details")
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	expect = ": 100% bad"
	percentMessage := "100% bad"
	result = formatMsg(percentMessage)
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}

	args := []any{errors.New("context"), "details"}
	expect = ": " + fmt.Sprint(args...)
	result = formatMsg(args...)
	if result != expect {
		t.Fatalf("expected '%s', got '%s'", expect, result)
	}
}

func TestFailureMessages(t *testing.T) {
	t.Parallel()

	message := panicMessage(t, func() { True(false, errors.New("context"), "details") })
	if !strings.Contains(message, "assertion failed: expected true, got false") {
		t.Fatalf("assertion message missing from panic: %q", message)
	}
	if !strings.Contains(message, ": "+fmt.Sprint(errors.New("context"), "details")) {
		t.Fatalf("context missing from panic: %q", message)
	}

	format := "request %s failed"
	message = panicMessage(t, func() { Equal(1, 2, format, "abc") })
	if !strings.Contains(message, ": request abc failed") {
		t.Fatalf("formatted context missing from panic: %q", message)
	}

	loneMessage := "100% bad"
	message = panicMessage(t, func() { True(false, loneMessage) })
	if !strings.Contains(message, ": 100% bad") {
		t.Fatalf("literal context missing from panic: %q", message)
	}
}
