# OpenAPI Generation

Fox can generate an OpenAPI 3.0.3 document from registered routes and handler
signatures. The current implementation is an MVP focused on structural API
documentation: paths, methods, parameters, request bodies, response bodies, and
common validation constraints.

It can also read regular Go comments from source files to fill operation
summaries, operation descriptions, and schema field descriptions. It does not
require `doc` tags or a separate code generation step.

## Install

The OpenAPI generator lives in a separate module. While it is developed in this
repository, it already uses the future standalone import path:

```go
import "github.com/fox-gonic/fox-openapi"
```

During monorepo development, `openapi/go.mod` uses:

```go
replace github.com/fox-gonic/fox => ../
```

After moving the module to its own repository, that local `replace` can be
removed.

## Basic Usage

Register routes first, then create the generator:

```go
package main

import (
	"github.com/fox-gonic/fox"
	"github.com/fox-gonic/fox-openapi"
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
	openapi.Source("."),
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

## Source Comments

Use `openapi.Source` to add descriptions from regular Go comments:

```go
type CreateUserRequest struct {
	// Display name for the new user.
	Name string `json:"name" binding:"required"`
}

type UserResponse struct {
	// Stable user identifier.
	ID int64 `json:"id"`
}

// Create user.
//
// Creates a user and returns the persisted representation.
func createUser(ctx *fox.Context, req CreateUserRequest) (UserResponse, error) {
	return UserResponse{}, nil
}

spec := openapi.New(router,
	openapi.Info("My API", "1.0.0"),
	openapi.Source("./..."),
)
```

The first paragraph of the handler comment becomes the operation summary. The
full handler comment becomes the operation description. Struct field comments
become schema property descriptions.

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

## Check Warnings

Generation is best-effort. Non-fatal issues are collected as warnings:

```go
for _, warning := range spec.Warnings() {
	log.Println(warning)
}
```

For example, Fox warns when a `uri` tag does not match the registered path:

```go
router.GET("/users/:id", getUser)

type GetUserRequest struct {
	UserID int64 `uri:"user_id"`
}
```

The path contains `:id`, but the struct asks Fox to bind `user_id`, so the
generated OpenAPI path parameter would not match the actual route placeholder.

## What Is Generated

The MVP generates:

- OpenAPI version `3.0.3`
- `info` from `openapi.Info(title, version)`
- `servers` from `openapi.Server(url)`
- operation summaries and descriptions from handler comments via `openapi.Source`
- `paths` and HTTP methods from registered fox routes
- Gin-style path parameters such as `/users/:id` as `/users/{id}`
- fallback path parameters from route placeholders when no matching `uri` tag exists
- `uri`, `query`, and `header` parameters from handler input structs
- warnings for `uri` tags that do not match registered path parameters
- JSON request bodies from handler input struct fields
- request and response field descriptions from struct comments via `openapi.Source`
- URL-encoded form request bodies from `form` tags
- JSON response bodies from handler return values
- `text/plain` response bodies for handlers that return `string`
- empty `200 OK` responses for handlers with no return value
- Struct schemas under `components/schemas` with `$ref` reuse
- Recursive and self-referential structs through component `$ref`s
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
| `form:"username"` | `application/x-www-form-urlencoded` request body property |
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

The current implementation intentionally does not generate:

- Multiple explicit success status codes
- Security schemes
- Swagger UI / Scalar / Redoc assets
- DomainEngine-specific multi-host specs
- Custom schema naming overrides

Those are planned as follow-up phases. The current generator is designed so
manual overrides can be added as metadata providers without replacing the route
and schema generation core.
