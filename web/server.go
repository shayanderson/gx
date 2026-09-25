package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// RootPattern is a pattern that matches the root path "/".
const RootPattern = "/{$}"

// DefaultShutdownTimeout is the default maximum time Stop waits for in-flight requests.
const DefaultShutdownTimeout = 2 * time.Second

// ErrorHandlerFunc handles a status error response.
type ErrorHandlerFunc func(*Context, StatusError)

// HandlerFunc is a http handler that returns an error.
type HandlerFunc func(*Context) error

// Serve serves an HTTP request.
func (h HandlerFunc) Serve(c *Context) {
	if hErr := h(c); hErr != nil {
		var err StatusError
		if sErr, ok := errors.AsType[StatusError](hErr); ok {
			err = sErr
		} else {
			err = statusError{
				err:    hErr,
				status: http.StatusInternalServerError,
			}
		}

		// write error response
		status := err.Status()
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		c.logError(status, err)

		// Error handler cannot change a response that has already started.
		if c.responseWritten() {
			return
		}

		// Use custom error handler if set.
		if c.errorHandler != nil {
			c.errorHandler(c, err)
			return
		}

		// Fallback error response.
		if err := c.JSON(map[string]string{"error": err.Error()}, status); err != nil {
			c.logErrorResponseWrite(err)
		}
	}
}

// ServeHTTP serves a handler using the default context configuration.
// To serve a handler with Server options and global middleware, use Server.Handler.
func (r HandlerFunc) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := NewContext(w, req)
	r.Serve(c)
}

// Middleware is a function that wraps a Handler.
type Middleware func(HandlerFunc) HandlerFunc

// chain applies middleware to a handler.
func chain(h HandlerFunc, middleware ...Middleware) HandlerFunc {
	for _, m := range middleware {
		h = m(h)
	}
	return h
}

// Options holds the configuration options for the Server.
type Options struct {
	// Addr is the address to listen on.
	Addr string

	// CertFile is the path to the TLS certificate file.
	CertFile string

	// CertKeyFile is the path to the TLS certificate key file.
	CertKeyFile string

	// IdleTimeout is the maximum amount of time to wait for the next request
	// when keep-alive is enabled.
	IdleTimeout time.Duration

	// Logger receives request and error logs. A nil logger disables logging.
	Logger *slog.Logger

	// LogPrefix prefixes request and error messages. It defaults to "http".
	LogPrefix string

	// ErrorHandler handles errors returned by handlers. A nil handler writes a JSON error response.
	ErrorHandler ErrorHandlerFunc

	// MaxReadSize is the maximum number of request-body bytes read by Bind.
	// A value of 0 uses DefaultMaxReadSize. A negative value disables the limit.
	MaxReadSize int64

	// MaxHeaderBytes is the maximum size of request headers.
	MaxHeaderBytes int

	// MaxHeaderValueCount is the maximum number of request header values.
	MaxHeaderValueCount int

	// ReadHeaderTimeout is the amount of time allowed to read request headers.
	ReadHeaderTimeout time.Duration

	// ReadTimeout is the maximum duration for reading the entire request, including the body.
	ReadTimeout time.Duration

	// ShutdownTimeout is the maximum time Stop waits for in-flight requests.
	// It defaults to DefaultShutdownTimeout.
	ShutdownTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration
}

// Server is a simple HTTP server with middleware support
type Server struct {
	contextOpts     contextOptions
	middleware      []Middleware
	middlewareChain func() HandlerFunc
	mux             *http.ServeMux
	opts            Options
	server          *http.Server
	stopping        atomic.Bool
}

// NewServer creates a new server instance.
func NewServer(opts Options) *Server {
	if opts.LogPrefix == "" {
		opts.LogPrefix = "http"
	}
	if opts.ReadHeaderTimeout == 0 {
		opts.ReadHeaderTimeout = 3 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 5 * time.Second
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 5 * time.Second
	}
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = DefaultShutdownTimeout
	}
	contextOpts := contextOptions{
		errorHandler: opts.ErrorHandler,
		logger:       opts.Logger,
		logPrefix:    opts.LogPrefix,
		maxReadSize:  DefaultMaxReadSize,
	}
	if opts.MaxReadSize > 0 {
		contextOpts.maxReadSize = opts.MaxReadSize
	} else if opts.MaxReadSize < 0 {
		contextOpts.maxReadSize = 0
	}

	s := &Server{
		contextOpts: contextOpts,
		opts:        opts,
		mux:         http.NewServeMux(),
	}
	s.middlewareChain = sync.OnceValue(func() HandlerFunc {
		h := HandlerFunc(func(c *Context) error {
			s.mux.ServeHTTP(c.Writer(), c.Request)
			return nil
		})
		for _, middleware := range slices.Backward(s.middleware) {
			h = middleware(h)
		}
		return h
	})
	s.server = &http.Server{
		Addr:                opts.Addr,
		Handler:             s.Handler(),
		IdleTimeout:         opts.IdleTimeout,
		MaxHeaderBytes:      opts.MaxHeaderBytes,
		MaxHeaderValueCount: opts.MaxHeaderValueCount,
		ReadHeaderTimeout:   opts.ReadHeaderTimeout,
		ReadTimeout:         opts.ReadTimeout,
		WriteTimeout:        opts.WriteTimeout,
	}
	return s
}

// Delete registers a new DELETE route with a handler.
func (s *Server) Delete(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodDelete+" "+pattern, handler, middleware...)
}

// Get registers a new GET route with a handler.
func (s *Server) Get(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodGet+" "+pattern, handler, middleware...)
}

// Handle registers a new route with a handler.
func (s *Server) Handle(pattern string, handler HandlerFunc, middleware ...Middleware) {
	h := chain(handler, middleware...)

	s.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := newContext(w, r, s.contextOpts)
		h.Serve(c)
	}))
}

// Handler returns an HTTP handler that applies the server's options and global middleware.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

// Mux returns the underlying http.ServeMux.
// Requests served directly through the mux bypass global middleware and request logging.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// Patch registers a new PATCH route with a handler.
func (s *Server) Patch(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodPatch+" "+pattern, handler, middleware...)
}

// Post registers a new POST route with a handler.
func (s *Server) Post(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodPost+" "+pattern, handler, middleware...)
}

// Put registers a new PUT route with a handler.
func (s *Server) Put(pattern string, handler HandlerFunc, middleware ...Middleware) {
	s.Handle(http.MethodPut+" "+pattern, handler, middleware...)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	var err error
	if s.opts.CertFile != "" && s.opts.CertKeyFile != "" {
		err = s.server.ListenAndServeTLS(s.opts.CertFile, s.opts.CertKeyFile)
	} else {
		err = s.server.ListenAndServe()
	}
	if err != nil && errors.Is(err, http.ErrServerClosed) && s.stopping.Load() {
		return nil
	}
	return err
}

// Stop stops the HTTP server.
func (s *Server) Stop() error {
	s.stopping.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.ShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Use adds middleware to the server. It must be called before the server begins serving requests.
// Route handler errors are handled before they return to global middleware, so next returns nil for
// route errors.
func (s *Server) Use(middleware ...Middleware) {
	s.middleware = append(s.middleware, middleware...)
}

// serveHTTP logs and dispatches an HTTP request.
func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	c := newContext(w, r, s.contextOpts)
	c.logRequest()

	s.middlewareChain().Serve(c)
}
