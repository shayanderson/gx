package web

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shayanderson/gx/test"
)

func TestNewContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	c := NewContext(rr, req)

	test.Same(t, req, c.Request)
	test.True(t, req.Context() == c.Context())
	test.NotNil(t, c.Writer())
	test.Equal(t, DefaultMaxReadSize, c.maxReadSize)
}

func TestContextSetAndGet(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(httptest.NewRecorder(), req)

	c.Set("key", "value")

	test.Equal(t, "value", c.Get("key"))
	test.Equal(t, "value", c.Request.Context().Value("key"))
	test.NotEqual(t, context.Background(), c.Context())
}

func TestContextLogErrorResponseWriteWithoutLogger(t *testing.T) {
	t.Parallel()

	c := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	c.logErrorResponseWrite(errors.New("write failed"))
}

func TestContextBind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"shay"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	c := NewContext(httptest.NewRecorder(), req)
	var got struct {
		Name string `json:"name"`
	}

	err := c.Bind(&got)

	test.NoError(t, err)
	test.Equal(t, "shay", got.Name)
}

func TestContextBindJSONMediaTypeWithSpaceBeforeParameters(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"shay"}`))
	req.Header.Set("Content-Type", "application/json ; charset=utf-8")
	c := NewContext(httptest.NewRecorder(), req)
	var got struct {
		Name string `json:"name"`
	}

	err := c.Bind(&got)

	test.NoError(t, err)
	test.Equal(t, "shay", got.Name)
}

func TestContextBindInvalidContentType(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"shay"}`))
	req.Header.Set("Content-Type", "text/plain")
	c := NewContext(httptest.NewRecorder(), req)

	err := c.Bind(&struct{}{})

	test.NotNil(t, err)
	statusErr, ok := errors.AsType[StatusError](err)
	test.True(t, ok)
	test.Equal(t, http.StatusBadRequest, statusErr.Status())
	test.Equal(t, "invalid content type, expected application/json", statusErr.Error())
}

func TestContextBindInvalidJSONMediaType(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"shay"}`))
	req.Header.Set("Content-Type", "application/jsonfoo")
	c := NewContext(httptest.NewRecorder(), req)

	err := c.Bind(&struct{}{})

	statusErr, ok := errors.AsType[StatusError](err)
	test.True(t, ok)
	test.Equal(t, http.StatusBadRequest, statusErr.Status())
	test.Equal(t, "invalid content type, expected application/json", statusErr.Error())
}

func TestContextBindInvalidJSON(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	c := NewContext(httptest.NewRecorder(), req)

	err := c.Bind(&struct{}{})

	test.NotNil(t, err)
	statusErr, ok := errors.AsType[StatusError](err)
	test.True(t, ok)
	test.Equal(t, http.StatusBadRequest, statusErr.Status())
	test.Equal(t, "invalid json body", statusErr.Error())
}

func TestContextBindLimitReadSize(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"shay"}`))
	req.Header.Set("Content-Type", "application/json")
	c := newContext(httptest.NewRecorder(), req, contextOptions{maxReadSize: 7})

	err := c.Bind(&struct{}{})

	test.NotNil(t, err)
	statusErr, ok := errors.AsType[StatusError](err)
	test.True(t, ok)
	test.Equal(t, http.StatusRequestEntityTooLarge, statusErr.Status())
	_, ok = errors.AsType[*http.MaxBytesError](err)
	test.True(t, ok)
}

