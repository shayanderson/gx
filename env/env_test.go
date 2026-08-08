package env

import (
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestBool(t *testing.T) {
	t.Setenv("TEST_BOOL_TRUE", "true")
	t.Setenv("TEST_BOOL_FALSE", "0")
	t.Setenv("TEST_BOOL_INVALID", "yes")

	test.True(t, Bool("TEST_BOOL_TRUE", false))
	test.False(t, Bool("TEST_BOOL_FALSE", true))
	test.True(t, Bool("TEST_BOOL_MISSING", true))
	test.True(t, Bool("TEST_BOOL_INVALID", true))
}

func TestBoolParseBoolValues(t *testing.T) {
	cases := []struct {
		name string
		v    string
		want bool
	}{
		{name: "one", v: "1", want: true},
		{name: "t", v: "t", want: true},
		{name: "T", v: "T", want: true},
		{name: "TRUE", v: "TRUE", want: true},
		{name: "true", v: "true", want: true},
		{name: "True", v: "True", want: true},
		{name: "zero", v: "0", want: false},
		{name: "f", v: "f", want: false},
		{name: "F", v: "F", want: false},
		{name: "FALSE", v: "FALSE", want: false},
		{name: "false", v: "false", want: false},
		{name: "False", v: "False", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_BOOL_PARSE", tc.v)
			test.Equal(t, tc.want, Bool("TEST_BOOL_PARSE", !tc.want))
		})
	}
}

func TestDuration(t *testing.T) {
	t.Setenv("TEST_DURATION_VALID", "1h30m")
	t.Setenv("TEST_DURATION_INVALID", "abc")

	test.Equal(t, 90*time.Minute, Duration("TEST_DURATION_VALID", time.Second))
	test.Equal(t, time.Second, Duration("TEST_DURATION_INVALID", time.Second))
	test.Equal(t, time.Minute, Duration("TEST_DURATION_MISSING", time.Minute))
}

func TestFloat64(t *testing.T) {
	t.Setenv("TEST_FLOAT64_VALID", "3.14")
	t.Setenv("TEST_FLOAT64_INVALID", "abc")

	test.Equal(t, 3.14, Float64("TEST_FLOAT64_VALID", 1.5))
	test.Equal(t, 7.5, Float64("TEST_FLOAT64_INVALID", 7.5))
	test.Equal(t, 9.5, Float64("TEST_FLOAT64_MISSING", 9.5))
}

func TestInt(t *testing.T) {
	t.Setenv("TEST_INT_VALID", "42")
	t.Setenv("TEST_INT_INVALID", "abc")

	test.Equal(t, 42, Int("TEST_INT_VALID", 1))
	test.Equal(t, 7, Int("TEST_INT_INVALID", 7))
	test.Equal(t, 9, Int("TEST_INT_MISSING", 9))
}

func TestMustBool(t *testing.T) {
	t.Setenv("TEST_MUST_BOOL_TRUE", "1")
	test.True(t, MustBool("TEST_MUST_BOOL_TRUE"))
}

func TestMustBoolParseBoolValues(t *testing.T) {
	cases := []struct {
		name string
		v    string
		want bool
	}{
		{name: "one", v: "1", want: true},
		{name: "t", v: "t", want: true},
		{name: "T", v: "T", want: true},
		{name: "TRUE", v: "TRUE", want: true},
		{name: "true", v: "true", want: true},
		{name: "True", v: "True", want: true},
		{name: "zero", v: "0", want: false},
		{name: "f", v: "f", want: false},
		{name: "F", v: "F", want: false},
		{name: "FALSE", v: "FALSE", want: false},
		{name: "false", v: "false", want: false},
		{name: "False", v: "False", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_MUST_BOOL_PARSE", tc.v)
			test.Equal(t, tc.want, MustBool("TEST_MUST_BOOL_PARSE"))
		})
	}
}

func TestMustBoolPanic(t *testing.T) {
	test.Panics(t, func() { MustBool("TEST_MUST_BOOL_MISSING") })
}

func TestMustBoolEmptyPanic(t *testing.T) {
	t.Setenv("TEST_MUST_BOOL_EMPTY", "")
	test.Panics(t, func() { MustBool("TEST_MUST_BOOL_EMPTY") })
}

func TestMustBoolInvalidPanic(t *testing.T) {
	t.Setenv("TEST_MUST_BOOL_INVALID", "yes")
	test.Panics(t, func() { MustBool("TEST_MUST_BOOL_INVALID") })
}

func TestMustDuration(t *testing.T) {
	t.Setenv("TEST_MUST_DURATION_VALID", "250ms")
	test.Equal(t, 250*time.Millisecond, MustDuration("TEST_MUST_DURATION_VALID"))
}

