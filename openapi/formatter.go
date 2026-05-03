package openapi

import (
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
)

// RegisterFormatter overrides schema generation for a Go type.
func RegisterFormatter(typ reflect.Type, schema *openapi3.Schema) Option {
	return func(g *Generator) {
		g.formatters[deref(typ)] = schema
	}
}

func cloneSchema(schema *openapi3.Schema) *openapi3.Schema {
	if schema == nil {
		return nil
	}
	clone := *schema
	return &clone
}
