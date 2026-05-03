package openapi

import (
	"net/http"

	"github.com/fox-gonic/fox"
)

// JSONHandler returns a Fox handler that serves the generated JSON spec.
func JSONHandler(g *Generator) fox.HandlerFunc {
	return func(ctx *fox.Context) {
		data, err := g.JSON()
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.Data(http.StatusOK, "application/json; charset=utf-8", data)
	}
}

// YAMLHandler returns a Fox handler that serves the generated YAML spec.
func YAMLHandler(g *Generator) fox.HandlerFunc {
	return func(ctx *fox.Context) {
		data, err := g.YAML()
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.Data(http.StatusOK, "application/yaml; charset=utf-8", data)
	}
}
