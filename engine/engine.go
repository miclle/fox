package engine

import (
	"embed"
	"io"
	"os"

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

// DefaultWriter is the default io.Writer used by Gin for debug output and
// middleware output like Logger() or Recovery().
// Note that both Logger and Recovery provides custom ways to configure their
// output io.Writer.
// To support coloring in Windows use:
//
//	import "github.com/mattn/go-colorable"
//	gin.DefaultWriter = colorable.NewColorableStdout()
var DefaultWriter io.Writer = os.Stdout

// DefaultErrorWriter is the default io.Writer used by Gin to debug errors
var DefaultErrorWriter io.Writer = os.Stderr

// HandlerFunc is a function that can be registered to a route to handle HTTP
// requests. Like http.HandlerFunc, but has a third parameter for the values of
// wildcards (path variables).
// func(){}
// func(ctx *Context) any { ... }
// func(ctx *Context) (any, err) { ... }
// func(ctx *Context, args *AutoBindingArgType) (any) { ... }
// func(ctx *Context, args *AutoBindingArgType) (any, err) { ... }
type HandlerFunc interface{}

// HandlersChain defines a HandlerFunc slice.
type HandlersChain []HandlerFunc

// Last returns the last handler in the chain. i.e. the last handler is the main one.
func (c HandlersChain) Last() HandlerFunc {
	if length := len(c); length > 0 {
		return c[length-1]
	}
	return nil
}

// Engine for server
type Engine struct {
	*gin.Engine

	RouterGroup
}

// New return engine instance
func New() *Engine {

	// Change gin default validator
	binding.Validator = new(DefaultValidator)

	router := gin.New()
	router.Use(Logger(), gin.Recovery())

	engine := &Engine{}
	engine.Engine = router
	engine.RouterGroup.router = &engine.Engine.RouterGroup

	return engine
}

// Use middleware
func (engine *Engine) Use(middleware ...HandlerFunc) {
	engine.RouterGroup.Use(middleware...)
}

// CORS config
func (engine *Engine) CORS(config cors.Config) {
	if config.Validate() == nil {
		engine.Engine.Use(cors.New(config))
	}
}

// RouterConfigFunc engine load router config func
type RouterConfigFunc func(router *Engine, embedFS ...embed.FS)

// Load router config
func (engine *Engine) Load(f RouterConfigFunc, fs ...embed.FS) {
	f(engine, fs...)
}