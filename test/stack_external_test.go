package test_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/shayanderson/gx/test"
)

type captureT struct {
	message string
}

func (t *captureT) Fatal(args ...any) {
	t.message = fmt.Sprint(args...)
}

func (*captureT) Helper() {}

func TestFailureStackIncludesExternalCallSite(t *testing.T) {
	capture := &captureT{}
	_, file, line, _ := runtime.Caller(0)
	test.True(capture, false) // The expected trace frame is this line.

	want := fmt.Sprintf(
		"%s:%d - %s\n",
		file,
		line+1,
		runtime.FuncForPC(reflect.ValueOf(TestFailureStackIncludesExternalCallSite).Pointer()).
			Name(),
	)
	if !strings.Contains(capture.message, want) {
		t.Fatalf("failure trace missing external call site %q:\n%s", want, capture.message)
	}

	internalFile := filepath.Join(filepath.Dir(file), "assert.go")
	if strings.Contains(capture.message, internalFile+":") {
		t.Fatalf("failure trace contains assertion-internal frame:\n%s", capture.message)
	}
}
