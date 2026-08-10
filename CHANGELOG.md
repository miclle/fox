# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2026-08-10

### Fixed
- Propagated `Context.Request` replacements through the middleware chain, so
  outer middleware can observe request context values injected by inner
  middleware after `Next()` returns.
- Kept the `context.Context` methods backed by an immutable request-context
  snapshot while synchronously carrying the latest request through the chain,
  avoiding races between asynchronous context reads and request replacement.

### Changed
- Updated validator and indirect networking dependencies.
- Updated GitHub Actions dependencies and pinned CI to Go 1.25.12.

## [0.1.1] - 2026-05-07

### Added
- Route manifest export helpers for generating a stable JSON snapshot of
  registered routes, including `RouteManifestFromEngine`, `WriteRouteManifest`,
  and the `fox.route-manifest/v1` manifest format.
- Manifest type metadata for closure handlers, covering input and result types
  while preserving route method, path, and original handler name.

## [0.1.0] - 2026-05-06

This release graduates the project from the `0.0.x` prototype track to the
`0.1.x` line. It bundles the 2026-05 audit results: a sweep of correctness
fixes, a small set of intentional breaking changes (each documented below
with a migration note), test coverage backfill across `examples/` and
`render/`, and Gin baseline benchmarks.

### Breaking Changes
- `IsValidHandlerFunc` no longer accepts `interface` as the second parameter
  type — handlers using `func(ctx, args any)` will now panic at registration.
  Use a concrete `struct` or `map` type instead.
- `logger.NewWithContext` no longer reads trace IDs from the
  `logger.TraceID` string context key. Propagate trace IDs with
  `logger.TraceIDKey` (the typed key) or via `logger.NewContext`.
- `*httperrors.Error` returned from `IsValid()` is no longer wrapped as
  `BIND_ERROR`. Consumers that matched on the `BIND_ERROR` response `code`
  to detect binding failures should use the HTTP 400 status code or their
  own specific codes instead.

### Added
- `examples/*/main_test.go` smoke tests for all examples.
- `render` package smoke test.
- Gin baseline benchmarks for performance comparison.

### Changed
- `httperrors.Error.MarshalJSON` no longer mutates its receiver and now
  flattens `Meta` when it is either a struct or a pointer to a struct (via
  `reflect.Indirect`). The method keeps a value receiver so that `Error` still
  marshals correctly when used as a value (for example in `[]Error` slices
  or `map[string]Error`).
- `bind` skips body reads only when `Content-Length == 0` **and** no transfer
  encoding is declared. Requests with unknown length (`Content-Length == -1`)
  or chunked encoding continue to read the body as before.
- `DefaultValidator` now reuses the package-level `Validate` instance, so
  custom validations registered via `fox.Validate.RegisterValidation(...)`
  are honored during binding.
- Documentation now uses the actual `httperrors.Error` fields and JSON response keys.
- Documentation now aligns Go version, security policy, performance reproduction notes, and release history.

### Fixed
- Package-level `logger.Debug`, `Info`, `Warn`, `Error`, `Fatal`, and `Panic` now spread variadic arguments correctly.
- `*httperrors.Error` returned from `IsValid()` (including values wrapped via
  `fmt.Errorf("%w", ...)`) is passed through without being wrapped as
  `BIND_ERROR`, preserving user-defined `Code` and `HTTPCode`. Detection uses
  `errors.As` so any error chain containing a `*httperrors.Error` is unwrapped.
- `IsValid()` is now called for pointer handler parameters after binding,
  covering value-receiver implementations on the addressable parameter.

### Deprecated
- `ErrInvalidHandlerType`; use `MsgInvalidHandlerType`. The alias will be
  removed in `v0.2.0`.

## [0.0.10] - 2026-04-30

### Added
- Added struct binding examples for URL and query parameters.
- Comprehensive unit tests for `logger` package (90.8% coverage)
- Comprehensive unit tests for `utils` package (100% coverage)
- CHANGELOG.md file for tracking project changes
- Added Codecov integration and expanded test coverage across core packages.

### Changed
- Upgraded the module and GitHub Actions toolchain to Go 1.25.
- Aligned Gin ecosystem dependencies with Gin upstream, including `github.com/gin-gonic/gin` v1.12.0 and `github.com/gin-contrib/cors` v1.7.7.
- Updated `github.com/rs/zerolog` to v1.35.1.
- Updated GitHub Actions dependencies, including `codecov/codecov-action` v6.

