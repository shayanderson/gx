package web

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shayanderson/gx/test"
)

func TestNewTestServer(t *testing.T) {
	ts := NewTestServer()
	defer ts.Stop()

	test.NotNil(t, ts.Client())
	test.NotNil(t, ts.Mux())
	test.NotEmpty(t, ts.URL("/"))
}

func TestTestServerURL(t *testing.T) {
	ts := NewTestServer()
	defer ts.Stop()

	url := ts.URL("/hello")

	test.True(t, strings.HasPrefix(url, "http://"))
	test.True(t, strings.HasSuffix(url, "/hello"))
}

func TestTestServerStartAndStop(t *testing.T) {
	ts := NewTestServer()

	test.NoError(t, ts.Start())
	test.NoError(t, ts.Stop())
}

func TestTestServerMux(t *testing.T) {
	ts := NewTestServer()
	defer ts.Stop()

	test.Same(t, ts.server.Mux(), ts.Mux())
}

func TestTestServerHandle(t *testing.T) {
	ts := NewTestServer()
	defer ts.Stop()
	ts.Handle("GET /hello", func(c *Context) error {
		return c.String("hello")
	})

	body, status := testServerRequest(t, ts, http.MethodGet, "/hello")

	test.Equal(t, http.StatusOK, status)
	test.Equal(t, "hello", body)
}

func TestTestServerMethodHelpers(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		add    func(*TestServer, string, HandlerFunc, ...Middleware)
	}{
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   "/delete",
			body:   "deleted",
			add:    (*TestServer).Delete,
		},
		{name: "get", method: http.MethodGet, path: "/get", body: "got", add: (*TestServer).Get},
		{
			name:   "patch",
			method: http.MethodPatch,
			path:   "/patch",
			body:   "patched",
			add:    (*TestServer).Patch,
		},
		{
			name:   "post",
			method: http.MethodPost,
			path:   "/post",
			body:   "posted",
			add:    (*TestServer).Post,
		},
		{name: "put", method: http.MethodPut, path: "/put", body: "put", add: (*TestServer).Put},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := NewTestServer()
			defer ts.Stop()
			tc.add(ts, tc.path, func(c *Context) error {
				return c.String(tc.body)
			})

			body, status := testServerRequest(t, ts, tc.method, tc.path)

			test.Equal(t, http.StatusOK, status)
			test.Equal(t, tc.body, body)
		})
	}
}

func TestTestServerRouteMiddleware(t *testing.T) {
	ts := NewTestServer()
	defer ts.Stop()
	ts.Get("/", func(c *Context) error {
		return c.String(c.Get("route").(string))
	}, func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.Set("route", "middleware")
			return next(c)
		}
	})

	body, status := testServerRequest(t, ts, http.MethodGet, "/")

	test.Equal(t, http.StatusOK, status)
	test.Equal(t, "middleware", body)
}

func TestTestServerUse(t *testing.T) {
	ts := NewTestServer()
	defer ts.Stop()
	ts.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.Writer().Header().Set("X-Test", "true")
			return next(c)
		}
	})
	ts.Get("/", func(c *Context) error {
		return c.String("used")
	})

	res, err := ts.Client().Get(ts.URL("/"))
	test.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	test.NoError(t, err)

	test.Equal(t, http.StatusOK, res.StatusCode)
	test.Equal(t, "true", res.Header.Get("X-Test"))
	test.Equal(t, "used", string(body))
}

func testServerRequest(t *testing.T, ts *TestServer, method, path string) (string, int) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL(path), nil)
	test.NoError(t, err)
	res, err := ts.Client().Do(req)
	test.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	test.NoError(t, err)
	return string(body), res.StatusCode
}
