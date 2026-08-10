package fox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fox-gonic/fox/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test RequestBody method

func TestContext_RequestBody_FirstRead(t *testing.T) {
	engine := New()
	body := "test request body"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	// First read should read from request body
	result, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Equal(t, body, string(result))

	// Body should be cached
	cachedBody, exists := ctx.Get(gin.BodyBytesKey)
	require.True(t, exists)
	assert.Equal(t, []byte(body), cachedBody)
}

func TestContext_RequestBody_CachedRead(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
	}

	// Pre-set cached body
	cachedBody := []byte("cached body")
	ctx.Set(gin.BodyBytesKey, cachedBody)

	// Should return cached body
	result, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Equal(t, cachedBody, result)
}

func TestContext_RequestBody_CachedValueNotByteSlice(t *testing.T) {
	engine := New()
	body := "test body"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	// Set cached value that is not []byte
	ctx.Set(gin.BodyBytesKey, "not a byte slice")

	// Should read from request body since cached value is not []byte
	result, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Equal(t, body, string(result))
}

func TestContext_RequestBody_MultipleReads(t *testing.T) {
	engine := New()
	body := "test body"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	// First read
	result1, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Equal(t, body, string(result1))

	// Second read should return cached body
	result2, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Equal(t, body, string(result2))

	// Both should be equal
	assert.Equal(t, result1, result2)
}

func TestContext_RequestBody_EmptyBody(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	result, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Equal(t, []byte{}, result)
}

func TestContext_RequestBody_NilRequest(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: nil,
	}

	result, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestContext_RequestBody_NilBody(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	result, err := ctx.RequestBody()
	require.NoError(t, err)
	// When body is nil, result can be nil or empty slice
	assert.Empty(t, result)
}

func TestContext_RequestBody_LargeBody(t *testing.T) {
	engine := New()
	largeBody := strings.Repeat("a", 10000)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	result, err := ctx.RequestBody()
	require.NoError(t, err)
	assert.Equal(t, largeBody, string(result))
	assert.Len(t, result, 10000)
}

// Test TraceID method

func TestContext_TraceID_FromContext(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
	}

	// Set TraceID in context
	expectedID := "context-trace-123"
	ctx.Set(logger.TraceID, expectedID)

	traceID := ctx.TraceID()
	assert.Equal(t, expectedID, traceID)
}

func TestContext_TraceID_FromRequestHeader(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(logger.TraceID, "header-trace-456")
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	traceID := ctx.TraceID()

	// TraceID should read from request header
	assert.Equal(t, "header-trace-456", traceID)
}

func TestContext_TraceID_FromResponseHeader(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	w.Header().Set(logger.TraceID, "response-trace-789")

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	traceID := ctx.TraceID()
	assert.Equal(t, "response-trace-789", traceID)
}

func TestContext_TraceID_Generated(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	traceID := ctx.TraceID()

	// Should generate a new ID
	assert.NotEmpty(t, traceID)
	assert.Len(t, traceID, 16) // Default ID length

	// Should be set in context and response header
	storedID, exists := ctx.Get(logger.TraceID)
	assert.True(t, exists)
	assert.Equal(t, traceID, storedID)
	assert.Equal(t, traceID, w.Header().Get(logger.TraceID))
}

func TestContext_TraceID_Consistency(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	// Multiple calls should return the same ID
	id1 := ctx.TraceID()
	id2 := ctx.TraceID()
	id3 := ctx.TraceID()

	assert.Equal(t, id1, id2)
	assert.Equal(t, id2, id3)
}

// Test context.Context interface methods

func TestContext_Done(t *testing.T) {
	engine := New()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	foxCtx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	// Channel should not be closed
	select {
	case <-foxCtx.Done():
		t.Fatal("Done channel should not be closed")
	default:
		// Expected
	}

	// Cancel context
	cancel()

	// Channel should be closed now
	select {
	case <-foxCtx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after cancel")
	}
}

func TestContext_Err(t *testing.T) {
	engine := New()

	t.Run("no error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(w)
		ginCtx.Request = req

		ctx := &Context{
			Context: ginCtx,
			engine:  engine,
			Request: req,
		}

		assert.NoError(t, ctx.Err())
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(w)
		ginCtx.Request = req

		foxCtx := &Context{
			Context: ginCtx,
			engine:  engine,
			Request: req,
		}

		assert.Equal(t, context.Canceled, foxCtx.Err())
	})

	t.Run("deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(w)
		ginCtx.Request = req

		foxCtx := &Context{
			Context: ginCtx,
			engine:  engine,
			Request: req,
		}

		assert.Equal(t, context.DeadlineExceeded, foxCtx.Err())
	})
}

func TestContext_Value(t *testing.T) {
	engine := New()

	type testKey string
	key := testKey("test-key")
	value := "test-value"

	ctx := context.WithValue(context.Background(), key, value)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	foxCtx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	result := foxCtx.Value(key)
	assert.Equal(t, value, result)

	// Non-existent key should return nil
	assert.Nil(t, foxCtx.Value(testKey("non-existent")))
}

