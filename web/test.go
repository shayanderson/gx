package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServer is a test HTTP server.
type TestServer struct {
	client *http.Client
	server *Server
	test   *httptest.Server
}

// NewTestServer creates a new in-memory test HTTP server.
// If options are provided, only the first value is used.
func NewTestServer(t testing.TB, options ...Options) *TestServer {
	t.Helper()

	opts := Options{
		Addr: ":0",
	}
	if len(options) > 0 {
		opts = options[0]
	}

	s := NewServer(opts)

	ts := httptest.NewTestServer(t, http.HandlerFunc(s.serveHTTP))

	return &TestServer{
		client: ts.Client(),
		server: s,
		test:   ts,
	}
}

// Client returns the HTTP client.
func (t *TestServer) Client() *http.Client {
	return t.client
}

// DELETE registers a new DELETE route with a handler.
func (t *TestServer) DELETE(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodDelete+" "+pattern, handler, middleware...)
}

// GET registers a new GET route with a handler.
func (t *TestServer) GET(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodGet+" "+pattern, handler, middleware...)
}

// Handle registers a new route with a handler.
func (t *TestServer) Handle(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(pattern, handler, middleware...)
}

// Mux returns the underlying http.ServeMux.
func (t *TestServer) Mux() *http.ServeMux {
	return t.server.Mux()
}

// PATCH registers a new PATCH route with a handler.
func (t *TestServer) PATCH(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodPatch+" "+pattern, handler, middleware...)
}

// POST registers a new POST route with a handler.
func (t *TestServer) POST(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodPost+" "+pattern, handler, middleware...)
}

// PUT registers a new PUT route with a handler.
func (t *TestServer) PUT(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodPut+" "+pattern, handler, middleware...)
}

// Start starts the HTTP server.
func (t *TestServer) Start() error {
	return nil
}

// Stop stops the HTTP server and closes the test server.
func (t *TestServer) Stop() error {
	t.test.Close()
	return nil
}

// URL returns the full test server URL with the given path.
func (t *TestServer) URL(path string) string {
	return t.test.URL + path
}

// Use adds middleware to the server.
func (t *TestServer) Use(middleware ...Middleware) {
	t.server.Use(middleware...)
}
