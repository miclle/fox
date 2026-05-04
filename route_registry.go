package fox

import (
	"reflect"
	"runtime"
	"sort"
)

type handlerRouteKey struct {
	Method string
	Path   string
}

// RouteInfo preserves the original fox handler metadata for registered routes.
// Gin only exposes the wrapped handler, so fox records the business handler at
// route registration time for external tooling such as documentation generators.
type RouteInfo struct {
	Method      string
	Path        string
	Handler     HandlerFunc
	HandlerType reflect.Type
	HandlerName string
}

func (engine *Engine) registerHandlerRoute(method, path string, handlers HandlersChain) {
	handler := handlers.Last()
	if handler == nil {
		return
	}

	funcValue := reflect.ValueOf(handler)
	funcName := ""
	if funcValue.IsValid() && funcValue.Kind() == reflect.Func {
		if fn := runtime.FuncForPC(funcValue.Pointer()); fn != nil {
			funcName = fn.Name()
		}
	}

	engine.handlerRoutesMu.Lock()
	defer engine.handlerRoutesMu.Unlock()

	if engine.handlerRoutesDisabled {
		return
	}

	if engine.handlerRoutes == nil {
		engine.handlerRoutes = make(map[handlerRouteKey]RouteInfo)
	}

	engine.handlerRoutes[handlerRouteKey{Method: method, Path: path}] = RouteInfo{
		Method:      method,
		Path:        path,
		Handler:     handler,
		HandlerType: reflect.TypeOf(handler),
		HandlerName: funcName,
	}
}

// HandlerRoutes returns a stable snapshot of routes registered through fox.
func (engine *Engine) HandlerRoutes() []RouteInfo {
	engine.handlerRoutesMu.RLock()
	defer engine.handlerRoutesMu.RUnlock()

	routes := make([]RouteInfo, 0, len(engine.handlerRoutes))
	for _, route := range engine.handlerRoutes {
		routes = append(routes, route)
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	return routes
}
