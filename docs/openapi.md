# OpenAPI Generation

Fox can generate an OpenAPI 3.0.3 document from registered routes and handler
signatures. The current implementation is an MVP focused on structural API
documentation: paths, methods, parameters, request bodies, response bodies, and
common validation constraints.

It does not read handler comments or field comments yet. Comment-based
descriptions are planned as a later `CommentDocProvider` layer, so this first
version does not require `doc` tags or a separate code generation step.

## Install

The OpenAPI generator lives in a separate subpackage:

```go
import "github.com/fox-gonic/fox/openapi"
```

## Basic Usage

Register routes first, then create the generator:

```go
package main

import (
	"github.com/fox-gonic/fox"
	"github.com/fox-gonic/fox/openapi"
)

type GetUserRequest struct {
	ID      int64  `uri:"id" binding:"required,gt=0"`
	Verbose bool   `query:"verbose"`
	Token   string `header:"X-Token" binding:"required"`
}

type UserResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func getUser(ctx *fox.Context, req GetUserRequest) (UserResponse, error) {
	return UserResponse{}, nil
}

func main() {
	router := fox.Default()

	router.GET("/users/:id", getUser)

	spec := openapi.New(router,
		openapi.Info("My API", "1.0.0"),
		openapi.Server("https://api.example.com"),
	)

	router.GET("/openapi.yaml", openapi.YAMLHandler(spec))
	router.GET("/openapi.json", openapi.JSONHandler(spec))

	router.Run(":8080")
}
```

Then fetch the generated spec:

```bash
curl http://localhost:8080/openapi.yaml
curl http://localhost:8080/openapi.json
```

## Write A YAML File

You can also write the generated spec to disk:

```go
file, err := os.Create("openapi.yaml")
if err != nil {
	panic(err)
}
defer file.Close()

if err := spec.WriteYAML(file); err != nil {
	panic(err)
}
```

Or get bytes directly:

```go
yamlData, err := spec.YAML()
jsonData, err := spec.JSON()
```

## What Is Generated

The MVP generates:

- OpenAPI version `3.0.3`
- `info` from `openapi.Info(title, version)`
- `servers` from `openapi.Server(url)`
- `paths` and HTTP methods from registered fox routes
- Gin-style path parameters such as `/users/:id` as `/users/{id}`
- `uri`, `query`, and `header` parameters from handler input structs
- JSON request bodies from handler input struct fields
- JSON response bodies from handler return values
- Default error responses for handlers that return `error`
- A reusable `HTTPError` schema based on `httperrors.Error`

## Supported Tags

Fox reads the same binding tags used by request binding:

```go
type CreateUserRequest struct {
	Name  string `json:"name" binding:"required,min=3"`
	Email string `json:"email" binding:"required,email"`
}
```

Supported parameter location tags:

| Tag | OpenAPI location |
|---|---|
| `uri:"id"` | `parameters[in=path]` |
| `query:"page"` | `parameters[in=query]` |
| `header:"X-Token"` | `parameters[in=header]` |
| `json:"name"` | request body property |
| `context:"user"` | skipped |

Supported validation constraints in the MVP:

| Binding rule | OpenAPI schema output |
|---|---|
| `required` | required parameter or required body field |
| `email` | `format: email` |
| `url`, `uri` | `format: uri` |
| `uuid`, `uuid4` | `format: uuid` |
| `min`, `gte` | `minLength`, `minItems`, or `minimum` |
| `max`, `lte` | `maxLength`, `maxItems`, or `maximum` |
| `gt`, `lt` | exclusive minimum / maximum |
| `len` | exact string length, array length, or numeric bounds |
| `oneof` | enum |
| `alphanum` | alphanumeric pattern |

## Current Limitations

The MVP intentionally does not generate:

- Interface summaries or descriptions from handler comments
- Field descriptions from struct field comments
- Multiple explicit success status codes
- Security schemes
- Swagger UI / Scalar / Redoc assets
- DomainEngine-specific multi-host specs
- Component schema deduplication with `$ref` for all structs

Those are planned as follow-up phases. The current generator is designed so
comment parsing and manual overrides can be added as metadata providers without
replacing the route and schema generation core.
