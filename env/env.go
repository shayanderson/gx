package env

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Bool retrieves the boolean value of the environment variable by key
func Bool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// Duration retrieves the duration value of the environment variable by key
func Duration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// Float64 retrieves the float64 value of the environment variable by key
func Float64(key string, fallback float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

// Int retrieves the integer value of the environment variable by key
func Int(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

// MustBool retrieves the boolean value of the environment variable by key
// panics if the variable is missing or empty
func MustBool(key string) bool {
	b, err := strconv.ParseBool(must(key))
	if err != nil {
		panic("invalid bool value for env var: " + key)
	}
	return b
}

// MustDuration retrieves the duration value of the environment variable by key
// panics if the variable is missing, empty, or invalid
func MustDuration(key string) time.Duration {
	d, err := time.ParseDuration(must(key))
	if err != nil {
		panic("invalid duration value for env var: " + key)
	}
	return d
}

// MustFloat64 retrieves the float64 value of the environment variable by key
// panics if the variable is missing, empty, or invalid
func MustFloat64(key string) float64 {
	f, err := strconv.ParseFloat(must(key), 64)
	if err != nil {
		panic("invalid float64 value for env var: " + key)
	}
	return f
}

// MustInt retrieves the integer value of the environment variable by key
// panics if the variable is missing, empty, or invalid integer
func MustInt(key string) int {
	i, err := strconv.Atoi(must(key))
	if err != nil {
		panic("invalid int value for env var: " + key)
	}
	return i
}

// MustString retrieves the string value of the environment variable by key
// panics if the variable is missing or empty
func MustString(key string) string {
	return must(key)
}

// MustStrings retrieves a slice of strings from the environment variable by key, split by comma
// panics if the variable is missing or empty
func MustStrings(key string) []string {
	return splitAndTrim(must(key), ",")
}

// String retrieves the string value of the environment variable by key
func String(key, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return v
}

// Strings retrieves a slice of strings from the environment variable by key, split by comma
func Strings(key string, fallback []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return splitAndTrim(v, ",")
}

// must retrieves the required environment variable by key
// panics if the variable is missing or empty
func must(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic("required env var missing or empty: " + key)
	}
	return v
}

// splitAndTrim splits a string by the given separator and trims whitespace from each element
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for v := range strings.SplitSeq(s, sep) {
		v = strings.TrimSpace(v)
		if v != "" {
			parts = append(parts, v)
		}
	}
	return parts
}
