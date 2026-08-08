package test

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// TestingT abstracts *testing.T so assertions can be tested directly
type TestingT interface {
	// Fatal is called when an assertion fails
	Fatal(args ...any)
	// Helper marks the calling function as a test helper function
	Helper()
}

// Ordered is a constraint that permits any type that supports the <, <=, >, >= operators
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~string
}

// formatMsg formats the optional message and arguments for inclusion in the fail message
func formatMsg(msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if len(msgAndArgs) == 1 {
		return fmt.Sprintf(": %v", msgAndArgs[0])
	}
	return fmt.Sprintf(": "+msgAndArgs[0].(string), msgAndArgs[1:]...)
}

// fail constructs a detailed error message including a stack trace and fails the test
func fail(t TestingT, msg string, msgAndArgs ...any) {
	t.Helper()

	var b strings.Builder
	b.WriteString("assertion failed: " + msg + formatMsg(msgAndArgs...) + "\n\nstack trace:\n")

	// skip first n frames until out of test package
	for i := 1; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		name := fn.Name()

		if strings.Contains(name, "/test.") ||
			strings.Contains(name, "runtime.") ||
			strings.Contains(name, "testing.") {
			continue
		}
		fmt.Fprintf(&b, "%s:%d - %s\n", file, line, fn.Name())
	}

	t.Fatal(b.String())
}

// Contains asserts that haystack contains needle
func Contains(t TestingT, haystack, needle any, msgAndArgs ...any) {
	t.Helper()
	v := reflect.ValueOf(haystack)
	switch v.Kind() {
	case reflect.String:
		if !strings.Contains(v.String(), fmt.Sprint(needle)) {
			fail(t, fmt.Sprintf("expected '%s' to contain '%s'", haystack, needle), msgAndArgs...)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if reflect.DeepEqual(v.Index(i).Interface(), needle) {
				return
			}
		}
		fail(t, fmt.Sprintf("expected '%v' to contain '%v'", haystack, needle), msgAndArgs...)
	case reflect.Map:
		for _, k := range v.MapKeys() {
			if reflect.DeepEqual(k.Interface(), needle) {
				return
			}
		}
		fail(t, fmt.Sprintf("expected map to contain key '%v'", needle), msgAndArgs...)
	default:
		fail(t, fmt.Sprintf("unsupported type %T in Contains", haystack), msgAndArgs...)
	}
}

// Empty asserts that the given string is empty
func Empty(t TestingT, s string, msgAndArgs ...any) {
	t.Helper()
	if s != "" {
		fail(t, "string is not empty", msgAndArgs...)
	}
}

// Equal asserts that the expected and actual values are equal
func Equal[T any](t TestingT, expected, actual T, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		fail(
			t,
			fmt.Sprintf("expected: '%v' (%T), got: '%v' (%T)", expected, expected, actual, actual),
			msgAndArgs...,
		)
	}
}

// Error asserts that the actual error matches the expected error
func Error(t TestingT, actual error, expected error, msgAndArgs ...any) {
	t.Helper()
	if actual == nil && expected == nil {
		return
	}
	if actual == nil || expected == nil {
		fail(
			t,
			fmt.Sprintf("one error is nil: expected '%v', got '%v'", expected, actual),
			msgAndArgs...,
		)
		return
	}
	if !errors.Is(actual, expected) && actual.Error() != expected.Error() {
		fail(
			t,
			fmt.Sprintf("expected error '%v', got '%v'", expected, actual),
			msgAndArgs...,
		)
	}
}

// False asserts that the given condition is false
func False(t TestingT, condition bool, msgAndArgs ...any) {
	t.Helper()
	if condition {
		fail(t, "expected false, got true", msgAndArgs...)
	}
}

// Greater asserts that actual is greater than min
func Greater[T Ordered](t TestingT, actual, min T, msgAndArgs ...any) {
	t.Helper()
	if !(actual > min) {
		fail(t, fmt.Sprintf("expected '%v' to be greater than '%v'", actual, min), msgAndArgs...)
	}
}

// GreaterOrEqual asserts that actual is greater than or equal to min
func GreaterOrEqual[T Ordered](t TestingT, actual, min T, msgAndArgs ...any) {
	t.Helper()
	if !(actual >= min) {
		fail(
			t,
			fmt.Sprintf("expected '%v' to be greater than or equal to '%v'", actual, min),
			msgAndArgs...,
		)
	}
}

