package web_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/shayanderson/gx/test"
	"github.com/shayanderson/gx/web"
)

func TestError(t *testing.T) {
	t.Parallel()

	err := web.Error(http.StatusBadRequest, "bad request")

	test.Equal(t, "bad request", err.Error())
	test.Equal(t, http.StatusBadRequest, err.Status())
}

func TestErrorf(t *testing.T) {
	t.Parallel()

	err := web.Errorf(http.StatusNotFound, "missing %s", "user")

	test.Equal(t, "missing user", err.Error())
	test.Equal(t, http.StatusNotFound, err.Status())
}

func TestErrorWrap(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("failed")
	err := web.ErrorWrap(http.StatusInternalServerError, baseErr)

	test.Equal(t, "failed", err.Error())
	test.Equal(t, http.StatusInternalServerError, err.Status())
	test.True(t, errors.Is(err, baseErr))
}

func TestErrorWrapNil(t *testing.T) {
	t.Parallel()

	err := web.ErrorWrap(http.StatusInternalServerError, nil)

	test.Nil(t, err)
}
