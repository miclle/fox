package fox

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"

	"github.com/fox-gonic/fox/logger"
	"github.com/fox-gonic/fox/utils"
)

// anyMethods for RouterGroup Any method.
var anyMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodConnect,
	http.MethodTrace,
}

// RouterGroup is gin.RouterGroup wrapper.
type RouterGroup struct {
	router *gin.RouterGroup
	engine *Engine
}

// handleWrapper gin.Handle wrapper.
func (group *RouterGroup) handleWrapper(handlers ...HandlerFunc) gin.HandlersChain {
	var handlersChain gin.HandlersChain

	for _, handler := range handlers {
		if !IsValidHandlerFunc(handler) {
			panic(fmt.Sprintf(ErrInvalidHandlerType, reflect.TypeOf(handler).String(), utils.NameOfFunction(handler)))
		}

		f := func(h HandlerFunc) gin.HandlerFunc {
			// support use gin middleware
			if ginHandler, ok := h.(gin.HandlerFunc); ok {
				return ginHandler
			}

			return func(c *gin.Context) {
				xRequestID := c.Writer.Header().Get(logger.TraceID)
				if xRequestID == "" {
					xRequestID = logger.DefaultGenRequestID()
					c.Header(logger.TraceID, xRequestID)
				}

				c.Set(logger.TraceID, xRequestID)

				var log logger.Logger
				if v, exists := c.Get(LoggerContextKey); exists {
					log = v.(logger.Logger)
				} else {
					log = logger.New(xRequestID)
				}

				var (
					ctx = &Context{
						Context: c,
						engine:  group.engine,
						Logger:  log,
						Request: c.Request,
					}
					res = call(ctx, h)
				)
				// The Context.Request may be changed in middleware,
				// so we need to update the gin.Context.Request at here
				c.Request = ctx.Request

				if ctx.IsAborted() {
					return
				}

				ctx.render(res)
			}
		}

		handlersChain = append(handlersChain, f(handler))
	}

	// GIN handle
	return handlersChain
}

// --------------------------------------------------------------------

// Use adds middleware to the group, see example code in GitHub.
func (group *RouterGroup) Use(middleware ...HandlerFunc) gin.IRoutes {
	handlersChain := group.handleWrapper(middleware...)
	return group.router.Use(handlersChain...)
}

// Group creates a new router group. You should add all the routes that have common middlewares or the same path prefix.
// For example, all the routes that use a common middleware for authorization could be grouped.
func (group *RouterGroup) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	handlersChain := group.handleWrapper(handlers...)
	return &RouterGroup{
		router: group.router.Group(relativePath, handlersChain...),
		engine: group.engine,
	}
}

// Handle gin.Handle wrapper.
func (group *RouterGroup) Handle(httpMethod, relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	handlersChain := group.handleWrapper(handlers...)

	absolutePath := utils.JoinPaths(group.router.BasePath(), relativePath)
	debugPrintRoute(group, httpMethod, absolutePath, handlers)
	group.engine.registerHandlerRoute(httpMethod, absolutePath, handlers)
	return group.router.Handle(httpMethod, relativePath, handlersChain...)
}

// GET is a shortcut for router.Handle("GET", path, handle).
func (group *RouterGroup) GET(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return group.Handle(http.MethodGet, relativePath, handlers...)
}

// POST is a shortcut for router.Handle("POST", path, handle).
func (group *RouterGroup) POST(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return group.Handle(http.MethodPost, relativePath, handlers...)
}

// DELETE is a shortcut for router.Handle("DELETE", path, handle).
func (group *RouterGroup) DELETE(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return group.Handle(http.MethodDelete, relativePath, handlers...)
}

// PATCH is a shortcut for router.Handle("PATCH", path, handle).
func (group *RouterGroup) PATCH(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return group.Handle(http.MethodPatch, relativePath, handlers...)
}

// PUT is a shortcut for router.Handle("PUT", path, handle).
func (group *RouterGroup) PUT(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return group.Handle(http.MethodPut, relativePath, handlers...)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, handle).
func (group *RouterGroup) OPTIONS(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return group.Handle(http.MethodOptions, relativePath, handlers...)
}

// HEAD is a shortcut for router.Handle("HEAD", path, handle).
func (group *RouterGroup) HEAD(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return group.Handle(http.MethodHead, relativePath, handlers...)
}

// Any registers a route that matches all the HTTP methods.
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
func (group *RouterGroup) Any(relativePath string, handlers ...HandlerFunc) {
	for _, method := range anyMethods {
		group.Handle(method, relativePath, handlers...)
	}
}
