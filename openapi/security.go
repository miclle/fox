package openapi

import "github.com/getkin/kin-openapi/openapi3"

// SecurityScheme registers an OpenAPI security scheme.
func SecurityScheme(name string, scheme *openapi3.SecurityScheme) Option {
	return func(g *Generator) {
		if g.spec.Components.SecuritySchemes == nil {
			g.spec.Components.SecuritySchemes = openapi3.SecuritySchemes{}
		}
		g.spec.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{Value: scheme}
	}
}

// HTTPBearerSecurity creates an HTTP bearer security scheme.
func HTTPBearerSecurity(description string) *openapi3.SecurityScheme {
	return &openapi3.SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  description,
	}
}