func TestContext_Deadline(t *testing.T) {
	engine := New()

	t.Run("with deadline", func(t *testing.T) {
		deadline := time.Now().Add(1 * time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(w)
		ginCtx.Request = req

		foxCtx := &Context{
			Context: ginCtx,
			engine:  engine,
			Request: req,
		}

		dl, ok := foxCtx.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, deadline, dl, 1*time.Second)
	})

	t.Run("without deadline", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(w)
		ginCtx.Request = req

		ctx := &Context{
			Context: ginCtx,
			engine:  engine,
			Request: req,
		}

		_, ok := ctx.Deadline()
		assert.False(t, ok)
	})
}

// Test Next method

func TestContext_Next(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	// Test that Next syncs the request
	ctx.Next()

	// Request should be synced to gin.Context
	assert.Equal(t, req, ginCtx.Request)
}

// TestContext_RequestReplacementVisibleToOuterMiddleware reproduces
// https://github.com/fox-gonic/fox/issues/79: a route-level middleware that
// replaces c.Request (e.g. c.Request.WithContext(...)) must be visible to the
// outer (earlier-registered) global middleware reading after c.Next().
func TestContext_RequestReplacementVisibleToOuterMiddleware(t *testing.T) {
	type ctxKey struct{}

	router := New()
	var outerSeen, handlerSeen any

	// Global middleware (registered first, runs first): reads the value injected
	// by the inner route-level middleware after the chain finishes.
	router.Use(func(c *Context) {
		c.Next()
		outerSeen = c.Request.Context().Value(ctxKey{})
	})

	// Route-level middleware replaces c.Request, then the real handler reads it.
	router.GET("/test", func(c *Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKey{}, "hello"))
		c.Next()
	}, func(c *Context) {
		handlerSeen = c.Request.Context().Value(ctxKey{})
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	// handler sees: hello
	assert.Equal(t, "hello", handlerSeen)
	// outer sees: hello (previously <nil>)
	assert.Equal(t, "hello", outerSeen)
}

// Test Copy method

func TestContext_Copy(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	// Create a logger (use default logger)
	testLogger := logger.NewWithoutCaller()

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Logger:  testLogger,
		Request: req,
	}

	// Set some values
	ctx.Set("key1", "value1")
	ctx.Set("key2", 123)

	// Copy context
	copied := ctx.Copy()

	// Verify copied context
	assert.NotNil(t, copied)
	assert.NotSame(t, ctx, copied)
	assert.NotSame(t, ctx.Context, copied.Context)

	// Verify fields are copied
	assert.Equal(t, ctx.engine, copied.engine)
	assert.Equal(t, ctx.Logger, copied.Logger)
	assert.Equal(t, ctx.Request, copied.Request)

	// Verify context values are copied
	val1, exists := copied.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val1)

	val2, exists := copied.Get("key2")
	assert.True(t, exists)
	assert.Equal(t, 123, val2)

	// Verify Request is set correctly
	assert.Equal(t, req, copied.Context.Request)
}

func TestContext_Copy_Independence(t *testing.T) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	ctx.Set("original", "value")

	// Copy context
	copied := ctx.Copy()

	// Modify original
	ctx.Set("new-key", "new-value")

	// Copied should not have new key
	_, exists := copied.Get("new-key")
	assert.False(t, exists)

	// But should still have original key
	val, exists := copied.Get("original")
	assert.True(t, exists)
	assert.Equal(t, "value", val)
}

// TestContext_Copy_NilRequest guards against a regression where Copy()
// panicked on a manually constructed Context with a nil Request.
func TestContext_Copy_NilRequest(t *testing.T) {
	engine := New()
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	// ginCtx.Request is nil here; Copy() must not dereference it.
	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
	}

	assert.NotPanics(t, func() {
		copied := ctx.Copy()
		assert.Nil(t, copied.Request)
	})
}

// Benchmark tests

func BenchmarkContext_RequestBody_FirstRead(b *testing.B) {
	engine := New()
	body := strings.Repeat("test data ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(w)
		ginCtx.Request = req

		ctx := &Context{
			Context: ginCtx,
			engine:  engine,
			Request: req,
		}

		_, _ = ctx.RequestBody()
	}
}

func BenchmarkContext_RequestBody_CachedRead(b *testing.B) {
	engine := New()
	body := []byte(strings.Repeat("test data ", 100))
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
	}

	ctx.Set(gin.BodyBytesKey, body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ctx.RequestBody()
	}
}

func BenchmarkContext_TraceID_Generated(b *testing.B) {
	engine := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(w)
		ginCtx.Request = req

		ctx := &Context{
			Context: ginCtx,
			engine:  engine,
			Request: req,
		}

		_ = ctx.TraceID()
	}
}

func BenchmarkContext_TraceID_Cached(b *testing.B) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Request: req,
	}

	// First call to generate and cache
	_ = ctx.TraceID()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.TraceID()
	}
}

func BenchmarkContext_Copy(b *testing.B) {
	engine := New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	ginCtx.Request = req

	testLogger := logger.NewWithoutCaller()

	ctx := &Context{
		Context: ginCtx,
		engine:  engine,
		Logger:  testLogger,
		Request: req,
	}

	ctx.Set("key1", "value1")
	ctx.Set("key2", 123)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.Copy()
	}
}
