package web

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestNewServer(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{Addr: ":0"})

	test.Equal(t, ":0", s.opts.Addr)
	test.Equal(t, 3*time.Second, s.opts.ReadHeaderTimeout)
	test.Equal(t, 5*time.Second, s.opts.ReadTimeout)
	test.Equal(t, DefaultShutdownTimeout, s.opts.ShutdownTimeout)
	test.Equal(t, 5*time.Second, s.opts.WriteTimeout)
	test.Equal(t, DefaultMaxReadSize, s.contextOpts.maxReadSize)
	test.NotNil(t, s.mux)
	test.NotNil(t, s.server)
}

func TestNewServerPreservesOptions(t *testing.T) {
	t.Parallel()
	const maxReadSize int64 = 1024

	s := NewServer(Options{
		Addr:                ":1234",
		CertFile:            "cert.pem",
		CertKeyFile:         "key.pem",
		IdleTimeout:         time.Second,
		MaxHeaderBytes:      32 * 1024,
		MaxHeaderValueCount: 64,
		ReadHeaderTimeout:   2 * time.Second,
		ReadTimeout:         3 * time.Second,
		ShutdownTimeout:     5 * time.Second,
		WriteTimeout:        4 * time.Second,
		MaxReadSize:         maxReadSize,
	})

	test.Equal(t, ":1234", s.opts.Addr)
	test.Equal(t, "cert.pem", s.opts.CertFile)
	test.Equal(t, "key.pem", s.opts.CertKeyFile)
	test.Equal(t, time.Second, s.opts.IdleTimeout)
	test.Equal(t, 32*1024, s.opts.MaxHeaderBytes)
	test.Equal(t, 64, s.opts.MaxHeaderValueCount)
	test.Equal(t, 2*time.Second, s.opts.ReadHeaderTimeout)
	test.Equal(t, 3*time.Second, s.opts.ReadTimeout)
	test.Equal(t, 5*time.Second, s.opts.ShutdownTimeout)
	test.Equal(t, 4*time.Second, s.opts.WriteTimeout)
	test.Equal(t, 32*1024, s.server.MaxHeaderBytes)
	test.Equal(t, 64, s.server.MaxHeaderValueCount)
	test.Equal(t, maxReadSize, s.contextOpts.maxReadSize)
}

func TestNewServerDisablesMaxReadSizeWithNegativeValue(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{MaxReadSize: -1})

	test.Equal(t, int64(0), s.contextOpts.maxReadSize)
}

func TestServerMux(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{Addr: ":0"})

	test.Same(t, s.mux, s.Mux())
}

func TestServerHandle(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{Addr: ":0"})
	s.Handle("GET /hello", func(c *Context) error {
		return c.String("hello")
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)

	s.Mux().ServeHTTP(rr, req)

	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "hello", rr.Body.String())
}

func TestServerHandler(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{})
	s.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.Writer().Header().Set("X-Middleware", "true")
			return next(c)
		}
	})
	s.Get("/hello", func(c *Context) error {
		return c.String("hello")
	})

	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/hello", nil))

	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "true", rr.Result().Header.Get("X-Middleware"))
	test.Equal(t, "hello", rr.Body.String())
}

func TestServerBuildsGlobalMiddlewareOnce(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{})
	builds := 0
	s.Use(func(next HandlerFunc) HandlerFunc {
		builds++
		return next
	})
	s.Get("/", func(c *Context) error {
		return c.String("ok")
	})

	for range 2 {
		s.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}

	test.Equal(t, 1, builds)
}

func TestServerServeHTTPLogsRequest(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	s := NewServer(Options{
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		LogPrefix: "public-api",
	})
	s.Get("/hello", func(c *Context) error {
		return c.String("hello")
	})

	rr := httptest.NewRecorder()
	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/hello", nil))

	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "hello", rr.Body.String())
	test.Equal(t, 1, strings.Count(logs.String(), "\n"))
	test.True(t, strings.Contains(logs.String(),
		`"msg":"public-api: GET http://example.com/hello HTTP/1.1 from 192.0.2.1:1234"`,
	))
}

