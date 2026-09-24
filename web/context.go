package web

import (
	"bufio"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

// DefaultMaxReadSize is the default maximum number of request-body bytes read by Bind.
const DefaultMaxReadSize int64 = 5 * 1024 * 1024

// responseWriter is a wrapper around http.ResponseWriter that tracks if the header has
// been written.
type responseWriter struct {
	http.ResponseWriter
	headerWritten atomic.Bool
}

// Flush implements the http.Flusher interface.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		w.headerWritten.Store(true)
		f.Flush()
	}
}

// Hijack implements the http.Hijacker interface.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacker not supported")
	}
	return h.Hijack()
}

// Push implements the http.Pusher interface.
func (w *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Unwrap returns the underlying response writer.
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Write writes the given bytes to the response.
func (w *responseWriter) Write(b []byte) (int, error) {
	if w.headerWritten.CompareAndSwap(false, true) {
		w.ResponseWriter.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader writes the HTTP status code to the response.
func (w *responseWriter) WriteHeader(status int) {
	if status >= http.StatusContinue &&
		status < http.StatusOK &&
		status != http.StatusSwitchingProtocols {
		if !w.headerWritten.Load() {
			w.ResponseWriter.WriteHeader(status)
		}
		return
	}
	if w.headerWritten.CompareAndSwap(false, true) {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	// Ignore duplicate header writes.
}

// Context represents the context of an HTTP request.
type Context struct {
	Request                 *http.Request
	allowNonJSONContentType bool
	ctx                     context.Context
	errorHandler            ErrorHandlerFunc
	logger                  *slog.Logger
	logPrefix               string
	maxReadSize             int64
	writer                  *responseWriter
}

// contextOptions holds the configuration options for a Context.
type contextOptions struct {
	allowNonJSONContentType bool
	errorHandler            ErrorHandlerFunc
	logger                  *slog.Logger
	logPrefix               string
	maxReadSize             int64
}

// NewContext creates a new Context.
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return newContext(w, r, contextOptions{maxReadSize: DefaultMaxReadSize})
}

// newContext creates a new Context with the given response writer, request, and context options.
func newContext(w http.ResponseWriter, r *http.Request, opts contextOptions) *Context {
	writer, ok := w.(*responseWriter)
	if !ok {
		writer = &responseWriter{ResponseWriter: w}
	}

	return &Context{
		Request:                 r,
		allowNonJSONContentType: opts.allowNonJSONContentType,
		ctx:                     r.Context(),
		errorHandler:            opts.errorHandler,
		logger:                  opts.logger,
		logPrefix:               opts.logPrefix,
		maxReadSize:             opts.maxReadSize,
		writer:                  writer,
	}
}

// Bind binds a JSON request body to the given struct. By default, it uses a lightweight
// Content-Type check that accepts application/json with optional parameters.
func (c *Context) Bind(v any) error {
	contentType := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("Content-Type")))
	suffix, isJSONContentType := strings.CutPrefix(contentType, "application/json")
	if !c.allowNonJSONContentType &&
		(!isJSONContentType ||
			(suffix != "" && !strings.HasPrefix(strings.TrimSpace(suffix), ";"))) {
		return Error(http.StatusBadRequest, "invalid content type, expected application/json")
	}

	r := io.Reader(c.Request.Body)
	if c.maxReadSize > 0 {
		// Use the underlying writer so MaxBytesReader can notify net/http to
		// close the connection after a body-limit error.
		r = http.MaxBytesReader(c.writer.Unwrap(), c.Request.Body, c.maxReadSize)
	}

	err := json.UnmarshalRead(r, v)
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return ErrorWrap(http.StatusRequestEntityTooLarge, err)
	}
	return Error(http.StatusBadRequest, "invalid json body")
}

// Context returns the underlying context.Context.
func (c *Context) Context() context.Context {
	return c.ctx
}

// Get retrieves a value from the context by key.
func (c *Context) Get(key any) any {
	return c.ctx.Value(key)
}

// HTML writes an HTML response.
// If a status code is provided, it writes that status code, otherwise defaults to 200.
func (c *Context) HTML(s string, code ...int) error {
	w := c.Writer()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(code) > 0 {
		c.Status(code[0])
	}
	_, err := io.WriteString(w, s)
	return err
}

// JSON writes the given value as JSON to the response.
// If a status code is provided, it writes that status code, otherwise defaults to 200.
// URL query parameter "pretty" can be used to pretty-print the JSON.
func (c *Context) JSON(v any, code ...int) error {
	var (
		data []byte
		err  error
	)
	if c.Request.URL.Query().Has("pretty") {
		data, err = json.Marshal(v, jsontext.WithIndent("  "))
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}

	w := c.Writer()
	w.Header().Set("Content-Type", "application/json")

	if len(code) > 0 {
		c.Status(code[0])
	}
	_, err = w.Write(data)
	return err
}

// Redirect redirects the request to the given URL with the given status code.
// If no status code is provided, it defaults to 303 See Other.
func (c *Context) Redirect(url string, code ...int) {
	status := http.StatusSeeOther
	if len(code) > 0 {
		status = code[0]
	}
	http.Redirect(c.Writer(), c.Request, url, status)
}

// Set sets a value in the context by key.
func (c *Context) Set(key, value any) {
	c.ctx = context.WithValue(c.ctx, key, value)
	c.Request = c.Request.WithContext(c.ctx)
}

// Status writes the HTTP status code in the response.
func (c *Context) Status(code int) {
	c.writer.WriteHeader(code)
}

// String writes a plain text response.
// If a status code is provided, it writes that status code, otherwise defaults to 200.
func (c *Context) String(s string, code ...int) error {
	w := c.Writer()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if len(code) > 0 {
		c.Status(code[0])
	}
	_, err := io.WriteString(w, s)
	return err
}

// Writer returns the underlying http.ResponseWriter.
func (c *Context) Writer() http.ResponseWriter {
	return c.writer
}

// logError logs an HTTP request error.
// It logs the error with the appropriate severity level based on the status code.
func (c *Context) logError(status int, err StatusError) {
	if c.logger == nil {
		return
	}

	level := slog.LevelError
	if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
		level = slog.LevelWarn
	}
	c.logger.LogAttrs(c.Context(), level, fmt.Sprintf(
		"%s: %s %s %s from %s (%d)",
		c.logPrefix,
		c.Request.Method,
		c.requestURL(),
		c.Request.Proto,
		c.Request.RemoteAddr,
		status,
	), slog.String("err", err.Error()))
}

// logErrorResponseWrite logs a failure to write a fallback error response.
// It includes the error message in the log.
func (c *Context) logErrorResponseWrite(err error) {
	if c.logger == nil {
		return
	}

	c.logger.ErrorContext(c.Context(), fmt.Sprintf(
		"%s: failed to write error response", c.logPrefix,
	), slog.String("err", err.Error()))
}

// logRequest logs an HTTP request.
func (c *Context) logRequest() {
	if c.logger == nil {
		return
	}

	c.logger.InfoContext(c.Context(), fmt.Sprintf(
		"%s: %s %s %s from %s",
		c.logPrefix,
		c.Request.Method,
		c.requestURL(),
		c.Request.Proto,
		c.Request.RemoteAddr,
	))
}

// requestURL returns the full URL of the request.
func (c *Context) requestURL() string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + c.Request.RequestURI
}

// responseWritten returns true if the response headers have already been written.
func (c *Context) responseWritten() bool {
	return c.writer.headerWritten.Load()
}
