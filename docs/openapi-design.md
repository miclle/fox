# Fox OpenAPI 自动生成 — 设计文档

> 状态：草案 (Draft)
> 版本：v0.1
> 日期：2026-05-03

## 1. 背景与目标

Fox 框架以"约定优于配置"为核心，handler 通过反射自动绑定请求、自动渲染响应。这意味着框架在运行期已经掌握了生成 OpenAPI 3.x spec 所需的几乎全部结构信息：

- 路由路径与 HTTP 方法（来自 `engine.Routes()`）
- 请求参数（来自 handler 入参 struct 的 `json` / `query` / `uri` / `header` / `binding` tag）
- 响应类型（来自 handler 返回值的 `reflect.Type`）
- 错误模型（来自 `httperrors.Error`）

**目标**：

1. 提供一个新子包 `fox/openapi`，对任意 `*fox.Engine` 在运行期自动生成 OpenAPI 3.0.3 spec（可选升级到 3.1）
2. 对现存 handler **零改造**即可获得基础 spec（路径、方法、参数、请求体、响应类型）
3. 提供一套轻量元数据 API，按需补充 `summary`/`description`/`operationId`/`tags`/`security`/多状态码响应
4. 内置 spec 输出端点（`/openapi.yaml`、`/openapi.json`）和可选的 Swagger UI / Scalar / Redoc 静态资源
5. 支持 `DomainEngine` 多域名场景

**非目标**：

- MVP 不做基于源代码注释的解析；后续通过可选 `CommentDocProvider` 做 dev-time AST 增强，不走 `swag` 注释 DSL 路线
- 不做 OpenAPI 2.0 (Swagger) 兼容
- 不替代用户手写 spec 的能力（用户始终可以提供 override）
- 不解决 RPC、GraphQL、gRPC-Gateway 场景

## 2. 设计原则

1. **零侵入优先**：基础 spec 不要求修改任何现存代码，所有元数据补充都是可选的
2. **运行期反射，不做 codegen**：与 fox 既有架构一致；避免引入额外构建步骤
3. **显式优于隐式**：能从代码可靠推断的字段才自动填充；推断不确定时留空，由用户显式声明
4. **可分层覆盖**：自动推断 → 注释元数据 → handler 元数据 → group 元数据 → 全局默认，后者可被前者覆盖
5. **错误优雅降级**：遇到无法反射的类型（`any`、`interface{}`、循环引用）输出 `additionalProperties: true` 并记录警告，不阻断生成

## 3. 架构总览

```mermaid
flowchart LR
    A[*fox.Engine] -->|engine.Routes| B[Route Walker]
    B --> C[Handler Reflector]
    C -->|reflect.Type 入参| D[Parameter / RequestBody Builder]
    C -->|reflect.Type 返回值| E[Response Builder]
    F[Doc Providers<br/>comments/manual] --> G[Spec Assembler]
    D --> G
    E --> G
    H[Schema Cache<br/>Type→Schema] -.-> D
    H -.-> E
    G --> I[openapi3.T]
    I --> J{Serializer}
    J --> K[YAML]
    J --> L[JSON]
    I --> M[Swagger UI / Scalar / Redoc]
```

核心组件职责：

| 组件 | 职责 |
|---|---|
| **Route Walker** | 遍历 `engine.Routes()`，提取 `(method, path, handler)` 三元组 |
| **Handler Reflector** | 解析 handler 签名，定位入参 struct 与返回值 |
| **Schema Cache** | `reflect.Type → *openapi3.SchemaRef` 的全局缓存，用于复用与 `$ref` 引用 |
| **Parameter / RequestBody Builder** | 按 tag 分类字段为 path/query/header 参数或 body schema |
| **Response Builder** | 解析返回类型；从 metadata registry 合并多状态码响应 |
| **Doc Providers** | 按来源补充 operation / schema / field 的描述性元数据 |
| **Spec Assembler** | 拼装 `openapi3.T`，按需引用 components |

## 4. 包结构

