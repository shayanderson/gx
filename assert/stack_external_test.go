package assert_test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shayanderson/gx/assert"
)

func failureMessage(t *testing.T) (message, file string, line int) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected assertion panic, but did not panic")
		}

		var ok bool
		message, ok = r.(string)
		if !ok {
			t.Fatalf("expected string panic, but got %T", r)
		}
	}()

	_, file, line, _ = runtime.Caller(0)
	line += 2 // The assertion call is two source lines below runtime.Caller.
	assert.True(false)
	return "", file, line
}

func TestFailureStackIncludesConsumerCallSite(t *testing.T) {
	message, file, line := failureMessage(t)
	wantCallSite := fmt.Sprintf("at %s:%d\n", file, line)
	if !strings.Contains(message, wantCallSite) {
		t.Fatalf("panic stack trace missing consumer call site %q:\n%s", wantCallSite, message)
	}

	internalFile := filepath.Join(filepath.Dir(file), "assert.go")
	if strings.Contains(message, "at "+internalFile+":") {
		t.Fatalf("panic stack trace contains assertion-internal frame:\n%s", message)
	}
}
