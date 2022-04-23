package fox

import (
	"fmt"
	"net/http"
	"path"
	"sync"

	"github.com/miclle/fox/internal/bytesconv"
)

var (
	default404Body = []byte("404 page not found")
	default405Body = []byte("405 method not allowed")
)

// HandlerFunc is a function that can be registered to a route to handle HTTP
// requests. Like http.HandlerFunc, but has a third parameter for the values of
// wildcards (path variables).
// func(){}
// func(ctx *Context) any { ... }
// func(ctx *Context) (any, err) { ... }
// func(ctx *Context, args *AutoBindingArgType) (any, err) { ... }
// func(ctx *Context, args *AutoBindingArgType) (any, code, err) { ... }
// func(ctx *Context, args *AutoBindingArgType) (any, code) { ... }
type HandlerFunc interface{}

// HandlersChain defines a HandlerFunc slice.
type HandlersChain []HandlerFunc

// Engine is a http.Handler which can be used to dispatch requests to different
// handler functions via configurable routes
type Engine struct {
	RouterGroup

	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 308 for all other request methods.
	RedirectTrailingSlash bool

	// If enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterwards the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 308 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool

	// If enabled, the router checks if another method is allowed for the
	// current route, if the current request can not be routed.
	// If this is the case, the request is answered with 'Method Not Allowed'
	// and HTTP status code 405.
	// If no other Method is allowed, the request is delegated to the NotFound
	// handler.
	HandleMethodNotAllowed bool

	// If enabled, the router automatically replies to OPTIONS requests.
	// Custom OPTIONS handlers take priority over automatic replies.
	HandleOPTIONS bool

	// An optional http.Handler that is called on automatic OPTIONS requests.
	// The handler is only called if HandleOPTIONS is true and no OPTIONS
	// handler for the specific path was set.
	// The "Allowed" header is set before calling the handler.
	GlobalOPTIONS http.Handler

	trees methodTrees

	paramsPool sync.Pool

	pool        sync.Pool // pool of contexts that are used in a request
	maxParams   uint16
	maxSections uint16

	// Configurable http.Handler which is called when no matching route is
	// found. If it is not set, http.NotFound is used.
	noRoute    HandlersChain
	allNoRoute HandlersChain

	// Configurable http.Handler which is called when a request
	// cannot be routed and HandleMethodNotAllowed is true.
	// If it is not set, http.Error with http.StatusMethodNotAllowed is used.
	// The "Allow" header with allowed request methods is set before the handler
	// is called.
	noMethod    HandlersChain
	allNoMethod HandlersChain

	// Function to handle panics recovered from http handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics.
	PanicHandler func(http.ResponseWriter, *http.Request, interface{})

	// cache is a key/value pair global for the engine.
	cache sync.Map
}

// Make sure the Router conforms with the http.Handler interface
var _ http.Handler = New()

// New returns a new initialized Router.
// Path auto-correction, including trailing slashes, is enabled by default.
func New() *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{
			Handlers: nil,
			basePath: "/",
			root:     true,
		},
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}
	engine.RouterGroup.engine = engine
	engine.pool.New = func() any {
		return engine.allocateContext()
	}
	return engine
}

func (engine *Engine) allocateContext() *Context {
	params := make(Params, 0, engine.maxParams)
	skippedNodes := make([]skippedNode, 0, engine.maxSections)
	return &Context{engine: engine, Params: &params, skippedNodes: &skippedNodes}
}

// Store sets the value for a key.
func (engine *Engine) Store(key string, value any) {
	engine.cache.Store(key, value)
}

// Load returns the value stored in the map for a key, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (engine *Engine) Load(key string) (value any, exists bool) {
	return engine.cache.Load(key)
}

// MustLoad returns the value for the given key if it exists, otherwise it panics.
func (engine *Engine) MustLoad(key string) any {
	if value, exists := engine.cache.Load(key); exists {
		return value
	}
	panic("Key \"" + key + "\" does not exist")
}

// Use attaches a global middleware to the router. i.e. the middleware attached through Use() will be
// included in the handlers chain for every single request. Even 404, 405, static files...
// For example, this is the right place for a logger or error management middleware.
func (engine *Engine) Use(middleware ...HandlerFunc) {
	engine.RouterGroup.Use(middleware...)
	engine.allNoRoute = engine.combineHandlers(engine.noRoute)
	engine.allNoMethod = engine.combineHandlers(engine.noMethod)
}

// NotFound configurable http.Handler which is called when no matching route is
// found. If it is not set, http.NotFound is used.
func (engine *Engine) NotFound(handlers ...HandlerFunc) {
	engine.noRoute = handlers
	engine.allNoRoute = engine.combineHandlers(engine.noRoute)
}

// NoMethod sets the handlers called when Engine.HandleMethodNotAllowed = true.
func (engine *Engine) NoMethod(handlers ...HandlerFunc) {
	engine.noMethod = handlers
	engine.allNoMethod = engine.combineHandlers(engine.noMethod)
}

