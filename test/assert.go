package test

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// TestingT abstracts *testing.T so assertions can be tested directly.
type TestingT interface {
	// Fatal is called when an assertion fails.
	Fatal(args ...any)

	// Helper marks the calling function as a test helper function.
	Helper()
}

// formatMsg formats the optional message and arguments for inclusion in the fail message.
func formatMsg(msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if len(msgAndArgs) == 1 {
		return fmt.Sprintf(": %v", msgAndArgs[0])
	}
	if format, ok := msgAndArgs[0].(string); ok {
		return fmt.Sprintf(": "+format, msgAndArgs[1:]...)
	}
	return ": " + fmt.Sprint(msgAndArgs...)
}

// fail constructs a detailed error message including a stack trace and fails the test.
func fail(t TestingT, msg string, msgAndArgs ...any) {
	t.Helper()

	var b strings.Builder

	b.WriteString("assertion failed: ")
	b.WriteString(msg)
	b.WriteString(formatMsg(msgAndArgs...))
	b.WriteString("\n\nstack trace:\n")

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

// Contains asserts that haystack contains needle.
func Contains(t TestingT, haystack, needle any, msgAndArgs ...any) {
	t.Helper()
	v := reflect.ValueOf(haystack)
	switch v.Kind() {
	case reflect.String:
		needleString, ok := needle.(string)
		if !ok {
			fail(
				t,
				fmt.Sprintf("string haystack requires string needle, got %T", needle),
				msgAndArgs...)
			return
		}
		if !strings.Contains(v.String(), needleString) {
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

// Empty asserts that the given string is empty.
func Empty(t TestingT, s string, msgAndArgs ...any) {
	t.Helper()
	if s != "" {
		fail(t, "string is not empty", msgAndArgs...)
	}
}

// Equal asserts that expected and actual are deeply equal according to reflect.DeepEqual.
// Use time.Time.Equal for instant comparisons and math.IsNaN for NaN values.
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

// ErrorIs asserts that err matches target according to errors.Is.
func ErrorIs(t TestingT, err, target error, msgAndArgs ...any) {
	t.Helper()
	if err == nil && target == nil {
		return
	}
	if err == nil || target == nil {
		fail(
			t,
			fmt.Sprintf("one error is nil: expected '%v', got '%v'", target, err),
			msgAndArgs...,
		)
		return
	}
	if !errors.Is(err, target) {
		fail(
			t,
			fmt.Sprintf("expected error '%v', got '%v'", target, err),
			msgAndArgs...,
		)
	}
}

// ErrorMessage asserts that err has the expected error message.
func ErrorMessage(t TestingT, err error, expected string, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		fail(t, fmt.Sprintf("expected error message %q, got nil", expected), msgAndArgs...)
		return
	}
	if err.Error() != expected {
		fail(
			t,
			fmt.Sprintf("expected error message %q, got %q", expected, err.Error()),
			msgAndArgs...,
		)
	}
}

// False asserts that the given condition is false.
func False(t TestingT, condition bool, msgAndArgs ...any) {
	t.Helper()
	if condition {
		fail(t, "expected false, got true", msgAndArgs...)
	}
}

// Greater asserts that actual is greater than minimum.
func Greater[T cmp.Ordered](t TestingT, actual, minimum T, msgAndArgs ...any) {
	t.Helper()
	if !(actual > minimum) {
		fail(
			t,
			fmt.Sprintf("expected '%v' to be greater than '%v'", actual, minimum),
			msgAndArgs...)
	}
}

// GreaterOrEqual asserts that actual is greater than or equal to minimum.
func GreaterOrEqual[T cmp.Ordered](t TestingT, actual, minimum T, msgAndArgs ...any) {
	t.Helper()
	if !(actual >= minimum) {
		fail(
			t,
			fmt.Sprintf("expected '%v' to be greater than or equal to '%v'", actual, minimum),
			msgAndArgs...,
		)
	}
}

// Len asserts that expected matches object's length.
func Len(t TestingT, expected int, object any, msgAndArgs ...any) {
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

// Less asserts that actual is less than maximum.
func Less[T cmp.Ordered](t TestingT, actual, maximum T, msgAndArgs ...any) {
	t.Helper()
	if !(actual < maximum) {
		fail(t, fmt.Sprintf("expected '%v' to be less than '%v'", actual, maximum), msgAndArgs...)
	}
}

// LessOrEqual asserts that actual is less than or equal to maximum.
func LessOrEqual[T cmp.Ordered](t TestingT, actual, maximum T, msgAndArgs ...any) {
	t.Helper()
	if !(actual <= maximum) {
		fail(
			t,
			fmt.Sprintf("expected '%v' to be less than or equal to '%v'", actual, maximum),
			msgAndArgs...,
		)
	}
}

// Nil asserts that the given value is nil.
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
	case reflect.UnsafePointer:
		if rv.Pointer() == 0 {
			return
		}
	}
	fail(t, "value is not nil", msgAndArgs...)
}

// NoError asserts that the given error is nil.
func NoError(t TestingT, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		fail(t, fmt.Sprintf("unexpected error: '%v'", err), msgAndArgs...)
	}
}

// NotEmpty asserts that the given string is not empty.
func NotEmpty(t TestingT, s string, msgAndArgs ...any) {
	t.Helper()
	if s == "" {
		fail(t, "string is empty", msgAndArgs...)
	}
}

// NotEqual asserts that expected and actual are not deeply equal according to reflect.DeepEqual.
func NotEqual[T any](t TestingT, expected, actual T, msgAndArgs ...any) {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		fail(t, fmt.Sprintf("expected values to differ, but both were '%v'", actual), msgAndArgs...)
	}
}