```
fox/openapi/
├── doc.go              // 包级注释
├── openapi.go          // 主入口：Generate / Spec / Handler
├── reflector.go        // Handler 签名反射、入参/出参定位
├── schema.go           // reflect.Type → openapi3.Schema 的转换
├── schema_cache.go     // Schema 缓存与 $ref 命名策略
├── tags.go             // tag 解析（json/query/uri/header/binding）
├── binding.go          // binding tag → schema 约束映射
├── metadata.go         // OperationMeta 结构与注册器
├── builder.go          // 链式 builder API（Summary/Tag/Response 等）
├── ui.go               // Swagger UI / Scalar / Redoc 嵌入资源
└── openapi_test.go
```

为什么独立子包：

- 避免主包引入 `kin-openapi` 这一较重依赖，主包用户按需 `import`
- 与 `httperrors` / `render` 现有子包风格一致

## 5. 核心数据流

### 5.1 反射切入点

handler 已被 `IsValidHandlerFunc` 验证，签名固定为以下五类之一：

```
1. func()
2. func(ctx *Context) T
3. func(ctx *Context) (T, error)
4. func(ctx *Context, args S) T
5. func(ctx *Context, args S) (T, error)
```

OpenAPI 生成只关心：

- **入参定位**：当 `numIn == 2` 时，`handlerType.In(1)` 即业务入参 struct（可能是指针）
- **返回类型定位**：当 `numOut >= 1` 时，`handlerType.Out(0)` 即响应类型；若是 `error` 接口或 `nil`，认为无响应体
- **handler 唯一标识**：用 `runtime.FuncForPC(funcValue.Pointer()).Name()` 作为 `operationId` 默认值

### 5.2 Route Walker

`gin.Engine.Routes()` 返回 `[]gin.RouteInfo{Method, Path, Handler, HandlerFunc}`。但 fox 的 handler 被 `handleWrapper` 包装成 `gin.HandlerFunc`，原 handler 的 `reflect.Value` 已丢失。

**方案**：在 `routergroup.go` 的 `Handle` 中，把原 `handler` 注册到新的 `engine.routeRegistry` 字典：

```go
type routeKey struct{ Method, Path string }
type routeEntry struct {
    Handler  HandlerFunc
    HandlerT reflect.Type
    HandlerN string  // runtime func name
    Doc      RouteDoc // 可选，由 builder API 或 DocProvider 写入
}
```

这是对主包的**唯一侵入式改动**。`routeRegistry` 默认开启，只记录轻量的路由与原始 handler 信息，不引入 `openapi` 子包依赖；如用户极度关注内存，可通过后续配置关闭采集。

路径转换规则：

- gin path `:id` 转换为 OpenAPI path `{id}`，如 `/users/:id` → `/users/{id}`
- `uri:"id"` 字段必须能匹配路径参数 `{id}`，不一致时生成 warning
- gin wildcard `*filepath` 转换为 `{filepath}`，并生成 `x-fox-wildcard: true`

### 5.3 入参字段分类

对入参 struct 的每个字段：

| Tag 优先级 | 归属 | 备注 |
|---|---|---|
| `uri:"x"` | path parameter（`required: true`） | 与路径占位符 `:x` 校对 |
| `query:"x"` | query parameter | |
| `header:"x"` | header parameter | |
| `context:"x"` | **跳过** | 由 `ctx.Get` 注入，非客户端可控 |
| 其他 | request body 字段 | 按 `Content-Type` 选择 schema 名 |

请求体的 `Content-Type` 推断顺序：

1. handler 元数据显式声明（builder API）
2. 字段是否带 `form` tag → `application/x-www-form-urlencoded`
3. 字段是否带 `xml` tag → `application/xml`
4. 默认 `application/json`（与 `binding.go` 中 `DefaultBinder = binding.JSON` 一致）

### 5.4 返回类型映射

| handler 返回 | OpenAPI 响应 |
|---|---|
| `T`（具体类型） | `200: { schema: T }` |
| `(T, error)` | `200: T` + 默认错误响应（见 §7） |
| `string` | `200: text/plain` |
| `error` 单返回 | 仅默认错误响应 |
| `render.Render` / `render.Redirect` | 跳过 schema 推断；默认 `200`（Redirect 用 `302` + `Location` header） |
| `any` / `interface{}` / `map[string]any` | `additionalProperties: true`；记录 warning |

## 6. Tag → OpenAPI 映射

