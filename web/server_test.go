package web

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shayanderson/gx/test"
)

func TestNewServer(t *testing.T) {
	s := NewServer(Options{Addr: ":0"})

	test.Equal(t, ":0", s.opts.Addr)
	test.Equal(t, 3*time.Second, s.opts.ReadHeaderTimeout)
	test.Equal(t, 5*time.Second, s.opts.ReadTimeout)
	test.Equal(t, 5*time.Second, s.opts.WriteTimeout)
	test.NotNil(t, s.mux)
	test.NotNil(t, s.server)
}

func TestNewServerPreservesOptions(t *testing.T) {
	s := NewServer(Options{
		Addr:              ":1234",
		CertFile:          "cert.pem",
		CertKeyFile:       "key.pem",
		IdleTimeout:       time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      4 * time.Second,
	})

	test.Equal(t, ":1234", s.opts.Addr)
	test.Equal(t, "cert.pem", s.opts.CertFile)
	test.Equal(t, "key.pem", s.opts.CertKeyFile)
	test.Equal(t, time.Second, s.opts.IdleTimeout)
	test.Equal(t, 2*time.Second, s.opts.ReadHeaderTimeout)
	test.Equal(t, 3*time.Second, s.opts.ReadTimeout)
	test.Equal(t, 4*time.Second, s.opts.WriteTimeout)
}

func TestServerMux(t *testing.T) {
	s := NewServer(Options{Addr: ":0"})

	test.Same(t, s.mux, s.Mux())
}

func TestServerHandle(t *testing.T) {
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

func TestServerMethodHelpers(t *testing.T) {
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

func TestChain(t *testing.T) {
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
	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return c.JSON(map[string]string{"ok": "true"}, http.StatusAccepted)
	})

	h.Serve(c)

	test.Equal(t, http.StatusAccepted, rr.Code)
	test.Equal(t, `{"ok":"true"}`+"\n", rr.Body.String())
}

func TestHandlerFuncServeStatusError(t *testing.T) {
	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return Error(http.StatusTeapot, "short and stout")
	})

	h.Serve(c)

	test.Equal(t, http.StatusTeapot, rr.Code)
	test.Equal(t, `{"error":"short and stout"}`+"\n", rr.Body.String())
}

func TestHandlerFuncServeErrorDefaultsToInternalServerError(t *testing.T) {
	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return errors.New("failed")
	})

	h.Serve(c)

	test.Equal(t, http.StatusInternalServerError, rr.Code)
	test.Equal(t, `{"error":"failed"}`+"\n", rr.Body.String())
}

func TestHandlerFuncServeInvalidStatusErrorDefaultsToInternalServerError(t *testing.T) {
	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	h := HandlerFunc(func(c *Context) error {
		return Error(http.StatusOK, "bad status")
	})

	h.Serve(c)

	test.Equal(t, http.StatusInternalServerError, rr.Code)
	test.Equal(t, `{"error":"bad status"}`+"\n", rr.Body.String())
}

func TestHandlerFuncServePanicsWhenErrorResponseWriteFails(t *testing.T) {
	c := NewContext(
		&failingWriter{header: make(http.Header)},
		httptest.NewRequest(http.MethodGet, "/", nil),
	)
	h := HandlerFunc(func(c *Context) error {
		return errors.New("failed")
	})

	test.Panics(t, func() {
		h.Serve(c)
	})
}

func TestHandlerFuncServeCustomErrorHandler(t *testing.T) {
	original := ErrorHandler
	defer func() { ErrorHandler = original }()
	rr := httptest.NewRecorder()
	c := NewContext(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	ErrorHandler = func(c *Context, err StatusError) {
		test.Equal(t, http.StatusBadRequest, err.Status())
		test.Equal(t, "bad", err.Error())
		_ = c.String("custom", http.StatusBadRequest)
	}
	h := HandlerFunc(func(c *Context) error {
		return Error(http.StatusBadRequest, "bad")
	})

	h.Serve(c)

	test.Equal(t, http.StatusBadRequest, rr.Code)
	test.Equal(t, "custom", rr.Body.String())
}

func TestHandlerFuncServeHTTP(t *testing.T) {
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
			test.True(t, c.isMiddleware())
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
