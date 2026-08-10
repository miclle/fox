package fox

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fox-gonic/fox/logger"
)

// Context with engine
type Context struct {
	*gin.Context

	engine *Engine
	Logger logger.Logger
	// baseCtx is an immutable snapshot of the request context taken when the
	// fox.Context is created. It backs Done/Err/Value/Deadline so that calls
	// from asynchronous goroutines stay stable even after the underlying
	// gin.Context is recycled by the sync.Pool (see PR #30).
	baseCtx context.Context
	// Request is the http request copy from gin.Context.
	// It carries the latest value through the middleware chain: a replacement
	// made by an inner middleware (e.g. c.Request.WithContext(...)) is visible
	// to outer middleware after Next() returns. The context.Context interface
	// methods (Done/Err/Value/Deadline) read baseCtx instead, so Request is
	// only ever touched synchronously.
	Request *http.Request
}

// RequestBody return request body bytes
// see c.ShouldBindBodyWith
func (c *Context) RequestBody() (body []byte, err error) {
	if cb, ok := c.Get(gin.BodyBytesKey); ok {
		if cbb, ok := cb.([]byte); ok {
			body = cbb
		}
	}

	if body == nil && c.Request != nil && c.Request.Body != nil {
		var (
			buf   bytes.Buffer
			bodyR = io.TeeReader(c.Request.Body, &buf)
		)
		if body, err = io.ReadAll(bodyR); err != nil {
			return body, err
		}

		c.Set(gin.BodyBytesKey, body)

		// copy the request body to the next handler
		c.Request.Body = io.NopCloser(&buf)
	}
	return body, err
}

// TraceID returns the request trace ID. It checks the gin context, request
// header, and response header in order, falling back to generating a new ID
// which is then written to both the response header and gin context.
//
// Note: This method has a side effect when no trace ID exists. If you only
// want to read without generating, check c.GetHeader(logger.TraceID) directly.
func (c *Context) TraceID() string {
	if id, exists := c.Get(logger.TraceID); exists {
		return id.(string)
	}

	if id := c.GetHeader(logger.TraceID); len(id) > 0 {
		return id
	}

	if id := c.Context.Writer.Header().Get(logger.TraceID); len(id) > 0 {
		return id
	}

	id := logger.DefaultGenRequestID()

	c.Header(logger.TraceID, id)
	c.Set(logger.TraceID, id)

	return id
}

func (c *Context) Done() <-chan struct{} {
	return c.base().Done()
}

func (c *Context) Err() error {
	return c.base().Err()
}

// Value returns the value associated with key from the request context
// snapshot taken when this Context was created. It is safe to call from
// asynchronous goroutines, but it does not observe values injected into the
// request later in the middleware chain (e.g. c.Request.WithContext(...)).
// To read live injected values within the synchronous chain, use
// c.Request.Context().Value(key).
func (c *Context) Value(key any) any {
	return c.base().Value(key)
}

func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.base().Deadline()
}

// base returns the immutable snapshot backing the context.Context lifecycle
// methods. Callers that construct a Context directly (tests) fall back to the
// live request context.
func (c *Context) base() context.Context {
	if c.baseCtx != nil {
		return c.baseCtx
	}
	return c.Request.Context()
}

func (c *Context) Next() {
	c.Context.Request = c.Request
	c.Context.Next()
	// Sync back the latest request (see the Request field doc).
	c.Request = c.Context.Request
}

func (c *Context) Copy() *Context {
	ginCtx := c.Context.Copy()
	ginCtx.Request = c.Request
	var baseCtx context.Context
	if c.Request != nil {
		baseCtx = c.Request.Context()
	}
	return &Context{
		Context: ginCtx,
		engine:  c.engine,
		Logger:  c.Logger,
		baseCtx: baseCtx,
		Request: c.Request,
	}
}
