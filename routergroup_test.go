package fox_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fox-gonic/fox"
)

func foo(c *fox.Context) (res any, err error) {
	res = "foo"
	return res, err
}

func boo(c *fox.Context) (res any, err error) {
	res = "boo"
	return res, err
}

func TestRouterGroup(t *testing.T) {
	assert := assert.New(t)

	router := fox.New()
	api := router.Group("/api")

	api.GET("foo", foo)

	api.GET("boo", boo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/foo", nil)
	router.ServeHTTP(w, req)
	assert.Equal(http.StatusOK, w.Code)
	assert.Equal("foo", w.Body.String())

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/boo", nil)
	router.ServeHTTP(w, req)
	assert.Equal(http.StatusOK, w.Code)
	assert.Equal("boo", w.Body.String())
}

func TestRouterGroupHandleInvalidHandler(t *testing.T) {
	router := fox.New()

	assert.Panics(t, func() {
		router.GET("too-many-values", func(c *fox.Context) (res any, other any, err error) { return res, other, err })
	})

	assert.Panics(t, func() {
		router.GET("invalid", "not a function")
	})

	assert.Panics(t, func() {
		router.Handle(http.MethodGet, "/invalid", func(i int) string { return "" })
	})
}

func TestRouterGroup_Use(t *testing.T) {
	router := fox.New()
	type ctxKey struct{}
	router.Use(func(c *fox.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKey{}, "context value"))
		// do not call the c.Next()
	})
	router.GET("/test", func(c *fox.Context) {
		val := c.Value(ctxKey{})
		if val != nil {
			c.String(200, val.(string))
		} else {
			c.String(200, "no context value")
		}
	})

	t.Run("with context value", func(t *testing.T) {
		assert := assert.New(t)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(200, w.Code)
		assert.Equal("context value", w.Body.String())
	})
}

// TestRouterGroup_RequestVisibleToOuterMiddleware reproduces issue #79:
// a middleware that replaces c.Request (e.g. c.Request.WithContext(...))
// must be visible to outer (earlier-registered) middleware reading after c.Next().
func TestRouterGroup_RequestVisibleToOuterMiddleware(t *testing.T) {
	type ctxKey struct{}

	tests := []struct {
		name     string
		path     string
		expected string
		register func(*fox.Engine)
	}{
		{
			name:     "two middleware layers",
			path:     "/test",
			expected: "hello",
			register: func(e *fox.Engine) {
				e.Use(func(c *fox.Context) {
					c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKey{}, "hello"))
					c.Next()
				})
				e.GET("/test", func(c *fox.Context) string { return "ok" })
			},
		},
		{
			// The scenario from the issue: a global metrics/logging middleware
			// registered on the engine reads a value injected by a route-level
			// (group) middleware.
			name:     "global middleware reads group middleware value",
			path:     "/api/test",
			expected: "from-group",
			register: func(e *fox.Engine) {
				api := e.Group("/api", func(c *fox.Context) {
					c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKey{}, "from-group"))
					c.Next()
				})
				api.GET("/test", func(c *fox.Context) string { return "ok" })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := fox.New()
			var outerSeen any

			// Outer middleware: registered first, runs first, reads after c.Next().
			router.Use(func(c *fox.Context) {
				c.Next()
				outerSeen = c.Request.Context().Value(ctxKey{})
			})
			tt.register(router)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expected, outerSeen)
		})
	}
}

// TestRouterGroup_ContextConcurrentWithRequestReplacement verifies that an
// asynchronous goroutine reading the context.Context interface (Done/Value)
// does not race with a synchronous c.Request replacement in the middleware
// chain (run with -race). All context.Context methods must read the immutable
// base snapshot, never the synchronously-mutated Request field.
func TestRouterGroup_ContextConcurrentWithRequestReplacement(t *testing.T) {
	router := fox.New()
	type ctxKey struct{}

	router.Use(func(c *fox.Context) {
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() {
			for {
				select {
				case <-c.Done(): // reads the immutable base snapshot
					return
				case <-stop:
					return
				default:
					_ = c.Value(ctxKey{}) // reads the same immutable snapshot
					time.Sleep(time.Millisecond)
				}
			}
		})
		c.Next()
		close(stop)
		wg.Wait()
	})
	router.Use(func(c *fox.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKey{}, "hello"))
		c.Next()
	})
	router.GET("/test", func(c *fox.Context) string {
		return "ok"
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRouterGroup_PresetLogger tests handler with preset logger in context
func TestRouterGroup_PresetLogger(t *testing.T) {
	router := fox.New()

	// Middleware that sets a custom logger
	router.Use(func(c *fox.Context) {
		// Preset a custom logger in context
		c.Set(fox.LoggerContextKey, c.Logger)
	})

	router.GET("/test", func(c *fox.Context) string {
		// Logger should be available from context
		return "test"
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "test", w.Body.String())
}

// TestRouterGroup_WithTraceID tests handler with existing trace ID
func TestRouterGroup_WithTraceID(t *testing.T) {
	router := fox.New()

	router.GET("/test", func(c *fox.Context) string {
		return "test"
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Pre-set a trace ID in the response writer
	w.Header().Set("X-Request-Id", "preset-trace-id")

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// TestHTTPMethods tests all HTTP method shortcuts
func TestHTTPMethods(t *testing.T) {
	router := fox.New()

	// Test DELETE
	router.DELETE("/delete", func() string {
		return "deleted"
	})

	// Test PUT
	router.PUT("/put", func() string {
		return "updated"
	})

	// Test PATCH
	router.PATCH("/patch", func() string {
		return "patched"
	})

	// Test OPTIONS
	router.OPTIONS("/options", func() string {
		return "options"
	})

	// Test HEAD
	router.HEAD("/head", func() {})

	// Test Any
	router.Any("/any", func() string {
		return "any method"
	})

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{"DELETE method", http.MethodDelete, "/delete", 200, "deleted"},
		{"PUT method", http.MethodPut, "/put", 200, "updated"},
		{"PATCH method", http.MethodPatch, "/patch", 200, "patched"},
		{"OPTIONS method", http.MethodOptions, "/options", 200, "options"},
		{"HEAD method", http.MethodHead, "/head", 200, ""},
		{"Any - GET", http.MethodGet, "/any", 200, "any method"},
		{"Any - POST", http.MethodPost, "/any", 200, "any method"},
		{"Any - PUT", http.MethodPut, "/any", 200, "any method"},
		{"Any - DELETE", http.MethodDelete, "/any", 200, "any method"},
		{"Any - PATCH", http.MethodPatch, "/any", 200, "any method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}