// NotNil asserts that the given value is not nil.
func NotNil(t TestingT, v any, msgAndArgs ...any) {
	t.Helper()
	if v == nil {
		fail(t, "value is nil", msgAndArgs...)
		return
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			fail(t, "value is nil", msgAndArgs...)
		}
	case reflect.UnsafePointer:
		if rv.Pointer() == 0 {
			fail(t, "value is nil", msgAndArgs...)
		}
	}
}

// Panics asserts that the given function panics.
func Panics(t TestingT, f func(), msgAndArgs ...any) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			fail(t, "expected panic, but function did not panic", msgAndArgs...)
		}
	}()
	f()
}

// Same asserts that expected and actual refer to the same object or slice view.
// Zero-length slices and zero-size values are rejected because their identity is
// not reliably observable.
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
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		if ev.IsNil() && av.IsNil() {
			return
		}
	}

	switch ev.Kind() {
	case reflect.Pointer:
		if ev.Type().Elem().Size() == 0 {
			fail(t, "Same cannot determine identity for zero-size values", msgAndArgs...)
			return
		}
		if ev.Pointer() != av.Pointer() {
			fail(
				t,
				fmt.Sprintf(
					"expected same object, got '%#x' and '%#x'", ev.Pointer(), av.Pointer(),
				),
				msgAndArgs...,
			)
		}

	case reflect.Slice:
		if ev.Len() == 0 || ev.Type().Elem().Size() == 0 {
			fail(
				t,
				"Same cannot determine identity for zero-length or zero-size slices",
				msgAndArgs...)
			return
		}
		if ev.Pointer() != av.Pointer() || ev.Len() != av.Len() || ev.Cap() != av.Cap() {
			fail(t, "expected same slice view", msgAndArgs...)
		}

	case reflect.Map, reflect.Func, reflect.Chan:
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

// True asserts that the given condition is true.
func True(t TestingT, condition bool, msgAndArgs ...any) {
	t.Helper()
	if !condition {
		fail(t, "expected true, got false", msgAndArgs...)
	}
}

// Type asserts that expected and actual have the same dynamic type.
func Type(t TestingT, expected, actual any, msgAndArgs ...any) {
	t.Helper()
	expectedType := reflect.TypeOf(expected)
	actualType := reflect.TypeOf(actual)
	if expectedType != actualType {
		fail(t, fmt.Sprintf("expected type %v but got %v", expectedType, actualType), msgAndArgs...)
	}
}
