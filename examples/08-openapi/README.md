# OpenAPI Generation Example

This example demonstrates Fox's OpenAPI MVP generator.

It shows how to:

- Generate OpenAPI 3.0.3 from registered Fox routes
- Infer path, query, header, request body, and response schemas from handler signatures
- Read handler and struct field comments with `openapi.Source(".")`
- Add explicit operation metadata with `openapi.Operation(...)`
- Apply shared metadata by path prefix with `openapi.Group(...)`
- Register bearer auth metadata with `openapi.SecurityScheme(...)`
- Map common `binding` rules into OpenAPI schema constraints
- Serve `/openapi.yaml` and `/openapi.json` with `openapi.Mount(...)`

## Run

```bash
cd examples/08-openapi
go run .
```

Use a different port if `8080` is already occupied:

```bash
PORT=18080 go run .
```

## Try The API

List users:

```bash
curl "http://localhost:8080/users?page=1&page_size=20"
```

Get a user:

```bash
curl http://localhost:8080/users/1
```

Create a user:

```bash
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Katherine Johnson","email":"katherine@example.com"}'
```

## Fetch The OpenAPI Spec

YAML:

```bash
curl http://localhost:8080/openapi.yaml
```

JSON:

```bash
curl http://localhost:8080/openapi.json
```

The generated spec includes:

- `/users`
- `/users/{id}`
- query parameters from `query` tags
- path parameters from `uri` tags
- JSON request body fields from `json` tags
- response schemas from handler return types
- operation summaries and field descriptions from Go comments
- operation tags and custom operation IDs from explicit metadata
- bearer auth security scheme metadata
- default error response schemas for handlers returning `error`

## Notes

The example is its own Go module and imports `github.com/fox-gonic/fox-openapi`.
During monorepo development, `go.mod` points that import path to `../../openapi`
with a local `replace`.