### 6.1 字段位置 tag

| 源 tag | OpenAPI 字段 |
|---|---|
| `json:"x,omitempty"` | `properties.x`；是否 required 由 `binding:"required"` 决定 |
| `query:"x"` | `parameters[in=query].name=x` |
| `uri:"x"` | `parameters[in=path].name=x` |
| `header:"x"` | `parameters[in=header].name=x` |
| `form:"x"` | requestBody 的 form schema 字段 |

### 6.2 `binding` 验证 tag

借鉴 `go-playground/validator` 规则集，转换表（不完全列举，按需扩展）：

| validator 规则 | OpenAPI 约束 |
|---|---|
| `required` | `required: true`（在父 schema 的 `required` 数组） |
| `email` | `format: email` |
| `url` / `uri` | `format: uri` |
| `uuid` / `uuid4` | `format: uuid` |
| `min=N` / `max=N`（数值） | `minimum` / `maximum` |
| `min=N` / `max=N`（字符串） | `minLength` / `maxLength` |
| `len=N` | `minLength == maxLength == N` |
| `gte=N` / `lte=N` | `minimum` / `maximum` |
| `gt=N` / `lt=N` | `exclusiveMinimum` / `exclusiveMaximum` |
| `oneof=a b c` | `enum: [a, b, c]` |
| `numeric` | 由类型保证；忽略 |
| `alphanum` | `pattern: "^[a-zA-Z0-9]+$"` |
| `omitempty` | 不进入 `required` |
| 其他未知规则 | 输出 warning，写入 `x-fox-binding` 扩展字段 |

### 6.3 类型映射

| Go 类型 | OpenAPI type / format |
|---|---|
| `string` | `string` |
| `bool` | `boolean` |
| `int` / `int32` | `integer` / `int32` |
| `int64` | `integer` / `int64` |
| `float32` / `float64` | `number` / `float` / `double` |
| `time.Time` | `string` / `date-time` |
| `[]byte` | `string` / `byte` |
| `*T` | `T` 但允许 `nullable: true`（默认 OpenAPI 3.0.3）；启用 3.1 时可用 `type: [T, "null"]` |
| `[]T` | `array`，`items: T` |
| `map[string]T` | `object`，`additionalProperties: T` |
| 自定义 struct | `$ref: "#/components/schemas/<TypeName>"` |
| 实现 `json.Marshaler` 但非 struct | `additionalProperties: true` + warning |

**命名策略**：`pkg.Type` → `pkg_Type`（避免 `/` 等非法字符）；同名冲突时追加包路径片段。

## 7. 元数据 API

### 7.1 DocProvider 分层

第一版先实现结构生成层。反射负责回答“有什么”；DocProvider 作为扩展点预留，后续负责回答“它是什么意思”。

```go
type DocProvider interface {
    OperationDoc(route RouteInfo) OperationDoc
    FieldDoc(t reflect.Type, field reflect.StructField) FieldDoc
}
```

MVP 只预留接口，不读取 `doc` / `description` 等额外业务 tag。第二版新增：

- `CommentDocProvider`：通过 `go/packages` / AST 读取 handler 函数注释与 struct 字段注释
- `ManualDocProvider`：读取 builder API 显式补充的 metadata

生成流程保持不变：`reflect` 生成基础结构，DocProvider 补充描述，manual override 最后兜底。因此第二版不会推倒第一版，只是增加 metadata 来源。

### 7.2 链式 Builder

`Handle` / `GET` / `POST` 等返回值保持 `gin.IRoutes` 兼容；元数据优先通过 `openapi.Route(...)` 辅助函数挂载到最近注册的路由，后续再评估是否引入 fox 自己的 route wrapper：

```go
router.POST("/users", createUser)
openapi.Route(router, "POST", "/users").
    Summary("Create user").
    Tags("users").
    Response(201, &User{}, "created").
    ErrorResponse(409, "USER_EXISTS", "username taken").
    Security("BearerAuth")
```

不采用把 functional options 混入 `POST(...handlers)` 的形式，因为当前 `handlers ...HandlerFunc` 会把 option 当作 handler 校验，容易破坏现有 API 语义。

### 7.3 Group 级元数据

