package fox

import (
	"reflect"
	"runtime"
	"sort"
)

type openAPIRouteKey struct {
	Method string
	Path   string
}

// OpenAPIRouteInfo preserves the original fox handler metadata for OpenAPI
// generation. Gin only exposes the wrapped handler, so fox records the business
// handler at route registration time.
type OpenAPIRouteInfo struct {
	Method      string
	Path        string
	Handler     HandlerFunc
	HandlerType reflect.Type
	HandlerName string
}

func (engine *Engine) registerOpenAPIRoute(method, path string, handlers HandlersChain) {
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

	engine.openAPIRoutesMu.Lock()
	defer engine.openAPIRoutesMu.Unlock()

	if engine.openAPIRoutes == nil {
		engine.openAPIRoutes = make(map[openAPIRouteKey]OpenAPIRouteInfo)
	}

	engine.openAPIRoutes[openAPIRouteKey{Method: method, Path: path}] = OpenAPIRouteInfo{
		Method:      method,
		Path:        path,
		Handler:     handler,
		HandlerType: reflect.TypeOf(handler),
		HandlerName: funcName,
	}
}

// OpenAPIRoutes returns a stable snapshot of routes registered through fox.
func (engine *Engine) OpenAPIRoutes() []OpenAPIRouteInfo {
	engine.openAPIRoutesMu.RLock()
	defer engine.openAPIRoutesMu.RUnlock()

	routes := make([]OpenAPIRouteInfo, 0, len(engine.openAPIRoutes))
	for _, route := range engine.openAPIRoutes {
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