func TestServerServeHTTPLogsTLSRequest(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	s := NewServer(Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	s.Get("/hello", func(c *Context) error {
		return c.String("hello")
	})
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.TLS = &tls.ConnectionState{}

	s.serveHTTP(httptest.NewRecorder(), req)

	test.True(t, strings.Contains(logs.String(),
		`"msg":"http: GET https://example.com/hello HTTP/1.1 from 192.0.2.1:1234"`,
	))
}

func TestServerServeHTTPLogsHandlerError(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	s := NewServer(Options{
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		LogPrefix: "public-api",
	})
	s.Get("/fail", func(*Context) error {
		return Error(http.StatusTeapot, "short and stout")
	})

	rr := httptest.NewRecorder()
	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/fail", nil))

	test.Equal(t, http.StatusTeapot, rr.Code)
	test.Equal(t, `{"error":"short and stout"}`, rr.Body.String())
	test.Equal(t, 2, strings.Count(logs.String(), "\n"))
	test.True(t, strings.Contains(logs.String(), `"level":"WARN"`))
	test.True(t, strings.Contains(logs.String(),
		`"msg":"public-api: GET http://example.com/fail HTTP/1.1 from 192.0.2.1:1234 (418)"`,
	))
	test.True(t, strings.Contains(logs.String(), `"err":"short and stout"`))
}

func TestServerServeHTTPLogsEffectiveErrorStatus(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	s := NewServer(Options{
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	})
	s.Get("/fail", func(*Context) error {
		return Error(http.StatusOK, "bad status")
	})

	rr := httptest.NewRecorder()
	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/fail", nil))

	test.Equal(t, http.StatusInternalServerError, rr.Code)
	test.True(t, strings.Contains(logs.String(), `"level":"ERROR"`))
	test.True(t, strings.Contains(logs.String(),
		`"msg":"http: GET http://example.com/fail HTTP/1.1 from 192.0.2.1:1234 (500)"`,
	))
	test.True(t, strings.Contains(logs.String(), `"err":"bad status"`))
	test.Equal(t, `{"error":"bad status"}`, rr.Body.String())
}

func TestServerServeHTTPErrorStatus(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	s := NewServer(Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	s.Get("/fail", func(*Context) error {
		return ErrorStatus()
	})

	rr := httptest.NewRecorder()
	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/fail", nil))

	test.Equal(t, http.StatusInternalServerError, rr.Code)
	test.Equal(t, `{"error":"internal server error"}`, rr.Body.String())
	test.True(t, strings.Contains(logs.String(), `"err":"internal server error"`))
}

func TestServerServeHTTPDoesNotUseDefaultLogger(t *testing.T) {
	var defaultLogs bytes.Buffer
	originalDefaultLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&defaultLogs, nil)))
	t.Cleanup(func() { slog.SetDefault(originalDefaultLogger) })

	s := NewServer(Options{})
	s.Get("/hello", func(c *Context) error {
		return c.String("hello")
	})

	rr := httptest.NewRecorder()
	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/hello", nil))

	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "hello", rr.Body.String())
	test.Equal(t, "", defaultLogs.String())
}

func TestServerMethodHelpers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		add    func(*Server, string, HandlerFunc, ...Middleware)
	}{
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   "/delete",
			body:   "deleted",
			add:    (*Server).Delete,
		},
		{name: "get", method: http.MethodGet, path: "/get", body: "got", add: (*Server).Get},
		{
			name:   "patch",
			method: http.MethodPatch,
			path:   "/patch",
			body:   "patched",
			add:    (*Server).Patch,
		},
		{name: "post", method: http.MethodPost, path: "/post", body: "posted", add: (*Server).Post},
		{name: "put", method: http.MethodPut, path: "/put", body: "put", add: (*Server).Put},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(Options{Addr: ":0"})
			tc.add(s, tc.path, func(c *Context) error {
				return c.String(tc.body)
			})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)

			s.Mux().ServeHTTP(rr, req)

			test.Equal(t, http.StatusOK, rr.Code)
			test.Equal(t, tc.body, rr.Body.String())
		})
	}
}

func TestServerUse(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{Addr: ":0"})
	mw := func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.Set("mw", true)
			return next(c)
		}
	}

	s.Use(mw)

	test.Equal(t, 1, len(s.middleware))
}

func TestServerGlobalMiddlewareResponsePreventsRouteErrorResponse(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{})
	s.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			if err := c.String("partial", http.StatusCreated); err != nil {
				return err
			}
			return next(c)
		}
	})
	s.Get("/", func(*Context) error {
		return Error(http.StatusInternalServerError, "failed")
	})
	rr := httptest.NewRecorder()

	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	test.Equal(t, http.StatusCreated, rr.Code)
	test.Equal(t, "partial", rr.Body.String())
}