```go
api := router.Group("/api/v1")
openapi.Group(api).
    Tag("users").
    Security("BearerAuth")
```

实现：用 `*RouterGroup` 的指针作为 key，将 group meta merge 到所有该 group 注册的 operation。

### 7.4 全局元数据

```go
spec := openapi.New(engine,
    openapi.Info{Title: "My API", Version: "1.0.0"},
    openapi.AddServer("https://api.example.com"),
    openapi.SecurityScheme("BearerAuth", openapi.BearerJWT()),
)
```

## 8. 响应与错误模型

### 8.1 默认错误响应 schema

`httperrors.Error.MarshalJSON` 已经定义了稳定的 JSON 结构 (`code` / `error` / `meta` / 自定义 fields)。生成器自动注册一个 `components/schemas/HTTPError`：

```yaml
components:
  schemas:
    HTTPError:
      type: object
      required: [code, error]
      properties:
        code:    { type: string }
        error:   { type: string }
        meta:    { }
      additionalProperties: true
```

handler 凡是返回 `(T, error)` 的，自动添加 `default` 响应指向 `HTTPError`。

### 8.2 多状态码

通过 builder API 显式声明：

```go
op.Response(200, &User{})
op.Response(404, openapi.HTTPError, "user not found")
op.Response(409, openapi.HTTPError, "conflict").WithCode("USER_EXISTS")
```

### 8.3 与 `RenderErrorFunc` 的协作

如果用户设置了 `engine.RenderErrorFunc`，生成器无法推断真实 schema。此时：

- 默认仍用 `HTTPError`，并在 spec 顶部加一条 `x-fox-warning`
- 用户可通过 `openapi.SetErrorSchema(myErrorType)` 覆盖

## 9. 多域名支持

`DomainEngine` 包含多个独立 `*Engine`，每个有自己的路由表。两种输出策略：

**方案 A（默认）：每个域名单独一份 spec**

```
GET  /openapi.yaml          → 主 engine
GET  /openapi.yaml?host=api.example.com → 指定域名
```

**方案 B：合并 spec，使用 `servers` 区分**

```yaml
servers:
  - url: https://api.example.com
  - url: https://admin.example.com
paths:
  /users:
    get:
      x-fox-domains: [api.example.com]
```

> **建议**：默认 A，提供 `openapi.Merge(domainEngine)` 方法生成 B。

## 10. UI 集成

提供三个可选 handler，使用 Go embed 嵌入静态资源：

```go
router.GET("/openapi.yaml",  openapi.YAMLHandler(spec))
router.GET("/openapi.json",  openapi.JSONHandler(spec))
router.GET("/docs",          openapi.SwaggerUI("/openapi.yaml"))
router.GET("/scalar",        openapi.Scalar("/openapi.yaml"))
router.GET("/redoc",         openapi.Redoc("/openapi.yaml"))
```

或一键挂载：

```go
openapi.Mount(router, openapi.MountOptions{
    SpecPath: "/openapi.yaml",
    UIPath:   "/docs",
    UI:       openapi.UISwagger, // or UIScalar / UIRedoc
})
```

## 11. 公开 API 草案

```go
package openapi

// New 构造一个 spec 生成器，立即扫描 engine 的当前路由
func New(engine *fox.Engine, opts ...Option) *Generator

// Option 风格的配置
func Info(info openapi3.Info) Option
func AddServer(url string, opts ...ServerOption) Option
func SecurityScheme(name string, scheme *openapi3.SecurityScheme) Option
func SetErrorSchema(t any) Option
func RegisterFormatter(typ reflect.Type, schema *openapi3.Schema) Option

// Generator
func (g *Generator) Spec() *openapi3.T
func (g *Generator) YAML() ([]byte, error)
func (g *Generator) JSON() ([]byte, error)
func (g *Generator) WriteYAML(w io.Writer) error
func (g *Generator) Mount(opts MountOptions)

// Operation 元数据 API
type Op struct {
    Summary     string
    Description string
    OperationID string
    Tags        []string
    Deprecated  bool
}

func (o *Op) Response(code int, body any, desc ...string) *Op
func (o *Op) ErrorResponse(code int, errorCode, desc string) *Op
func (o *Op) Security(name string, scopes ...string) *Op
func (o *Op) Header(name, desc string, required bool) *Op
```

