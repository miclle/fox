package fox

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func registeredRouteHandler(_ *Context) string {
	return "ok"
}

func TestHandlerRoutesReturnsOriginalHandlerMetadata(t *testing.T) {
	engine := New()
	engine.GET("/health", registeredRouteHandler)

	routes := engine.HandlerRoutes()

	require.Len(t, routes, 1)
	require.Equal(t, "GET", routes[0].Method)
	require.Equal(t, "/health", routes[0].Path)
	require.Equal(t, reflect.TypeOf(registeredRouteHandler), routes[0].HandlerType)
	require.Contains(t, routes[0].HandlerName, "registeredRouteHandler")
}