// Len asserts that the given object's length matches the expected length
func Len(t TestingT, object any, expected int, msgAndArgs ...any) {
	t.Helper()
	v := reflect.ValueOf(object)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		actual := v.Len()
		if actual != expected {
			fail(
				t,
				fmt.Sprintf("expected length '%d' but got '%d'", expected, actual),
				msgAndArgs...,
			)
		}
	default:
		fail(
			t,
			fmt.Sprintf("invalid type %T (must be array, slice, map, string, or channel)", object),
			msgAndArgs...,
		)
	}
}

// Less asserts that actual is less than max
func Less[T Ordered](t TestingT, actual, max T, msgAndArgs ...any) {
	t.Helper()
	if !(actual < max) {
		fail(t, fmt.Sprintf("expected '%v' to be less than '%v'", actual, max), msgAndArgs...)
	}
}

// LessOrEqual asserts that actual is less than or equal to max
func LessOrEqual[T Ordered](t TestingT, actual, max T, msgAndArgs ...any) {
	t.Helper()
	if !(actual <= max) {
		fail(
			t,
			fmt.Sprintf("expected '%v' to be less than or equal to '%v'", actual, max),
			msgAndArgs...,
		)
	}
}

// Nil asserts that the given value is nil
func Nil(t TestingT, v any, msgAndArgs ...any) {
	t.Helper()
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			return
		}
	}
	fail(t, "value is not nil", msgAndArgs...)
}

// NoError asserts that the given error is nil
func NoError(t TestingT, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		fail(t, fmt.Sprintf("unexpected error: '%v'", err), msgAndArgs...)
	}
}

// NotEmpty asserts that the given string is not empty
func NotEmpty(t TestingT, s string, msgAndArgs ...any) {
	t.Helper()
	if s == "" {
		fail(t, "string is empty", msgAndArgs...)
	}
}

// NotEqual asserts that the expected and actual values are not equal
func NotEqual[T any](t TestingT, expected, actual T, msgAndArgs ...any) {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		fail(t, fmt.Sprintf("expected values to differ, but both were '%v'", actual), msgAndArgs...)
	}
}

// NotNil asserts that the given value is not nil
func NotNil(t TestingT, v any, msgAndArgs ...any) {
	t.Helper()
	if v == nil {
		fail(t, "value is nil", msgAndArgs...)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			fail(t, "value is nil", msgAndArgs...)
		}
	}
}

// Panics asserts that the given function panics
func Panics(t TestingT, f func(), msgAndArgs ...any) {
	defer func() {
		if r := recover(); r == nil {
			fail(t, "expected panic, but function did not panic", msgAndArgs...)
		}
	}()
	f()
}

// Same asserts that expected and actual refer to the same object
func Same(t TestingT, expected, actual any, msgAndArgs ...any) {
	t.Helper()

	if expected == nil || actual == nil {
		if expected == actual {
			return
		}

		fail(
			t,
			fmt.Sprintf("expected same object, got expected '%v', got '%v'", expected, actual),
			msgAndArgs...,
		)
		return
	}

	ev := reflect.ValueOf(expected)
	av := reflect.ValueOf(actual)

	if ev.Type() != av.Type() {
		fail(
			t,
			fmt.Sprintf("expected same type '%v', got '%v'", ev.Type(), av.Type()),
			msgAndArgs...,
		)
		return
	}

	switch ev.Kind() {
	case reflect.Pointer,
		reflect.Map,
		reflect.Slice,
		reflect.Func,
		reflect.Chan:

		if ev.Pointer() != av.Pointer() {
			fail(
				t,
				fmt.Sprintf(
					"expected same object, got '%#x' and '%#x'", ev.Pointer(), av.Pointer(),
				),
				msgAndArgs...,
			)
		}

	default:
		fail(
			t,
			fmt.Sprintf("Same only supports pointer-like types, got '%T'", expected),
			msgAndArgs...,
		)
	}
}

// True asserts that the given condition is true
func True(t TestingT, condition bool, msgAndArgs ...any) {
	t.Helper()
	if !condition {
		fail(t, "expected true, got false", msgAndArgs...)
	}
}

// Type asserts that the types of a and b are the same
func Type(t TestingT, a, b any, msgAndArgs ...any) {
	t.Helper()
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)
	if ta != tb {
		fail(t, fmt.Sprintf("expected type %v but got %v", tb, ta), msgAndArgs...)
	}
}
