package assert

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// fail constructs a detailed error message including a stack trace and panics.
func fail(msg string, message ...string) {
	var b strings.Builder

	b.WriteString("assertion failed: ")
	b.WriteString(msg)
	if len(message) > 1 {
		panic("assertion accepts at most one message")
	}
	if len(message) == 1 {
		b.WriteString(": ")
		b.WriteString(message[0])
	}
	b.WriteString("\n\nstack trace:\n")

	// Skip first n frames until out of assert package.
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

// Empty asserts that the given string is empty.
func Empty(s string, message ...string) {
	if s != "" {
		fail("string is not empty", message...)
	}
}

// formatEqualFailure formats the failure message for an equality assertion.
func formatEqualFailure(expected, actual any) string {
	expectedText := fmt.Sprintf("%v", expected)
	actualText := fmt.Sprintf("%v", actual)
	if expectedText == actualText {
		expectedText = fmt.Sprintf("%#v", expected)
		actualText = fmt.Sprintf("%#v", actual)
	}

	if reflect.TypeOf(expected) == reflect.TypeOf(actual) {
		return fmt.Sprintf("expected: '%s', got: '%s'", expectedText, actualText)
	}
	return fmt.Sprintf(
		"expected: '%s' (%T), got: '%s' (%T)",
		expectedText, expected, actualText, actual,
	)
}

// Equal asserts that expected and actual are deeply equal. Its argument order is
// (expected, actual).
func Equal[T any](expected, actual T, message ...string) {
	if !reflect.DeepEqual(expected, actual) {
		fail(formatEqualFailure(expected, actual), message...)
	}
}

// Error asserts that actual matches expected. Its argument order is (expected,
// actual). It succeeds when both errors are nil or errors.Is(actual, expected)
// succeeds.
func Error(expected error, actual error, message ...string) {
	if actual == nil && expected == nil {
		return
	}
	if actual == nil || expected == nil {
		fail(
			fmt.Sprintf("one error is nil: expected '%v', got '%v'", expected, actual),
			message...,
		)
		return
	}
	if !errors.Is(actual, expected) {
		fail(
			fmt.Sprintf("expected error '%v', got '%v'", expected, actual),
			message...,
		)
	}
}

// False asserts that the given condition is false.
func False(condition bool, message ...string) {
	if condition {
		fail("expected false, got true", message...)
	}
}

// Greater asserts that actual is greater than minimum.
func Greater[T cmp.Ordered](actual, minimum T, message ...string) {
	if !(actual > minimum) {
		fail(fmt.Sprintf("expected '%v' to be greater than '%v'", actual, minimum), message...)
	}
}

// GreaterOrEqual asserts that actual is greater than or equal to minimum.
func GreaterOrEqual[T cmp.Ordered](actual, minimum T, message ...string) {
	if !(actual >= minimum) {
		fail(
			fmt.Sprintf("expected '%v' to be greater than or equal to '%v'", actual, minimum),
			message...,
		)
	}
}

// Len asserts that expected matches object's length. Its argument order is
// (expected, object). Object must be an array, pointer to array, slice, map,
// string, or channel.
func Len(expected int, object any, message ...string) {
	v := reflect.ValueOf(object)
	valid := false
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		valid = true
	case reflect.Pointer:
		valid = v.Type().Elem().Kind() == reflect.Array
	}
	if !valid {
		fail(
			fmt.Sprintf(
				"invalid type %T (must be array, pointer to array, slice, map, string, or channel)",
				object,
			),
			message...,
		)
		return
	}

	actual := v.Len()
	if actual != expected {
		fail(
			fmt.Sprintf("expected length '%d' but got '%d'", expected, actual),
			message...,
		)
	}
}

// Less asserts that actual is less than maximum.
func Less[T cmp.Ordered](actual, maximum T, message ...string) {
	if !(actual < maximum) {
		fail(fmt.Sprintf("expected '%v' to be less than '%v'", actual, maximum), message...)
	}
}

// LessOrEqual asserts that actual is less than or equal to maximum.
func LessOrEqual[T cmp.Ordered](actual, maximum T, message ...string) {
	if !(actual <= maximum) {
		fail(
			fmt.Sprintf("expected '%v' to be less than or equal to '%v'", actual, maximum),
			message...,
		)
	}
}

// Nil asserts that the given value is nil.
func Nil(v any, message ...string) {
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			return
		}
	case reflect.UnsafePointer:
		if rv.Pointer() == 0 {
			return
		}
	}
	fail("value is not nil", message...)
}

// NoError asserts that the given error is nil.
func NoError(err error, message ...string) {
	if err != nil {
		fail(fmt.Sprintf("unexpected error: '%v'", err), message...)
	}
}

// NotEmpty asserts that the given string is not empty.
func NotEmpty(s string, message ...string) {
	if s == "" {
		fail("string is empty", message...)
	}
}

// NotEqual asserts that expected and actual are not deeply equal. Its argument
// order is (expected, actual).
func NotEqual[T any](expected, actual T, message ...string) {
	if reflect.DeepEqual(expected, actual) {
		fail(fmt.Sprintf("expected values to differ, but both were '%v'", actual), message...)
	}
}

// NotNil asserts that the given value is not nil.
func NotNil(v any, message ...string) {
	if v == nil {
		fail("value is nil", message...)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			fail("value is nil", message...)
		}
	case reflect.UnsafePointer:
		if rv.Pointer() == 0 {
			fail("value is nil", message...)
		}
	}
}

// True asserts that the given condition is true.
func True(condition bool, message ...string) {
	if !condition {
		fail("expected true, got false", message...)
	}
}

// Type asserts that expected and actual have the same dynamic type. Its
// argument order is (expected, actual).
func Type(expected, actual any, message ...string) {
	te := reflect.TypeOf(expected)
	ta := reflect.TypeOf(actual)
	if te != ta {
		fail(fmt.Sprintf("expected type %v but got %v", te, ta), message...)
	}
}