func TestMustDurationPanic(t *testing.T) {
	test.Panics(t, func() { MustDuration("TEST_MUST_DURATION_MISSING") })
}

func TestMustDurationEmptyPanic(t *testing.T) {
	t.Setenv("TEST_MUST_DURATION_EMPTY", "")
	test.Panics(t, func() { MustDuration("TEST_MUST_DURATION_EMPTY") })
}

func TestMustDurationInvalidPanic(t *testing.T) {
	t.Setenv("TEST_MUST_DURATION_INVALID", "abc")
	test.Panics(t, func() { MustDuration("TEST_MUST_DURATION_INVALID") })
}

func TestMustFloat64(t *testing.T) {
	t.Setenv("TEST_MUST_FLOAT64_VALID", "123.45")
	test.Equal(t, 123.45, MustFloat64("TEST_MUST_FLOAT64_VALID"))
}

func TestMustFloat64Panic(t *testing.T) {
	test.Panics(t, func() { MustFloat64("TEST_MUST_FLOAT64_MISSING") })
}

func TestMustFloat64EmptyPanic(t *testing.T) {
	t.Setenv("TEST_MUST_FLOAT64_EMPTY", "")
	test.Panics(t, func() { MustFloat64("TEST_MUST_FLOAT64_EMPTY") })
}

func TestMustFloat64InvalidPanic(t *testing.T) {
	t.Setenv("TEST_MUST_FLOAT64_INVALID", "abc")
	test.Panics(t, func() { MustFloat64("TEST_MUST_FLOAT64_INVALID") })
}

func TestMustInt(t *testing.T) {
	t.Setenv("TEST_MUST_INT_VALID", "123")
	test.Equal(t, 123, MustInt("TEST_MUST_INT_VALID"))
}

func TestMustIntPanic(t *testing.T) {
	test.Panics(t, func() { MustInt("TEST_MUST_INT_MISSING") })
}

func TestMustIntEmptyPanic(t *testing.T) {
	t.Setenv("TEST_MUST_INT_EMPTY", "")
	test.Panics(t, func() { MustInt("TEST_MUST_INT_EMPTY") })
}

func TestMustIntInvalidPanic(t *testing.T) {
	t.Setenv("TEST_MUST_INT_INVALID", "x")
	test.Panics(t, func() { MustInt("TEST_MUST_INT_INVALID") })
}

func TestMustString(t *testing.T) {
	t.Setenv("TEST_MUST_STRING", "value")
	test.Equal(t, "value", MustString("TEST_MUST_STRING"))
}

func TestMustStringPanic(t *testing.T) {
	test.Panics(t, func() { MustString("TEST_MUST_STRING_MISSING") })
}

func TestMustStringEmptyPanic(t *testing.T) {
	t.Setenv("TEST_MUST_STRING_EMPTY", "")
	test.Panics(t, func() { MustString("TEST_MUST_STRING_EMPTY") })
}

func TestMustStrings(t *testing.T) {
	t.Setenv("TEST_MUST_STRINGS", " a, b , ,c ")
	got := MustStrings("TEST_MUST_STRINGS")
	want := []string{"a", "b", "c"}
	test.Equal(t, want, got)
}

func TestMustStringsPanic(t *testing.T) {
	test.Panics(t, func() { MustStrings("TEST_MUST_STRINGS_MISSING") })
}

func TestMustStringsEmptyPanic(t *testing.T) {
	t.Setenv("TEST_MUST_STRINGS_EMPTY", "")
	test.Panics(t, func() { MustStrings("TEST_MUST_STRINGS_EMPTY") })
}

func TestString(t *testing.T) {
	t.Setenv("TEST_STRING", "hello")
	test.Equal(t, "hello", String("TEST_STRING", "fallback"))
	test.Equal(t, "fallback", String("TEST_STRING_MISSING", "fallback"))
}

func TestStrings(t *testing.T) {
	t.Setenv("TEST_STRINGS", " one, two ,, three ")
	got := Strings("TEST_STRINGS", []string{"fallback"})
	want := []string{"one", "two", "three"}
	test.Equal(t, want, got)

	fallback := []string{"fallback", "values"}
	got = Strings("TEST_STRINGS_MISSING", fallback)
	test.Equal(t, fallback, got)
}

func TestSplitAndTrim(t *testing.T) {
	got := splitAndTrim(" a, b ,, c , ", ",")
	want := []string{"a", "b", "c"}
	test.Equal(t, want, got)

	got = splitAndTrim("", ",")
	test.Equal(t, []string{}, got)
}
