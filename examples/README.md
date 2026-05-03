# Fox Framework Examples

This directory contains comprehensive examples demonstrating various features of the Fox web framework.

## Examples Overview

| Example | Description | Key Features |
|---------|-------------|--------------|
| [01-basic](01-basic/) | Basic usage | Simple routes, path parameters, JSON responses |
| [02-binding](02-binding/) | Parameter binding | JSON/URI/Query binding, validation |
| [03-middleware](03-middleware/) | Middleware usage | Custom middleware, authentication, rate limiting |
| [04-domain-routing](04-domain-routing/) | Domain-based routing | Multi-domain, regex patterns, multi-tenant |
| [05-custom-validator](05-custom-validator/) | Custom validation | IsValider interface, complex validation rules |
| [06-error-handling](06-error-handling/) | Error handling | HTTP errors, custom errors, error codes |
| [07-logger-config](07-logger-config/) | Logger configuration | Console/file logging, rotation, structured logs |
| [08-openapi](08-openapi/) | OpenAPI generation | Generated OpenAPI YAML/JSON, schemas, validation constraints |

## Quick Start

Each example has its own directory with:
- `main.go` - The example code
- `README.md` - Detailed documentation and testing instructions

### Running an Example

```bash
# Navigate to an example directory
cd examples/01-basic

# Run the example
go run main.go

# Test the endpoints (see each example's README for specific commands)
curl http://localhost:8080/ping
```

## Example Categories

### 🚀 Getting Started

Perfect for beginners:
1. **01-basic** - Start here to learn the basics
2. **02-binding** - Learn parameter binding and validation
3. **06-error-handling** - Understand error handling

### 🔧 Advanced Features

For more complex scenarios:
4. **03-middleware** - Custom middleware and request processing
5. **04-domain-routing** - Multi-domain and multi-tenant applications
6. **05-custom-validator** - Complex validation rules
7. **07-logger-config** - Production-ready logging

## Common Patterns

### 1. Basic Route

```go
router.GET("/hello", func(ctx *fox.Context) string {
    return "Hello, World!"
})
```

### 2. Parameter Binding

```go
type CreateUserRequest struct {
    Username string `json:"username" binding:"required"`
    Email    string `json:"email" binding:"required,email"`
}

router.POST("/users", func(ctx *fox.Context, req *CreateUserRequest) (*User, error) {
    // req is automatically bound and validated
    return createUser(req), nil
})
```

### 3. Error Handling

```go
var ErrNotFound = &httperrors.Error{
    HTTPCode: http.StatusNotFound,
    Code:     "NOT_FOUND",
    Message:  "Resource not found",
}

router.GET("/user/:id", func(ctx *fox.Context) (*User, error) {
    user, err := findUser(ctx.Param("id"))
    if err != nil {
        return nil, ErrNotFound
    }
    return user, nil
})
```

### 4. Middleware

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !isAuthenticated(c) {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        c.Next()
    }
}

router.Use(AuthMiddleware())
```

### 5. Custom Validation

```go
type SignupRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

func (sr *SignupRequest) IsValid() error {
    if len(sr.Password) < 8 {
        return &httperrors.Error{
            HTTPCode: http.StatusBadRequest,
            Code:     "WEAK_PASSWORD",
            Message:  "Password must be at least 8 characters",
        }
    }
    return nil
}
```

## Learning Path

### Beginner

1. Start with **01-basic** to understand basic routing
2. Move to **02-binding** for parameter handling
3. Learn **06-error-handling** for proper error responses

### Intermediate

4. Study **03-middleware** for request processing
5. Explore **05-custom-validator** for complex validation
6. Configure **07-logger-config** for production logging
7. Generate docs with **08-openapi**

### Advanced

8. Implement **04-domain-routing** for multi-tenant apps
9. Combine patterns from multiple examples
10. Build your own production application

## Prerequisites

- Go 1.24 or higher
- Basic understanding of HTTP and REST APIs
- Familiarity with Go syntax

## Installation

```bash
go get github.com/fox-gonic/fox
```

## Additional Resources

- [Fox Documentation](https://github.com/fox-gonic/fox)
- [Gin Framework](https://gin-gonic.com/) (Fox is built on Gin)
- [Go-Playground Validator](https://github.com/go-playground/validator) (Used for validation)

## Tips

1. **Read the README**: Each example has detailed documentation
2. **Modify and Experiment**: Try changing the code to understand how it works
3. **Check Error Messages**: Fox provides detailed validation errors
4. **Use Structured Logging**: Add context to your logs for better debugging
5. **Handle Errors Properly**: Always return meaningful error messages

## Contributing

Found an issue or have a suggestion for a new example? Please open an issue or pull request in the main repository.

## License

These examples are part of the Fox framework and are released under the same license as Fox.
