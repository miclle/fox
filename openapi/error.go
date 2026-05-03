package openapi

import "reflect"

// SetErrorSchema overrides the default error response schema.
func SetErrorSchema(body any) Option {
	return func(g *Generator) {
		if body == nil {
			return
		}
		g.errorSchema = reflect.TypeOf(body)
	}
}
