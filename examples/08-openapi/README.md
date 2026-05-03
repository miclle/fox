# OpenAPI Generation Example

This example demonstrates Fox's OpenAPI MVP generator.

It shows how to:

- Generate OpenAPI 3.0.3 from registered Fox routes
- Infer path, query, header, request body, and response schemas from handler signatures
- Map common `binding` rules into OpenAPI schema constraints
- Serve `/openapi.yaml` and `/openapi.json`

## Run

```bash
cd examples/08-openapi
go run main.go
```

Use a different port if `8080` is already occupied:

```bash
PORT=18080 go run main.go
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
- default error response schemas for handlers returning `error`

## Notes

This MVP does not read Go comments for summaries or field descriptions yet.
Those descriptions are planned for a later comment-based metadata provider.
