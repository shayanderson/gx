package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

// LimitReadSize is the maximum size of a request body that will be read
// defaults to 5 MB
// set to 0 to disable limit
var LimitReadSize int64 = 5 * 1024 * 1024 // 5 MB

// responseWriter is a wrapper around http.ResponseWriter that tracks if the header has been written
type responseWriter struct {
	http.ResponseWriter
	headerWritten atomic.Bool
}

// Flush implements the http.Flusher interface
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements the http.Hijacker interface
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacker not supported")
	}
	return h.Hijack()
}

// Push implements the http.Pusher interface
func (w *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Write writes the given bytes to the response
func (w *responseWriter) Write(b []byte) (int, error) {
	if w.headerWritten.CompareAndSwap(false, true) {
		w.ResponseWriter.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader writes the HTTP status code to the response
func (w *responseWriter) WriteHeader(status int) {
	if w.headerWritten.CompareAndSwap(false, true) {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	// ignore duplicate header writes
}

// Context represents the context of an HTTP request
type Context struct {
	ctx     context.Context
	isMW    bool
	Request *http.Request
	writer  http.ResponseWriter
}

// NewContext creates a new Context
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		ctx:     r.Context(),
		Request: r,
		writer:  &responseWriter{ResponseWriter: w},
	}
}

// Bind binds the request body as JSON to the given struct
func (c *Context) Bind(v any) error {
	ct := c.Request.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		return Error(http.StatusBadRequest, "invalid content type, expected application/json")
	}
	var dec *json.Decoder
	if LimitReadSize > 0 {
		dec = json.NewDecoder(io.LimitReader(c.Request.Body, LimitReadSize))
	} else {
		dec = json.NewDecoder(c.Request.Body)
	}
	return dec.Decode(v)
}

// Context returns the underlying context.Context
func (c *Context) Context() context.Context {
	return c.ctx
}

// Get retrieves a value from the context by key
func (c *Context) Get(key any) any {
	return c.ctx.Value(key)
}

// isMiddleware returns true if the context is being used in middleware
func (c *Context) isMiddleware() bool {
	return c.isMW
}

// HTML writes an HTML response
// if a status code is provided, it writes that status code, otherwise defaults to 200
func (c *Context) HTML(s string, code ...int) error {
	w := c.Writer()
	if len(code) > 0 {
		c.Status(code[0])
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := io.WriteString(w, s)
	return err
}

// JSON writes the given value as JSON to the response
// if a status code is provided, it writes that status code, otherwise defaults to 200
// URL query parameter "pretty" can be used to pretty-print the JSON
func (c *Context) JSON(v any, code ...int) error {
	w := c.Writer()
	w.Header().Set("Content-Type", "application/json")
	if len(code) > 0 {
		c.Status(code[0])
	}
	enc := json.NewEncoder(w)
	if pretty := c.Request.URL.Query().Has("pretty"); pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// middleware marks the context as being used in middleware
func (c *Context) middleware() {
	c.isMW = true
}

// Redirect redirects the request to the given URL with the given status code
// if no status code is provided, it defaults to 303 See Other
func (c *Context) Redirect(url string, code ...int) {
	status := http.StatusSeeOther
	if len(code) > 0 {
		status = code[0]
	}
	http.Redirect(c.Writer(), c.Request, url, status)
}

// Set sets a value in the context by key
func (c *Context) Set(key, value any) {
	c.ctx = context.WithValue(c.ctx, key, value)
	c.Request = c.Request.WithContext(c.ctx)
}

// Status writes the HTTP status code in the response
func (c *Context) Status(code int) {
	c.writer.WriteHeader(code)
}

// String writes a plain text response
// if a status code is provided, it writes that status code, otherwise defaults to 200
func (c *Context) String(s string, code ...int) error {
	w := c.Writer()
	if len(code) > 0 {
		c.Status(code[0])
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := io.WriteString(w, s)
	return err
}

// Writer returns the underlying http.ResponseWriter
func (c *Context) Writer() http.ResponseWriter {
	return c.writer
}
