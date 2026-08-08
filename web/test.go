package web

import (
	"net/http"
	"net/http/httptest"
)

// TestServer is a test HTTP server
type TestServer struct {
	client *http.Client
	server *Server
	test   *httptest.Server
}

// NewTestServer creates a new test HTTP server
func NewTestServer() *TestServer {
	s := NewServer(Options{
		Addr: ":0",
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// apply middleware
		h := HandlerFunc(func(c *Context) error {
			s.mux.ServeHTTP(c.Writer(), c.Request)
			return nil
		})
		for i := len(s.middleware) - 1; i >= 0; i-- {
			h = s.middleware[i](h)
		}
		h.ServeHTTP(w, r)
	}))

	return &TestServer{
		client: ts.Client(),
		server: s,
		test:   ts,
	}
}

// Client returns the HTTP client
func (t *TestServer) Client() *http.Client {
	return t.client
}

// Delete registers a new DELETE route with a handler
func (t *TestServer) Delete(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodDelete+" "+pattern, handler, middleware...)
}

// Get registers a new GET route with a handler
func (t *TestServer) Get(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodGet+" "+pattern, handler, middleware...)
}

// Handle registers a new route with a handler
func (t *TestServer) Handle(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(pattern, handler, middleware...)
}

// Mux returns the underlying http.ServeMux
func (t *TestServer) Mux() *http.ServeMux {
	return t.server.Mux()
}

// Patch registers a new PATCH route with a handler
func (t *TestServer) Patch(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodPatch+" "+pattern, handler, middleware...)
}

// Post registers a new POST route with a handler
func (t *TestServer) Post(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodPost+" "+pattern, handler, middleware...)
}

// Put registers a new PUT route with a handler
func (t *TestServer) Put(pattern string, handler HandlerFunc, middleware ...Middleware) {
	t.server.Handle(http.MethodPut+" "+pattern, handler, middleware...)
}

// Start starts the HTTP server
func (t *TestServer) Start() error {
	return nil
}

// Stop stops the HTTP server and closes the test server
func (t *TestServer) Stop() error {
	t.test.Close()
	return nil
}

// URL returns the full test server URL with the given path
func (t *TestServer) URL(path string) string {
	return t.test.URL + path
}

// Use adds middleware to the server
func (t *TestServer) Use(middleware ...Middleware) {
	t.server.Use(middleware...)
}
