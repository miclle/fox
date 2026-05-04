# fox-openapi CLI

`fox-openapi` generates an OpenAPI 3.0.3 document from a Fox engine at build time. The normal path keeps business code free of OpenAPI imports: expose a constructor such as `NewEngine() *fox.Engine`, then point the CLI at it.

## Install

```bash
go install github.com/fox-gonic/fox-openapi/cmd/fox-openapi@latest
```

For local development in this repository:

```bash
cd openapi
go run ./cmd/fox-openapi version
```

## Quickstart

Create `fox-openapi.yaml` in your application root:

```yaml
entry: github.com/acme/myapp/internal/server.NewEngine
out: api/openapi.yaml
sources:
  - ./...
info:
  title: Acme API
  version: 1.0.0
servers:
  - url: https://api.acme.com
```

Then generate and check the committed spec:

```bash
fox-openapi generate
fox-openapi check
```

## Entry Function

`entry` must name an exported function with one of these signatures:

```go
func NewEngine() *fox.Engine
func NewEngine() (*fox.Engine, error)
```

The function should register routes and return the engine. It should not call `Run`, open listeners, or start background infrastructure that is not needed for route registration.

## Config

Supported config keys:

- `entry`: required entry function.
- `out`: output file, default `api/openapi.yaml`.
- `format`: `yaml` or `json`; inferred from `out` when omitted.
- `sources`: source directories for Go doc comments; default `./...`.
- `includeTestFiles`: include `_test.go` while scanning source comments.
- `info`: `title`, `version`, `description`.
- `servers`: list of `url` and optional `description`.
- `tags`: top-level OpenAPI tag registry.
- `securitySchemes`: serializable HTTP, API key, OAuth2, or OpenID Connect schemes.
- `metadataHook`: optional advanced Go hook.
- `autoAdd`: run `go get github.com/fox-gonic/fox-openapi` when the user module is missing the requirement.

CLI flags override config values. Config values override defaults.

## Metadata Hook

For metadata that needs Go values, add a small optional hook:

```go
func ConfigureOpenAPI() []openapi.Option {
	return []openapi.Option{
		openapi.Group("/users", openapi.Tags("users")),
		openapi.Operation("GET", "/users/:id", openapi.Security("BearerAuth")),
	}
}
```

Then configure it:

```yaml
metadataHook: github.com/acme/myapp/internal/openapimeta.ConfigureOpenAPI
```

## Commands

```bash
fox-openapi generate --entry github.com/acme/myapp/internal/server.NewEngine --out api/openapi.yaml
fox-openapi check
fox-openapi serve --addr 127.0.0.1:8765
fox-openapi version
```

`serve` exposes `/openapi.yaml`, `/openapi.json`, `/docs`, `/scalar`, and `/redoc`. UI pages are embedded and do not load CDN assets.

## CI

```yaml
- name: Generate OpenAPI spec
  working-directory: openapi
  run: go run ./cmd/fox-openapi generate --workdir ../examples/09-openapi-cli
- name: Verify spec is up to date
  run: git diff --exit-code examples/09-openapi-cli/api/openapi.yaml
```

## Troubleshooting

- `entry is required`: set `entry` in `fox-openapi.yaml` or pass `--entry`.
- `user module must require github.com/fox-gonic/fox-openapi`: add the module requirement or pass `--auto-add`.
- Exit code `2`: generated driver failed to build. Check imports, replaces, and entry/hook signatures.
- Exit code `3`: the driver built but failed at runtime. Check entry side effects or returned errors.
- Exit code `4`: `check` found drift; run `fox-openapi generate` and commit the updated spec.