func TestChain(t *testing.T) {
	t.Parallel()

	called := make([]string, 0)
	h := chain(func(c *Context) error {
		called = append(called, "handler")
		return nil
	}, func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			called = append(called, "one")
			return next(c)
		}
	}, func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			called = append(called, "two")
			return next(c)
		}
	})
	c := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	err := h(c)

	test.NoError(t, err)
	test.Equal(t, []string{"two", "one", "handler"}, called)
}

func TestHandlerFuncServe(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return c.JSON(map[string]string{"ok": "true"}, http.StatusAccepted)
	})

	h.Serve(c)

	test.Equal(t, http.StatusAccepted, rr.Code)
	test.Equal(t, `{"ok":"true"}`, rr.Body.String())
}

func TestHandlerFuncServeStatusError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return Error(http.StatusTeapot, "short and stout")
	})

	h.Serve(c)

	test.Equal(t, http.StatusTeapot, rr.Code)
	test.Equal(t, `{"error":"short and stout"}`, rr.Body.String())
}

func TestHandlerFuncServeWrappedStatusError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(*Context) error {
		return fmt.Errorf("load resource: %w", Error(http.StatusNotFound, "not found"))
	})

	h.Serve(c)

	test.Equal(t, http.StatusNotFound, rr.Code)
	test.Equal(t, `{"error":"not found"}`, rr.Body.String())
}

func TestHandlerFuncServeErrorDefaultsToInternalServerError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return errors.New("failed")
	})

	h.Serve(c)

	test.Equal(t, http.StatusInternalServerError, rr.Code)
	test.Equal(t, `{"error":"failed"}`, rr.Body.String())
}

func TestHandlerFuncServeInvalidStatusErrorDefaultsToInternalServerError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return Error(http.StatusOK, "bad status")
	})

	h.Serve(c)

	test.Equal(t, http.StatusInternalServerError, rr.Code)
	test.Equal(t, `{"error":"bad status"}`, rr.Body.String())
}

func TestHandlerFuncServeUnknownServerErrorUsesGenericMessage(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(*Context) error {
		return ErrorStatus(599)
	})

	h.Serve(c)

	test.Equal(t, 599, rr.Code)
	test.Equal(t, `{"error":"internal server error"}`, rr.Body.String())
}

func TestHandlerFuncServeErrorStatusUsesStatusMessage(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(*Context) error {
		return ErrorStatus(http.StatusServiceUnavailable)
	})

	h.Serve(c)

	test.Equal(t, http.StatusServiceUnavailable, rr.Code)
	test.Equal(t, `{"error":"service unavailable"}`, rr.Body.String())
}

func TestHandlerFuncServeWrappedServerError(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(*Context) error {
		return ErrorWrap(http.StatusInternalServerError, errors.New("failed"))
	})

	h.Serve(c)

	test.Equal(t, http.StatusInternalServerError, rr.Code)
	test.Equal(t, `{"error":"failed"}`, rr.Body.String())
}

func TestHandlerFuncServeDoesNotWriteErrorAfterResponse(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		if err := c.String("partial", http.StatusCreated); err != nil {
			return err
		}
		return Error(http.StatusInternalServerError, "failed")
	})

	h.Serve(c)

	test.Equal(t, http.StatusCreated, rr.Code)
	test.Equal(t, "partial", rr.Body.String())
}

func TestHandlerFuncServeDoesNotWriteErrorAfterFlush(t *testing.T) {
	t.Parallel()

	w := &interfaceWriter{header: make(http.Header)}
	c := NewContext(w, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		c.Writer().(http.Flusher).Flush()
		return Error(http.StatusInternalServerError, "failed")
	})

	h.Serve(c)

	test.True(t, w.flushed)
	test.False(t, w.wrote)
}

func TestHandlerFuncServeLogsErrorResponseWriteFailure(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	c := newContext(
		&failingWriter{header: make(http.Header)},
		httptest.NewRequest(http.MethodGet, "/", nil),
		contextOptions{
			logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
			logPrefix: "http",
		},
	)
	h := HandlerFunc(func(c *Context) error {
		return errors.New("failed")
	})

	h.Serve(c)

	test.True(t, strings.Contains(logs.String(),
		`"msg":"http: failed to write error response"`,
	))
	logLines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	test.Equal(t, 2, len(logLines))
	test.True(t, strings.Contains(logLines[1], `"err":`))
}

