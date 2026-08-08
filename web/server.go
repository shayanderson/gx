package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// RootPattern is a pattern that matches the root path "/"
const RootPattern = "/{$}"

// ErrorHandlerFunc is the default error handler used for handling error responses
var ErrorHandler ErrorHandlerFunc

// ErrorHandler is a custom error handler for handling error responses
type ErrorHandlerFunc func(*Context, StatusError)

// HandlerFunc is a http handler that returns an error
type HandlerFunc func(*Context) error

// Serve serves an HTTP request
func (h HandlerFunc) Serve(c *Context) {
	if !c.isMiddleware() {
		// log request when not in middleware
		slog.Info( // #nosec G706
			fmt.Sprintf(
				"http: %s http://%s%s %s from %s",
				c.Request.Method,
				c.Request.Host,
				c.Request.RequestURI,
				c.Request.Proto,
				c.Request.RemoteAddr,
			),
		)
	}

	if hErr := h(c); hErr != nil {
		var err StatusError
		if sErr, ok := hErr.(StatusError); ok {
			err = sErr
		} else {
			err = statusError{
				err:    hErr,
				status: http.StatusInternalServerError,
			}
		}
		// log error
		slog.Error(fmt.Sprintf( // #nosec G706
			"http: %s http://%s%s %s from %s (%d)",
			c.Request.Method,
			c.Request.Host,
			c.Request.RequestURI,
			c.Request.Proto,
			c.Request.RemoteAddr,
			err.Status(),
		), slog.String("err", err.Error()))
		// write error response
		code := err.Status()
		if code < 400 || code > 599 {
			code = http.StatusInternalServerError
		}
		// use custom error handler if set
		if ErrorHandler != nil {
			ErrorHandler(c, err)
			return
		}
		// fallback error response
		if err := c.JSON(map[string]string{"error": err.Error()}, code); err != nil {
			panic("http server failed to write error response: " + err.Error())
		}
	}
}

// ServeHTTP serves an HTTP request
func (r HandlerFunc) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := NewContext(w, req)
	defer c.Request.Body.Close()

	r.Serve(c)
}

// Middleware is a function that wraps a Handler
type Middleware func(HandlerFunc) HandlerFunc

// chain applies middleware to a handler
func chain(h HandlerFunc, middleware ...Middleware) HandlerFunc {
	for _, m := range middleware {
		h = m(h)
	}
	return h
}

// Options holds the configuration options for the Server
type Options struct {
	// Addr is the address to listen on
	Addr string

	// CertFile is the path to the TLS certificate file
	CertFile string

	// CertKeyFile is the path to the TLS certificate key file
	CertKeyFile string

	// IdleTimeout is the maximum amount of time to wait for the next request
	// when keep-alive is enabled
	IdleTimeout time.Duration

	// ReadHeaderTimeout is the amount of time allowed to read request headers
	ReadHeaderTimeout time.Duration

	// ReadTimeout is the maximum duration for reading the entire request, including the body
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration
}

// Server is a simple HTTP server with middleware support
type Server struct {
	middleware []Middleware
	mux        *http.ServeMux
	opts       Options
	server     *http.Server
	stopping   atomic.Bool
}

// NewServer creates a new server instance
func NewServer(opts Options) *Server {
	if opts.ReadHeaderTimeout == 0 {
		opts.ReadHeaderTimeout = 3 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 5 * time.Second
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 5 * time.Second
	}

	s := &Server{
		opts: opts,
		mux:  http.NewServeMux(),
	}
	s.server = &http.Server{
		Addr:              opts.Addr,
		Handler:           s.mux,
		IdleTimeout:       opts.IdleTimeout,
		ReadHeaderTimeout: opts.ReadHeaderTimeout,
		ReadTimeout:       opts.ReadTimeout,
		WriteTimeout:      opts.WriteTimeout,
	}
	return s
}

// Delete registers a new DELETE route with a handler
func (s *Server) Delete(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodDelete+" "+pattern, handler, middleware...)
}

// Get registers a new GET route with a handler
func (s *Server) Get(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodGet+" "+pattern, handler, middleware...)
}

// Handle registers a new route with a handler
func (s *Server) Handle(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.mux.Handle(pattern, chain(handler, middleware...))
}

// Mux returns the underlying http.ServeMux
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// Patch registers a new PATCH route with a handler
func (s *Server) Patch(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodPatch+" "+pattern, handler, middleware...)
}

// Post registers a new POST route with a handler
func (s *Server) Post(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodPost+" "+pattern, handler, middleware...)
}

// Put registers a new PUT route with a handler
func (s *Server) Put(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodPut+" "+pattern, handler, middleware...)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// base handler to start the chain
	h := HandlerFunc(func(c *Context) error {
		s.mux.ServeHTTP(c.Writer(), c.Request)
		return nil
	})

	// apply middleware
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = s.middleware[i](h)
	}

	// wrap base handler
	s.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := NewContext(w, r)
		c.middleware()
		h.Serve(c)
	})

	slog.Info("http server: starting", slog.String("addr", s.opts.Addr))
	var err error
	if s.opts.CertFile != "" && s.opts.CertKeyFile != "" {
		err = s.server.ListenAndServeTLS(s.opts.CertFile, s.opts.CertKeyFile)
	} else {
		err = s.server.ListenAndServe()
	}
	if err != nil && err == http.ErrServerClosed && s.stopping.Load() {
		return nil
	}
	return err
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	slog.Info("http server: stopping")
	s.stopping.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Use adds middleware to the server
func (s *Server) Use(middleware ...Middleware) {
	s.middleware = append(s.middleware, middleware...)
}
