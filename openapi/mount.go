package openapi

import "github.com/fox-gonic/fox"

type mountConfig struct {
	yamlPath string
	jsonPath string
}

// MountOption configures OpenAPI spec endpoint mounting.
type MountOption func(*mountConfig)

// MountYAML sets the YAML spec endpoint path.
func MountYAML(path string) MountOption {
	return func(config *mountConfig) {
		config.yamlPath = path
	}
}

// MountJSON sets the JSON spec endpoint path.
func MountJSON(path string) MountOption {
	return func(config *mountConfig) {
		config.jsonPath = path
	}
}

// Mount registers YAML and JSON spec endpoints on the engine.
func Mount(engine *fox.Engine, g *Generator, opts ...MountOption) {
	config := mountConfig{
		yamlPath: "/openapi.yaml",
		jsonPath: "/openapi.json",
	}
	for _, opt := range opts {
		opt(&config)
	}
	if config.yamlPath != "" {
		engine.GET(config.yamlPath, YAMLHandler(g))
	}
	if config.jsonPath != "" {
		engine.GET(config.jsonPath, JSONHandler(g))
	}
}
