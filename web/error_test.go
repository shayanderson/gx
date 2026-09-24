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

func TestErrorStatus(t *testing.T) {
	t.Parallel()

	err := web.ErrorStatus(http.StatusServiceUnavailable)

	test.Equal(t, "service unavailable", err.Error())
	test.Equal(t, http.StatusServiceUnavailable, err.Status())
}

func TestErrorStatusDefaultsToInternalServerError(t *testing.T) {
	t.Parallel()

	err := web.ErrorStatus()

	test.Equal(t, "internal server error", err.Error())
	test.Equal(t, http.StatusInternalServerError, err.Status())
}

func TestErrorStatusUsesInternalServerErrorForUnknownStatus(t *testing.T) {
	t.Parallel()

	err := web.ErrorStatus(599)

	test.Equal(t, "internal server error", err.Error())
	test.Equal(t, 599, err.Status())
}