func TestContextBindNoLimitReadSize(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"shay"}`))
	req.Header.Set("Content-Type", "application/json")
	c := newContext(httptest.NewRecorder(), req, contextOptions{})
	var got struct {
		Name string `json:"name"`
	}

	err := c.Bind(&got)

	test.NoError(t, err)
	test.Equal(t, "shay", got.Name)
}

func TestContextString(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	err := c.String("hello", http.StatusCreated)

	test.NoError(t, err)
	test.Equal(t, http.StatusCreated, rr.Code)
	test.Equal(t, "text/plain; charset=utf-8", rr.Result().Header.Get("Content-Type"))
	test.Equal(t, "hello", rr.Body.String())
}

func TestContextHTML(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	err := c.HTML("<p>hello</p>", http.StatusAccepted)

	test.NoError(t, err)
	test.Equal(t, http.StatusAccepted, rr.Code)
	test.Equal(t, "text/html; charset=utf-8", rr.Result().Header.Get("Content-Type"))
	test.Equal(t, "<p>hello</p>", rr.Body.String())
}

func TestContextJSON(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	err := c.JSON(map[string]string{"name": "shay"}, http.StatusCreated)

	test.NoError(t, err)
	test.Equal(t, http.StatusCreated, rr.Code)
	test.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	test.Equal(t, `{"name":"shay"}`, rr.Body.String())
}

func TestContextJSONDoesNotCommitResponseOnMarshalError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	err := c.JSON(make(chan int), http.StatusCreated)

	test.NotNil(t, err)
	test.False(t, c.responseWritten())
	test.Equal(t, "", rr.Header().Get("Content-Type"))
	test.Equal(t, "", rr.Body.String())
}

func TestContextJSONPretty(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/?pretty", nil))

	err := c.JSON(map[string]string{"name": "shay"})

	test.NoError(t, err)
	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "{\n  \"name\": \"shay\"\n}", rr.Body.String())
}

func TestContextRedirect(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(rr, req)

	c.Redirect("/next")

	test.Equal(t, http.StatusSeeOther, rr.Code)
	test.Equal(t, "/next", rr.Header().Get("Location"))
}

func TestContextRedirectWithStatus(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := NewContext(rr, req)

	c.Redirect("/next", http.StatusTemporaryRedirect)

	test.Equal(t, http.StatusTemporaryRedirect, rr.Code)
	test.Equal(t, "/next", rr.Header().Get("Location"))
}

func TestContextStatus(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Status(http.StatusNoContent)
	c.Status(http.StatusInternalServerError)

	test.Equal(t, http.StatusNoContent, rr.Code)
}

func TestContextWriterDefaultsToOKOnWrite(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	n, err := c.Writer().Write([]byte("hello"))

	test.NoError(t, err)
	test.Equal(t, 5, n)
	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "hello", rr.Body.String())
}

func TestContextWriterWriteAfterStatus(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Status(http.StatusAccepted)
	n, err := c.Writer().Write([]byte("hello"))

	test.NoError(t, err)
	test.Equal(t, 5, n)
	test.Equal(t, http.StatusAccepted, rr.Code)
	test.Equal(t, "hello", rr.Body.String())
}

func TestContextWriterFlush(t *testing.T) {
	t.Parallel()

	w := &interfaceWriter{}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Writer().(http.Flusher).Flush()

	test.True(t, w.flushed)
	test.True(t, c.responseWritten())
}

func TestContextWriterFlushUnsupported(t *testing.T) {
	t.Parallel()

	w := &basicWriter{header: make(http.Header)}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Writer().(http.Flusher).Flush()

	test.False(t, w.wrote)
	test.False(t, c.responseWritten())
}

func TestContextWriterAllowsInformationalResponses(t *testing.T) {
	t.Parallel()

	w := &basicWriter{header: make(http.Header)}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Status(http.StatusEarlyHints)
	c.Status(http.StatusContinue)
	err := c.String("ok")

	test.NoError(t, err)
	test.Equal(t, []int{http.StatusEarlyHints, http.StatusContinue, http.StatusOK}, w.codes)
	test.True(t, c.responseWritten())
}

func TestContextWriterIgnoresInformationalResponseAfterFinalResponse(t *testing.T) {
	t.Parallel()

	w := &basicWriter{header: make(http.Header)}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Status(http.StatusCreated)
	c.Status(http.StatusEarlyHints)

	test.Equal(t, []int{http.StatusCreated}, w.codes)
	test.True(t, c.responseWritten())
}

func TestContextWriterTreatsSwitchingProtocolsAsFinalResponse(t *testing.T) {
	t.Parallel()

	w := &basicWriter{header: make(http.Header)}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	c.Status(http.StatusSwitchingProtocols)
	c.Status(http.StatusOK)
	c.Status(http.StatusEarlyHints)

	test.Equal(t, []int{http.StatusSwitchingProtocols}, w.codes)
	test.True(t, c.responseWritten())
}

func TestContextWriterUnwrap(t *testing.T) {
	t.Parallel()

	w := &basicWriter{header: make(http.Header)}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	unwrap := c.Writer().(interface{ Unwrap() http.ResponseWriter })

	test.Same(t, w, unwrap.Unwrap())
}

func TestContextWriterHijack(t *testing.T) {
	t.Parallel()

	w := &interfaceWriter{}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	conn, rw, err := c.Writer().(http.Hijacker).Hijack()
	defer func() {
		err := conn.Close()
		test.NoError(t, err)
	}()

	test.NoError(t, err)
	test.NotNil(t, conn)
	test.NotNil(t, rw)
	test.True(t, w.hijacked)
}

func TestContextWriterHijackUnsupported(t *testing.T) {
	t.Parallel()

	c := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	conn, rw, err := c.Writer().(http.Hijacker).Hijack()

	test.Nil(t, conn)
	test.Nil(t, rw)
	test.NotNil(t, err)
}

func TestContextWriterPush(t *testing.T) {
	t.Parallel()

	w := &interfaceWriter{}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))

	err := c.Writer().(http.Pusher).Push("/asset.css", nil)

	test.NoError(t, err)
	test.Equal(t, "/asset.css", w.pushedTarget)
}

func TestContextWriterPushUnsupported(t *testing.T) {
	t.Parallel()

	c := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	err := c.Writer().(http.Pusher).Push("/asset.css", nil)

	test.Error(t, err, http.ErrNotSupported)
}

type basicWriter struct {
	header http.Header
	code   int
	codes  []int
	wrote  bool
}

func (w *basicWriter) Header() http.Header {
	return w.header
}

func (w *basicWriter) Write(b []byte) (int, error) {
	w.wrote = true
	return len(b), nil
}

func (w *basicWriter) WriteHeader(statusCode int) {
	w.code = statusCode
	w.codes = append(w.codes, statusCode)
}

type interfaceWriter struct {
	basicWriter
	flushed      bool
	hijacked     bool
	pushedTarget string
}

func (w *interfaceWriter) Flush() {
	w.flushed = true
}

func (w *interfaceWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	clientConn, serverConn := net.Pipe()
	clientConn.Close()
	return serverConn, bufio.NewReadWriter(
		bufio.NewReader(serverConn),
		bufio.NewWriter(serverConn),
	), nil
}

func (w *interfaceWriter) Push(target string, opts *http.PushOptions) error {
	w.pushedTarget = target
	return nil
}
