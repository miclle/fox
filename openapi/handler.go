package openapi

import (
	"net/http"

	"github.com/fox-gonic/fox"
)

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
