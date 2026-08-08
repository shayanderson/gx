package assert

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

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

// fail constructs a detailed error message including a stack trace and panics
func fail(msg string, msgAndArgs ...any) {
	var b strings.Builder
	b.WriteString("assertion failed: " + msg + formatMsg(msgAndArgs...) + "\n\nstack trace:\n")

	// skip first n frames until out of assert package
	for i := 1; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		name := fn.Name()

		if strings.Contains(name, "/assert.") || strings.Contains(name, "runtime.") {
			continue
		}
		fmt.Fprintf(&b, "at %s:%d\n", file, line)
	}
	panic(b.String())
}

// Empty asserts that the given string is empty
func Empty(s string, msgAndArgs ...any) {
	if s != "" {
		fail("string is not empty", msgAndArgs...)
	}
}

// Equal asserts that the expected and actual values are equal
func Equal[T any](expected, actual T, msgAndArgs ...any) {
	if !reflect.DeepEqual(expected, actual) {
		fail(
			fmt.Sprintf("expected: '%v' (%T), got: '%v' (%T)", expected, expected, actual, actual),
			msgAndArgs...,
		)
	}
}

// Error asserts that the actual error matches the expected error
func Error(actual error, expected error, msgAndArgs ...any) {
	if actual == nil && expected == nil {
		return
	}
	if actual == nil || expected == nil {
		fail(
			fmt.Sprintf("one error is nil: expected '%v', got '%v'", expected, actual),
			msgAndArgs...,
		)
		return
	}
	if !errors.Is(actual, expected) && actual.Error() != expected.Error() {
		fail(
			fmt.Sprintf("expected error '%v', got '%v'", expected, actual),
			msgAndArgs...,
		)
	}
}

// False asserts that the given condition is false
func False(condition bool, msgAndArgs ...any) {
	if condition {
		fail("expected false, got true", msgAndArgs...)
	}
}

// Greater asserts that actual is greater than min
func Greater[T Ordered](actual, min T, msgAndArgs ...any) {
	if !(actual > min) {
		fail(fmt.Sprintf("expected '%v' to be greater than '%v'", actual, min), msgAndArgs...)
	}
}

// GreaterOrEqual asserts that actual is greater than or equal to min
func GreaterOrEqual[T Ordered](actual, min T, msgAndArgs ...any) {
	if !(actual >= min) {
		fail(
			fmt.Sprintf("expected '%v' to be greater than or equal to '%v'", actual, min),
			msgAndArgs...,
		)
	}
}

// Len asserts that the given object's length matches the expected length
func Len(object any, expected int, msgAndArgs ...any) {
	v := reflect.ValueOf(object)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		actual := v.Len()
		if actual != expected {
			fail(
				fmt.Sprintf("expected length '%d' but got '%d'", expected, actual),
				msgAndArgs...,
			)
		}
	default:
		fail(
			fmt.Sprintf("invalid type %T (must be array, slice, map, string, or channel)", object),
			msgAndArgs...,
		)
	}
}

// Less asserts that actual is less than max
func Less[T Ordered](actual, max T, msgAndArgs ...any) {
	if !(actual < max) {
		fail(fmt.Sprintf("expected '%v' to be less than '%v'", actual, max), msgAndArgs...)
	}
}

// LessOrEqual asserts that actual is less than or equal to max
func LessOrEqual[T Ordered](actual, max T, msgAndArgs ...any) {
	if !(actual <= max) {
		fail(
			fmt.Sprintf("expected '%v' to be less than or equal to '%v'", actual, max),
			msgAndArgs...,
		)
	}
}

// Nil asserts that the given value is nil
func Nil(v any, msgAndArgs ...any) {
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
	fail("value is not nil", msgAndArgs...)
}

// NoError asserts that the given error is nil
func NoError(err error, msgAndArgs ...any) {
	if err != nil {
		fail(fmt.Sprintf("unexpected error: '%v'", err), msgAndArgs...)
	}
}

// NotEmpty asserts that the given string is not empty
func NotEmpty(s string, msgAndArgs ...any) {
	if s == "" {
		fail("string is empty", msgAndArgs...)
	}
}

// NotEqual asserts that the expected and actual values are not equal
func NotEqual[T any](expected, actual T, msgAndArgs ...any) {
	if reflect.DeepEqual(expected, actual) {
		fail(fmt.Sprintf("expected values to differ, but both were '%v'", actual), msgAndArgs...)
	}
}

// NotNil asserts that the given value is not nil
func NotNil(v any, msgAndArgs ...any) {
	if v == nil {
		fail("value is nil", msgAndArgs...)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			fail("value is nil", msgAndArgs...)
		}
	}
}

// True asserts that the given condition is true
func True(condition bool, msgAndArgs ...any) {
	if !condition {
		fail("expected true, got false", msgAndArgs...)
	}
}

// Type asserts that the types of a and b are the same
func Type(a, b any, msgAndArgs ...any) {
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)
	if ta != tb {
		fail(fmt.Sprintf("expected type %v but got %v", tb, ta), msgAndArgs...)
	}
}
