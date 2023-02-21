package engine

import (
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"

	"github.com/fox-gonic/fox/errors"
	"github.com/fox-gonic/fox/logger"
	"github.com/fox-gonic/fox/utils"
)

// RouterGroup is gin.RouterGroup wrapper
type RouterGroup struct {
	router *gin.RouterGroup
}

// handleWrapper gin.Handle wrapper
func (group *RouterGroup) handleWrapper(handlers ...HandlerFunc) gin.HandlersChain {

	var handlersChain gin.HandlersChain

	for _, handler := range handlers {

		if reflect.TypeOf(handler).Kind() != reflect.Func {
			panic("handler must be a callable function")
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
					handleName = utils.NameOfFunction(h)
					start      = time.Now()
					context    = &Context{Context: c, Logger: log}
					res, err   = call(context, h)
					latency    = time.Until(start).String()
				)

				fields := map[string]interface{}{
					"latency": latency,
					"type":    "HANDLER",
				}

				c.Header("latency", latency)

				context.Logger.WithFields(fields).Info(handleName)

				if context.Context.IsAborted() {
					return
				}

				// output parameter processing
				if err != nil {
					res = errors.Wrap(err)
				}

				switch r := res.(type) {
				case *errors.Error:
					c.AbortWithStatusJSON(r.HTTPCode, r)
					return
				case error:
					if e, ok := r.(errors.StatusCoder); ok {
						c.AbortWithStatusJSON(e.StatusCode(), r)
						return
					}
					c.AbortWithStatusJSON(400, errors.Wrap(r))
					return
				case string:
					c.String(200, r)
					c.Abort()
					return
				case render.Redirect:
					c.Redirect(r.Code, r.Location)
					c.Abort()
					return
				case render.YAML:
					c.YAML(http.StatusOK, r.Data)
					c.Abort()
					return
				case render.XML:
					c.XML(http.StatusOK, r.Data)
					c.Abort()
					return
				case render.HTML:
					c.Render(http.StatusOK, r)
					c.Abort()
					return
				case render.Reader:
					c.Render(http.StatusOK, r)
					c.Abort()
					return
				case nil:
					// nothing to do
					return
				default:
					c.AbortWithStatusJSON(http.StatusOK, r)
					return
				}
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
	}
}

// Handle gin.Handle wrapper
func (group *RouterGroup) Handle(httpMethod, relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	handlersChain := group.handleWrapper(handlers...)

	absolutePath := utils.JoinPaths(group.router.BasePath(), relativePath)
	debugPrintRoute(group, httpMethod, absolutePath, handlers)
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
