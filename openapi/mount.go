package openapi

import (
	"github.com/gin-gonic/gin"

	"github.com/fox-gonic/fox"
)

// Router is the minimal surface Mount needs — both *fox.Engine and
// *fox.RouterGroup satisfy it.
type Router interface {
	GET(relativePath string, handlers ...fox.HandlerFunc) gin.IRoutes
}

type mountConfig struct {
	yamlPath string
	jsonPath string
}

// MountOption configures OpenAPI spec endpoint mounting.
type MountOption func(*mountConfig)

// MountYAML sets the YAML spec endpoint path. Empty string disables.
func MountYAML(path string) MountOption {
	return func(config *mountConfig) {
		config.yamlPath = path
	}
}

// MountJSON sets the JSON spec endpoint path. Empty string disables.
func MountJSON(path string) MountOption {
	return func(config *mountConfig) {
		config.jsonPath = path
	}
}

// Mount registers YAML and JSON spec endpoints on the router.
//
// router accepts *fox.Engine or *fox.RouterGroup. Mount forces spec generation
// immediately so the spec endpoints themselves are not included in the
// generated paths. Routes registered after Mount are not picked up unless the
// caller invokes Generator.Regenerate().
func Mount(router Router, g *Generator, opts ...MountOption) {
	config := mountConfig{
		yamlPath: "/openapi.yaml",
		jsonPath: "/openapi.json",
	}
	for _, opt := range opts {
		opt(&config)
	}

	g.ensureGenerated()

	if config.yamlPath != "" {
		router.GET(config.yamlPath, YAMLHandler(g))
	}
	if config.jsonPath != "" {
		router.GET(config.jsonPath, JSONHandler(g))
	}
}