### Fixed
- Attached trace IDs to the request context in logger middleware.
- Renamed `reponse_time.go` to `response_time.go` (typo correction)

## [0.1.0-beta] - 2024-12-06

> Note: This pre-release entry is superseded by `[0.1.0]`. The interim
> `0.0.x` releases (see `[0.0.10]` above) carried incremental fixes; the
> `[0.1.0]` entry above marks the project's graduation to the `0.1.x` line.

### Added
- Smart handler signature support with automatic parameter binding and rendering
- Support for multiple handler signatures:
  - `func()`
  - `func(ctx *Context) T`
  - `func(ctx *Context) (T, error)`
  - `func(ctx *Context, args S) T`
  - `func(ctx *Context, args S) (T, error)`
- Automatic parameter binding from JSON, Form, Query, URI, Header
- Context value binding with `context:"key"` tag
- Custom validator interface `IsValider`
- Automatic response rendering based on return types
- Structured HTTP error handling with `httperrors.Error`
- High-performance logging system based on zerolog
- TraceID/RequestID tracking support
- Multi-domain routing support with regex pattern matching
- X-Response-Time middleware for performance monitoring
- CORS configuration support

### Changed
- Upgraded to Go 1.24.0
- Updated github.com/gin-gonic/gin to v1.11.0
- Updated golang.org/x/crypto to v0.45.0
- Updated github.com/go-playground/validator/v10 to v10.28.0
- Updated github.com/stretchr/testify to v1.11.1
- Updated github.com/gin-contrib/cors to v1.7.6
- Updated github.com/rs/zerolog to v1.34.0
- Enabled ContextWithFallback by default for better context.Context support

### Fixed
- Data race issue when concurrently executing `fox.New()` (#33)
- Context.Request not taking effect in middlewares (#31)
- Context.Context implementation for thread safety (#30)
- Valid handler signature validation (#22)
- Linter errors and code formatting issues (#17)

### Removed
- Custom tag name binding from validator (breaking change) (#20)

## Dependencies

### Core Dependencies
- github.com/gin-gonic/gin v1.12.0
- github.com/gin-contrib/cors v1.7.7
- github.com/go-playground/validator/v10 v10.30.3
- github.com/json-iterator/go v1.1.12
- github.com/mitchellh/mapstructure v1.5.0
- gopkg.in/natefinch/lumberjack.v2 v2.2.1
- github.com/rs/zerolog v1.35.1
- github.com/stretchr/testify v1.11.1

### Indirect Dependencies
- golang.org/x/crypto v0.52.0
- golang.org/x/net v0.55.0
- golang.org/x/sys v0.45.0
- golang.org/x/text v0.37.0
- google.golang.org/protobuf v1.36.11

## Development

### CI/CD
- GitHub Actions for automated testing
- golangci-lint v2.4.0 for code quality checks
- Dependabot for automatic dependency updates (weekly)
- Multi-platform testing (Ubuntu, macOS)
- Race condition detection enabled

### Code Quality
- Test coverage: ~57% (main package)
- Linters: gosec, misspell, revive, testifylint, perfsprint, usestdlibvars
- Code formatting: gofmt, gofumpt, goimports

## Notes

**⚠️ Pre-1.0 Status**: This project is on the `0.1.x` line. The public API
is stabilizing but may still see breaking changes in subsequent minor
releases. Each breaking change is documented in this CHANGELOG with a
migration note. Production use is possible with the understanding that you
may need to adapt to breaking changes between minor versions until `v1.0.0`.

### Known Issues
- lumberjack dependency uses +incompatible version

### Upcoming Features
- Built-in security middleware (rate limiting, CSRF, request size limits)
- Additional middleware for common use cases
- Improved documentation and examples
- Performance optimizations

## Contributing

Please see the [contributing guidelines](CONTRIBUTING.md) for more information on how to contribute to this project.

## License

See [LICENSE](LICENSE) file for details.

---

For more information, visit the [GitHub repository](https://github.com/fox-gonic/fox).

[Unreleased]: https://github.com/fox-gonic/fox/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/fox-gonic/fox/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/fox-gonic/fox/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/fox-gonic/fox/compare/v0.0.10...v0.1.0
[0.0.10]: https://github.com/fox-gonic/fox/compare/v0.0.9...v0.0.10