func TestServerErrorHandler(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{ErrorHandler: func(c *Context, err StatusError) {
		test.Equal(t, http.StatusBadRequest, err.Status())
		test.Equal(t, "bad", err.Error())
		_ = c.String("custom", http.StatusBadRequest)
	}})
	s.Get("/", func(c *Context) error {
		return Error(http.StatusBadRequest, "bad")
	})
	rr := httptest.NewRecorder()

	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	test.Equal(t, http.StatusBadRequest, rr.Code)
	test.Equal(t, "custom", rr.Body.String())
}

func TestServerErrorHandlerDoesNotRunAfterResponse(t *testing.T) {
	t.Parallel()

	called := false
	s := NewServer(Options{ErrorHandler: func(*Context, StatusError) {
		called = true
	}})
	s.Get("/", func(c *Context) error {
		if err := c.String("partial", http.StatusCreated); err != nil {
			return err
		}
		return Error(http.StatusInternalServerError, "failed")
	})
	rr := httptest.NewRecorder()

	s.serveHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	test.False(t, called)
	test.Equal(t, http.StatusCreated, rr.Code)
	test.Equal(t, "partial", rr.Body.String())
}

func TestServerBindErrorResponses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		body        string
		maxReadSize int64
		status      int
	}{
		{
			name:   "invalid JSON",
			body:   `{`,
			status: http.StatusBadRequest,
		},
		{
			name:        "body too large",
			body:        `{"name":"shay"}`,
			maxReadSize: 7,
			status:      http.StatusRequestEntityTooLarge,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(Options{MaxReadSize: tc.maxReadSize})
			s.Post("/", func(c *Context) error {
				return c.Bind(&struct{}{})
			})
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			s.Handler().ServeHTTP(rr, req)

			test.Equal(t, tc.status, rr.Code)
		})
	}
}

func TestServerBindRequiresJSONContentType(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{})
	s.Post("/", func(c *Context) error {
		return c.Bind(&struct{}{})
	})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	test.Equal(t, http.StatusBadRequest, rr.Code)
	test.Equal(t, `{"error":"invalid content type, expected application/json"}`, rr.Body.String())
}

func TestServerBindAnyContentType(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{})
	s.Post("/", func(c *Context) error {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.BindAnyContentType(&body); err != nil {
			return err
		}
		return c.String(body.Name)
	})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"shay"}`))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "shay", rr.Body.String())
}

func TestHandlerFuncServeHTTP(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h := HandlerFunc(func(c *Context) error {
		return c.String("served")
	})

	h.ServeHTTP(rr, req)

	test.Equal(t, http.StatusOK, rr.Code)
	test.Equal(t, "served", rr.Body.String())
}

func TestServerStart(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	test.NoError(t, err)
	addr := ln.Addr().String()
	test.NoError(t, ln.Close())
	s := NewServer(Options{Addr: addr})
	s.Get("/", func(c *Context) error {
		return c.String("started")
	})
	s.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.Writer().Header().Set("X-Middleware", "true")
			return next(c)
		}
	})
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.Start()
	}()

	var res *http.Response
	for range 100 {
		res, err = http.Get("http://" + s.server.Addr + "/")
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	test.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	test.NoError(t, err)
	test.Equal(t, http.StatusOK, res.StatusCode)
	test.Equal(t, "true", res.Header.Get("X-Middleware"))
	test.Equal(t, "started", string(body))

	test.NoError(t, s.Stop())
	test.NoError(t, <-errCh)
}

func TestServerStartTLSReturnsError(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{
		Addr:        "127.0.0.1:0",
		CertFile:    "missing-cert.pem",
		CertKeyFile: "missing-key.pem",
	})

	err := s.Start()

	test.NotNil(t, err)
	test.True(t, strings.Contains(err.Error(), "missing-cert.pem"))
}

func TestServerStop(t *testing.T) {
	t.Parallel()

	s := NewServer(Options{Addr: ":0"})

	err := s.Stop()

	test.NoError(t, err)
	test.True(t, s.stopping.Load())
}

type failingWriter struct {
	header http.Header
}

func (w *failingWriter) Header() http.Header {
	return w.header
}

func (w *failingWriter) Write(b []byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *failingWriter) WriteHeader(statusCode int) {}