## 12. 依赖选型

| 候选 | 优点 | 缺点 |
|---|---|---|
| **`github.com/getkin/kin-openapi/openapi3`** | 完整 OpenAPI 3.0/3.1 模型，序列化器完整，社区活跃 | 体积较大 (~150 KB)，依赖较多 |
| `github.com/swaggest/openapi-go` | API 设计较干净 | 文档较少，社区小 |
| 自研 minimal struct + `goccy/go-yaml` | 零外部依赖，体积最小 | 维护成本高，要跟 OpenAPI 规范 |

**推荐**：`kin-openapi`，置于子包内不影响主包用户。

## 13. 实施分阶段

### Phase 1 — MVP（约 2-3 天）

- [ ] 新增 `routeRegistry` 到 `Engine`，记录原 handler reflect.Type
- [ ] `fox/openapi` 包骨架 + reflector + schema cache
- [ ] 类型映射（基本类型 + struct + slice + map + pointer）
- [ ] tag 分类（json/query/uri/header）
- [ ] 常见 `binding` tag → schema 约束（required/email/min/max/gt/lt/oneof）
- [ ] 全局 builder：`Info` / `Server`
- [ ] `YAMLHandler` / `JSONHandler`
- [ ] 单元测试覆盖率 80%+

### Phase 2 — 完整元数据（约 2 天）

- [ ] 扩展 `binding` tag → schema 约束映射，覆盖更多 validator 规则
- [ ] Operation builder API（Summary / Tag / Response / Security）
- [ ] Group 级元数据
- [ ] `httperrors` 自动错误响应
- [ ] `time.Time` 等特殊类型 formatter
- [ ] `CommentDocProvider`：读取 handler 函数注释和 struct 字段注释

### Phase 3 — UI 与多域名（约 1-2 天）

- [ ] Swagger UI / Scalar / Redoc embed
- [ ] `Mount` 一键挂载
- [ ] `DomainEngine` 多域名支持
- [ ] 示例项目 `examples/08-openapi/`
- [ ] 文档与最佳实践

### Phase 4 — 优化与扩展（按需）

- [ ] OpenAPI 3.1 webhooks
- [ ] 自定义类型 formatter 注册（如 `decimal.Decimal`）
- [ ] `oneOf` / `anyOf` 支持（接口类型显式声明）
- [ ] Spec diff 工具（CI 中检测破坏性变更）

## 14. 风险与权衡

| 风险 | 影响 | 缓解 |
|---|---|---|
| 反射性能开销 | 启动期一次性 | 用 schema cache，路由注册期增量构建 |
| handler 返回 `any` | spec 信息缺失 | 输出 warning 列表；推荐用具体类型 |
| 循环引用 struct | 栈溢出 | schema cache 在递归前先放占位符 `$ref` |
| `engine.routeRegistry` 内存占用 | 路由多时占用上升 | 默认只存轻量 reflect 信息；后续提供关闭采集配置 |
| `kin-openapi` 升级破坏性变更 | API 不稳定 | 在 `fox/openapi` 内做一层薄封装，不暴露原始类型 |
| 与 `swag` 等用户自有方案冲突 | 用户已有 spec | 提供 `openapi.Merge(existingSpec)` 合并能力 |

## 15. 待确认问题

1. **是否在 Debug 模式自动挂载 `/openapi.yaml`**？类似 gin 的 debug 路由
2. **`time.Time` 默认 format**：MVP 用 `date-time`，是否允许通过 tag `format:"date"` 覆盖？
3. **国际化**：错误响应 description 是否支持多语言？

## 16. 参考实现

- [Huma](https://huma.rocks/) — Go 反射式 OpenAPI 框架，可参考其类型映射策略
- [Echo Swagger](https://github.com/swaggo/echo-swagger) — 注释式
- [Fiber Swagger](https://github.com/swaggo/fiber-swagger) — 注释式
- [Goa](https://goa.design/) — DSL 式（与 fox 设计哲学差异较大，仅参考）
- [Chi OpenAPI](https://github.com/swaggest/rest) — 显式声明式

---

**下一步**：等待 §15 问题确认后，进入 Phase 1 实现。
