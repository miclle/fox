// Package fox provides a lightweight web framework for building web applications.
package fox

import (
	"embed"
	"io"
	"net/http"
	"os"
	"reflect"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	// DebugMode indicates gin mode is debug.
	DebugMode = gin.DebugMode
	// ReleaseMode indicates gin mode is release.
	ReleaseMode = gin.ReleaseMode
	// TestMode indicates gin mode is test.
	TestMode = gin.TestMode
)

var foxMode = DebugMode

// SetMode sets gin mode according to input string.
func SetMode(value string) {
	gin.SetMode(value)
	foxMode = value
}

// Mode returns current fox mode.
func Mode() string {
	return foxMode
}

// DefaultWriter is the default io.Writer used by Gin for debug output and
// middleware output like Logger() or Recovery().
// Note that both Logger and Recovery provides custom ways to configure their
// output io.Writer.
// To support coloring in Windows use:
//
//	import "github.com/mattn/go-colorable"
//	gin.DefaultWriter = colorable.NewColorableStdout()
var DefaultWriter io.Writer = os.Stdout

// DefaultErrorWriter is the default io.Writer used by Gin to debug errors.
var DefaultErrorWriter io.Writer = os.Stderr

// ErrInvalidHandlerType is the error message for invalid handler type.
var ErrInvalidHandlerType = "invalid handler type: %s\n" +
	"handler signature: %s\n" +
	"Supported handler types:\n" +
	"1. func()\n" +
	"2. func(ctx *Context) T\n" +
	"3. func(ctx *Context) (T, error)\n" +
	"4. func(ctx *Context, args S) T\n" +
	"5. func(ctx *Context, args S) (T, error)\n" +
	"Where:\n" +
	"- S can be struct or map type, S will be auto binding from request body\n" +
	"- T can be any type, T will be auto render to response body\n" +
	"- error can be any type that implements error interface"

// HandlerFunc is a function that can be registered to a route to handle HTTP requests.
// Like http.HandlerFunc, but support auto binding and auto render.
//
// Support handler types:
//  1. func(){}
//  2. func(ctx *Context) T { ... }
//  3. func(ctx *Context) (T, error) { ... }
//  4. func(ctx *Context, args S) T { ... }
//  5. func(ctx *Context, args S) (T, error) { ... }
//
// Where:
//   - S can be struct or map type, S will be auto binding from request body
//   - T can be any type, T will be auto render to response body
//   - error can be any type that implements error interface
type HandlerFunc any

// HandlersChain defines a HandlerFunc slice.
type HandlersChain []HandlerFunc

var Recovery = gin.Recovery

// Last returns the last handler in the chain. i.e. the last handler is the main one.
func (c HandlersChain) Last() HandlerFunc {
	if length := len(c); length > 0 {
		return c[length-1]
	}
	return nil
}

type RenderErrorFunc func(ctx *Context, err error)

// Engine for server.
type Engine struct {
	*gin.Engine

	RouterGroup

	// DefaultRenderErrorStatusCode is the default http status code used for automatic rendering
	DefaultRenderErrorStatusCode int

	RenderErrorFunc RenderErrorFunc

	handlerRoutesMu       sync.RWMutex
	handlerRoutes         map[handlerRouteKey]RouteInfo
	handlerRoutesDisabled bool
}

// DisableRouteRegistry stops collecting handler reflection metadata for new
// routes. Existing entries are dropped. Use when you do not run any tooling
// (such as openapi generation) and want to free the per-route memory.
func (engine *Engine) DisableRouteRegistry() {
	engine.handlerRoutesMu.Lock()
	defer engine.handlerRoutesMu.Unlock()
	engine.handlerRoutesDisabled = true
	engine.handlerRoutes = nil
}

func init() {
	// Change gin default validator.
	binding.Validator = new(DefaultValidator)
}

// New return engine instance.
func New() *Engine {
	engine := &Engine{
		Engine:                       gin.New(),
		DefaultRenderErrorStatusCode: http.StatusBadRequest,
	}

	// recommend default use context.Context to store request-scoped values
	engine.ContextWithFallback = true

	engine.router = &engine.Engine.RouterGroup
	engine.engine = engine

	return engine
}

// Default return an Engine instance with Logger and Recovery middleware already attached.
func Default() *Engine {
	engine := New()
	engine.Use(NewXResponseTimer(), Logger(), Recovery())
	return engine
}

// Use middleware.
func (engine *Engine) Use(middleware ...HandlerFunc) {
	engine.RouterGroup.Use(middleware...)
}

// NotFound adds handlers for NoRoute. It returns a 404 code by default.
func (engine *Engine) NotFound(handlers ...HandlerFunc) {
	handlersChain := engine.handleWrapper(handlers...)
	engine.Engine.NoRoute(handlersChain...)
}

func (engine *Engine) NoRoute(handlers ...HandlerFunc) {
	handlersChain := engine.handleWrapper(handlers...)
	engine.Engine.NoRoute(handlersChain...)
}

func (engine *Engine) NoMethod(handlers ...HandlerFunc) {
	handlersChain := engine.handleWrapper(handlers...)
	engine.Engine.NoMethod(handlersChain...)
}

// CORS config.
func (engine *Engine) CORS(config cors.Config) {
	if config.Validate() == nil {
		engine.Engine.Use(cors.New(config))
	}
}

// RouterConfigFunc engine load router config func.
type RouterConfigFunc func(router *Engine, embedFS ...embed.FS)

// Load router config.
func (engine *Engine) Load(f RouterConfigFunc, fs ...embed.FS) {
	f(engine, fs...)
}

// IsValidHandlerFunc checks if the handler matches the HandlerFunc type requirements.
func IsValidHandlerFunc(handler HandlerFunc) bool {
	handlerType := reflect.TypeOf(handler)

	// Check if it's a function typ
	if handlerType.Kind() != reflect.Func {
		return false
	}

	// Check number of parameters
	numIn := handlerType.NumIn()
	if numIn > 2 {
		return false
	}

	// Check number of return values
	numOut := handlerType.NumOut()
	if numOut > 2 {
		return false
	}

	// Check if first parameter is *Context
	if numIn > 0 {
		firstParam := handlerType.In(0)
		if firstParam.Kind() != reflect.Ptr || firstParam.Elem().Name() != "Context" {
			return false
		}
	}

	// Check if second parameter is struct or map type
	if numIn > 1 {
		secondParam := handlerType.In(1)
		// If it's a pointer type, get the type it points to
		if secondParam.Kind() == reflect.Ptr {
			secondParam = secondParam.Elem()
		}
		// Check if it's a struct or map type
		if secondParam.Kind() != reflect.Struct && secondParam.Kind() != reflect.Map && secondParam.Kind() != reflect.Interface {
			return false
		}
	}

	// Check return value types
	// First return value can be any type
	if numOut > 1 {
		// Second return value must implement error interface
		secondReturn := handlerType.Out(1)
		if !secondReturn.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return false
		}
	}

	return true
}