func (engine *Engine) addRoute(method, path string, handlers HandlersChain) {
	assert1(path[0] == '/', "path must begin with '/'")
	assert1(method != "", "HTTP method can not be empty")
	assert1(len(handlers) > 0, "there must be at least one handler")

	for _, handler := range handlers {
		if handler == nil {
			panic("handler can not be nil")
		}
	}

	root := engine.trees.get(method)
	if root == nil {
		root = new(node)
		root.fullPath = "/"
		engine.trees = append(engine.trees, methodTree{method: method, root: root})
	}
	root.addRoute(path, handlers)

	// Update maxParams
	if paramsCount := countParams(path); paramsCount > engine.maxParams {
		engine.maxParams = paramsCount
	}

	if sectionsCount := countSections(path); sectionsCount > engine.maxSections {
		engine.maxSections = sectionsCount
	}
}

func (engine *Engine) recv(w http.ResponseWriter, req *http.Request) {
	if rcv := recover(); rcv != nil {
		engine.PanicHandler(w, req, rcv)
	}
}

// Run attaches the router to a http.Server and starts listening and serving HTTP requests.
// It is a shortcut for http.ListenAndServe(addr, router)
// Note: this method will block the calling goroutine indefinitely unless an error happens.
func (engine *Engine) Run(addr string) (err error) {
	defer func() {
		if err != nil {
			fmt.Fprintf(DefaultErrorWriter, "[ERROR] %v\n", err)
		}
	}()

	err = http.ListenAndServe(addr, engine)
	return
}

// ServeHTTP makes the router implement the http.Handler interface.
func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := engine.pool.Get().(*Context)
	ctx.reset(w, req)
	engine.handleHTTPRequest(ctx)
	engine.pool.Put(ctx)
}

func (engine *Engine) handleHTTPRequest(ctx *Context) {
	if engine.PanicHandler != nil {
		defer engine.recv(ctx.Writer, ctx.Request)
	}

	httpMethod := ctx.Request.Method
	path := ctx.Request.URL.Path
	unescape := false

	// Find root of the tree for the given HTTP method
	t := engine.trees
	for i, tl := 0, len(t); i < tl; i++ {
		if t[i].method != httpMethod {
			continue
		}
		root := t[i].root
		// Find route in tree
		value := root.getValue(path, ctx.Params, ctx.skippedNodes, unescape)
		if value.params != nil {
			ctx.Params = value.params
		}
		if value.handlers != nil {
			ctx.handlers = value.handlers
			ctx.fullPath = value.fullPath
			ctx.Next()
			ctx.Writer.WriteHeaderNow()
			return
		}
		if httpMethod != http.MethodConnect && path != "/" {
			if value.tsr && engine.RedirectTrailingSlash {
				redirectTrailingSlash(ctx)
				return
			}
			if engine.RedirectFixedPath && redirectFixedPath(ctx, root, engine.RedirectFixedPath) {
				return
			}
		}
		break
	}

	// Handle 405
	if engine.HandleMethodNotAllowed {
		for _, tree := range engine.trees {
			if tree.method == httpMethod {
				continue
			}
			if value := tree.root.getValue(path, nil, ctx.skippedNodes, unescape); value.handlers != nil {
				ctx.handlers = engine.allNoMethod
				serveError(ctx, http.StatusMethodNotAllowed, default405Body)
				return
			}
		}
	}
	ctx.handlers = engine.allNoRoute
	serveError(ctx, http.StatusNotFound, default404Body)
}

var mimePlain = []string{"text/plain"}

func serveError(c *Context, code int, defaultMessage []byte) {
	c.Writer.status = code
	c.Next()
	if c.Writer.Written() {
		return
	}
	if c.Writer.Status() == code {
		c.Writer.Header()["Content-Type"] = mimePlain
		_, err := c.Writer.Write(defaultMessage)
		if err != nil {
			// debugPrint("cannot write message to writer during serve error: %v", err)
		}
		return
	}
	c.Writer.WriteHeaderNow()
}

func redirectTrailingSlash(ctx *Context) {
	req := ctx.Request
	p := req.URL.Path
	if prefix := path.Clean(ctx.Request.Header.Get("X-Forwarded-Prefix")); prefix != "." {
		p = prefix + "/" + req.URL.Path
	}
	req.URL.Path = p + "/"
	if length := len(p); length > 1 && p[length-1] == '/' {
		req.URL.Path = p[:length-1]
	}
	redirectRequest(ctx)
}

func redirectFixedPath(ctx *Context, root *node, trailingSlash bool) bool {
	req := ctx.Request
	rPath := req.URL.Path

	if fixedPath, ok := root.findCaseInsensitivePath(CleanPath(rPath), trailingSlash); ok {
		req.URL.Path = bytesconv.BytesToString(fixedPath)
		redirectRequest(ctx)
		return true
	}
	return false
}

func redirectRequest(ctx *Context) {
	req := ctx.Request
	// rPath := req.URL.Path
	rURL := req.URL.String()

	code := http.StatusMovedPermanently // Permanent redirect, request with GET method
	if req.Method != http.MethodGet {
		code = http.StatusPermanentRedirect
	}
	// debugPrint("redirecting request %d: %s --> %s", code, rPath, rURL)
	http.Redirect(ctx.Writer, req, rURL, code)
	ctx.Writer.WriteHeaderNow()
}
