# Fox OpenAPI — 设计与 CLI 实现交接文档

> 状态：library 已实现 Phase 1+2 作为内部反射载体；CLI 为生产推荐方向，待实现
> 版本：v0.5（library + CLI 合并稿）
> 日期：2026-05-04
> 接收方注意：本文档面向**没有当前会话上下文**的实现者（人或 AI）。所有背景、目标、决策、陷阱、验收标准都写在此文档内，不需要回看会话历史。

---

## 0. 阅读指引

| 你是谁 | 应先读 |
|---|---|
| 第一次接手 CLI 实现的工程师 | §1 → §2 → §3 → §4 → §5 → §10 |
| 想理解反射 / spec 生成内部机制 | §6 → §7 → §8 |
| 评审本设计的 reviewer | §1 → §3 → §13 |
| 排查实现中的问题 | §10 → §13 → §14 |

强烈建议动手前**通读一遍全文**，CLI 的许多细节互相牵连（driver 文件生成、模块路径解析、entry point 选择）。

---

## 1. 背景与方向

### 1.1 Fox 框架是什么

Fox 是基于 Gin 二次封装的 Go Web 框架（module `github.com/fox-gonic/fox`），核心特性：

- handler 签名固定为反射风格：`func(ctx *fox.Context, args S) (T, error)`（也允许 `func(ctx)` / `func(ctx) T` / `func(ctx) error` 等几种简化变体）
- 框架运行期通过 `reflect` 自动绑定请求参数（`uri` / `query` / `header` / `json` / `form` tag）
- 自动渲染返回值，错误模型统一走 `httperrors.Error`

正因为签名信息丰富，框架在运行期就持有生成 OpenAPI 所需的几乎全部反射信息。

### 1.2 现状

仓库 `openapi/` 子目录是独立 Go module `github.com/fox-gonic/fox-openapi`（用 `replace` 指向本地路径），已实现：

- 从 `*fox.Engine` 的运行期路由表生成 OpenAPI 3.0.3 spec
- path/query/header/body 参数推断
- `binding` validator tag → OpenAPI 约束映射
- `Source(paths, opts)` 从 Go 源码注释补充 summary / description / field doc
- `Operation` / `Group` / `Server` / `SecurityScheme` 等 functional option API
- `httperrors.Error` 自动映射为默认错误响应
- spec endpoint 挂载（`/openapi.yaml`、`/openapi.json`）

### 1.3 路线选择：CLI 为主，library 为内部依赖

经过对 library / Hybrid CLI 两种路线的对比，**正式确定以 CLI 作为生产推荐方向**。

| 方案 | 定位 | 是否继续维护 |
|---|---|---|
| `fox-openapi` library（运行期反射 + 端点挂载） | dev-time 便利方案 + CLI 内部反射载体 | 是，但仅修必要 bug，新功能不在 library 上扩展 |
| `fox-openapi` CLI（独立工具，构建期产出 YAML） | **生产推荐方向**：CI 落地 spec 制品 | 是，新功能在此演进 |

**为什么改 CLI**：

1. **零业务侵入**：业务代码不需要 import openapi 包、不需要在 main 中 `openapi.New(...)` / `Mount(...)`
2. **零运行期开销**：spec 在 CI 中生成，不进入业务进程的内存与启动路径
3. **可纳入版本控制**：`api/openapi.yaml` 作为 PR 的 review 对象，破坏性变更显式可见
4. **复用现有 library 反射逻辑**：CLI 在 driver 子进程中调用 library，不重写一遍

**library 兼容性**：library 尚未在生产项目中使用，CLI 化过程中如有 API 调整不需要保证向后兼容。

### 1.4 关键决策记录

| 决策点 | 选择 | 拒绝的方案 | 理由 |
|---|---|---|---|
| 走静态 AST 分析（类似 swag） vs 复用 library 的运行期反射 | **运行期反射（Hybrid CLI）** | 纯静态分析 | fox 支持动态注册路由、handler dressing、非字面量路径，纯静态分析会大量误判 |
| 怎么"运行" | **生成临时 driver `main.go`，`go run` 它，捕获 stdout** | 在 CLI 进程内 `import` 用户包 | Go 不支持运行期加载用户代码 |
| Driver 怎么找入口 | 用户在配置或命令行指定一个**返回 `*fox.Engine` 的导出函数**，如 `internal/server.NewEngine` | 自动发现 main 函数 | main 函数往往启动监听、读配置、连数据库；CLI 不希望产生副作用 |
| 是否要支持 `*DomainEngine` | **暂不支持**，首版只支持单 engine | — | YAGNI，可在第二版加 |
| Library 的 `New(engine, opts...)` 接口要不要保留 | **保留作为 dev-time 工具**，但 CLI 在新功能上演进 | 删除 library | library 内部仍是反射的实现载体；CLI 是其上层调度器 |

### 1.5 设计原则

1. **零侵入优先**：基础 spec 不要求修改任何现存代码，所有元数据补充都是可选的
2. **运行期反射，不做 codegen**：与 fox 既有架构一致；避免引入额外构建步骤
3. **显式优于隐式**：能从代码可靠推断的字段才自动填充；推断不确定时留空
4. **可分层覆盖**：自动推断 → 注释元数据 → handler 元数据 → group 元数据 → 全局默认，后者可被前者覆盖
5. **错误优雅降级**：遇到无法反射的类型（`any`、`interface{}`、循环引用）输出 `additionalProperties: true` 并记录警告，不阻断生成

---

## 2. 用户故事

### 2.1 主流程

```bash
# 在用户项目根目录
$ fox-openapi generate \
    --entry github.com/acme/myapp/internal/server.NewEngine \
    --out api/openapi.yaml

✓ Resolved entry: github.com/acme/myapp/internal/server.NewEngine
✓ Generated driver in .fox-openapi/driver/main.go
✓ Built and ran driver (1.4s)
✓ Wrote api/openapi.yaml (47 paths, 23 schemas, 0 warnings)
```

### 2.2 配置文件

允许把参数固化到 `fox-openapi.yaml`（项目根目录），然后 `fox-openapi generate` 不带参数即可：

```yaml
# fox-openapi.yaml
entry: github.com/acme/myapp/internal/server.NewEngine
out: api/openapi.yaml
format: yaml          # yaml | json
sources:
  - ./...             # 注释提取路径
info:
  title: Acme API
  version: 1.0.0
servers:
  - https://api.acme.com
```

CLI 优先读命令行参数，否则读配置文件，否则使用默认值。

### 2.3 CI 集成

```yaml
# .github/workflows/openapi.yml
- name: Generate OpenAPI spec
  run: |
    go install github.com/fox-gonic/fox-openapi/cmd/fox-openapi@latest
    fox-openapi generate
- name: Verify spec is up to date
  run: git diff --exit-code api/openapi.yaml
```

第二步在 spec 与代码不一致时让 CI 失败，强制开发者把 spec 一起提交。

### 2.4 dev-time check 模式（次要）

```bash
$ fox-openapi check
api/openapi.yaml is out of date. Run `fox-openapi generate` to refresh.
exit code: 4
```

---

## 3. CLI 命令面

### 3.1 二进制与位置

`fox-openapi`，主入口位于 `cmd/fox-openapi/main.go`（在 `fox-openapi` module 下，与 library 同 module）。

### 3.2 子命令

| 命令 | 功能 |
|---|---|
| `fox-openapi generate` | 生成 spec 并写到 `--out` |
| `fox-openapi check` | 生成到临时文件并 diff `--out`；不一致时退出码 4 |
| `fox-openapi serve` | 本地起 HTTP 服务，预览 spec 与内嵌 UI（Swagger UI / Scalar / Redoc）；支持文件变化自动重新生成 |
| `fox-openapi version` | 打印版本与 commit |

### 3.3 全局 flags

| flag | 默认值 | 说明 |
|---|---|---|
| `--config` | `./fox-openapi.yaml` | 配置文件路径，不存在不报错 |
| `--entry` | （无） | 形如 `module/path/pkg.FuncName`；必须导出且签名为 `func() *fox.Engine` 或 `func() (*fox.Engine, error)` |
| `--out` | `api/openapi.yaml` | 产物路径 |
| `--format` | 由 `--out` 后缀推断 | `yaml` 或 `json` |
| `--source` | `./...`（可重复） | 传给 library `Source()` 的路径列表 |
| `--include-test-files` | `false` | 透传 `IncludeTestFiles()` |
| `--workdir` | 当前目录 | 用户项目根目录（含 `go.mod`） |
| `--keep-driver` | `false` | 保留临时 driver 目录用于排查 |
| `--verbose` | `false` | 打印执行细节 |

### 3.4 serve 子命令的额外 flags

| flag | 默认值 | 说明 |
|---|---|---|
| `--addr` | `127.0.0.1:8765` | HTTP 监听地址 |
| `--ui` | `swagger`（可重复） | 内嵌的 UI；可选 `swagger` / `scalar` / `redoc`；多个 UI 同时挂载在不同路径 |
| `--watch` | `true` | 监听 entry 包及 `--source` 路径下 `.go` 文件变化，自动重新跑 driver 刷新 spec |
| `--open` | `false` | 启动后用系统默认浏览器打开主 UI 路径 |

### 3.5 退出码

| 退出码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 用法错误（参数缺失、配置非法、entry 找不到） |
| 2 | 用户代码编译失败 |
| 3 | driver 运行时崩溃 |
| 4 | check 模式发现 drift |
| 5 | 写文件失败 |

---

## 4. 整体架构

```mermaid
flowchart TB
    A[fox-openapi CLI] --> B[Config Loader]
    B --> C[Entry Resolver]
    C --> D[Driver Generator]
    D --> E[Temp Workspace]
    E --> F[go run]
    F --> G[Driver Process]
    G -->|stdout: spec bytes| H[CLI Receiver]
    H --> I[Format & Write]

    subgraph "Driver Process（用户代码 + 注入）"
      G1[import 用户 entry] --> G2[调用 NewEngine]
      G2 --> G3[openapi.New + Source + 其他 opts]
      G3 --> G4[Spec.YAML/JSON]
      G4 --> G5[os.Stdout.Write]
    end

    G --> G1
```

**Driver 进程是整个方案的关键**：CLI **临时生成**一个 Go 程序作为子进程运行。它 import 用户的 entry 包，调用 entry 函数拿到 `*fox.Engine`，再调用 library 的 `openapi.New(...)` 生成 spec，把字节流写到 stdout。CLI 父进程读 stdout 即可。

---

## 5. CLI 实现细节

### 5.1 Driver 模板

```go
// Code generated by fox-openapi. DO NOT EDIT.
package main

import (
    "fmt"
    "os"

    "github.com/fox-gonic/fox-openapi"

    userentry "{{.EntryImportPath}}"
)

func main() {
    engine{{if .EntryReturnsError}}, err{{end}} := userentry.{{.EntryFuncName}}()
    {{if .EntryReturnsError}}
    if err != nil {
        fmt.Fprintf(os.Stderr, "entry returned error: %v\n", err)
        os.Exit(1)
    }
    {{end}}

    opts := []openapi.Option{
        {{range .InfoOpts}}{{.}},
        {{end}}
        {{range .ServerOpts}}{{.}},
        {{end}}
        {{if .SourcePaths}}
        openapi.Source(
            []string{ {{range .SourcePaths}}{{printf "%q" .}}, {{end}} },
            {{if .IncludeTestFiles}}openapi.IncludeTestFiles(),{{end}}
        ),
        {{end}}
    }

    g := openapi.New(engine, opts...)

    var (
        out []byte
        err2 error
    )
    {{if eq .Format "json"}}
    out, err2 = g.JSON()
    {{else}}
    out, err2 = g.YAML()
    {{end}}
    if err2 != nil {
        fmt.Fprintf(os.Stderr, "generate spec: %v\n", err2)
        os.Exit(1)
    }

    for _, w := range g.Warnings() {
        fmt.Fprintln(os.Stderr, "WARN:", w)
    }

    if _, err := os.Stdout.Write(out); err != nil {
        fmt.Fprintf(os.Stderr, "write stdout: %v\n", err)
        os.Exit(1)
    }
}
```

### 5.2 临时工作区布局

**推荐方案**：driver 放在用户项目下 `<workdir>/.fox-openapi/driver/`，单独一个 `go.mod`，通过 `replace` 指回用户项目：

```
<workdir>/.fox-openapi/driver/
├── go.mod        # module fox-openapi-driver; require fox-openapi; replace acme/myapp => ../..
└── main.go
```

**优点**：用户项目的 go.mod 完全不动；CLI 只需要写 4 个文件；执行后可清理。

**陷阱**：用户项目本身可能有 `replace` 指令（如 `replace github.com/fox-gonic/fox => ../fox`），driver 的 go.mod 也要复制这些 replace。实现时把用户 `go.mod` 解析后，把所有 `replace` 行原样复制到 driver go.mod，**并把相对路径转成绝对路径**（因为 driver 在 `.fox-openapi/driver/` 下，相对路径 base 不同）。

把这些放到 `internal/cli/modfile.go`，用 `golang.org/x/mod/modfile` 解析与重写 go.mod。

### 5.3 Config Loader

- 复用 library 已使用的 `github.com/goccy/go-yaml`
- 优先级：CLI flag > config 文件 > 默认值
- 配置文件不存在时静默用默认值；`--config` 显式指定但不存在则报错

### 5.4 Entry Resolver

输入示例：`github.com/acme/myapp/internal/server.NewEngine`

步骤：

1. 拆分为 `importPath = github.com/acme/myapp/internal/server`，`funcName = NewEngine`
2. 用 `golang.org/x/tools/go/packages` 加载 `importPath`，`Mode = NeedTypes | NeedSyntax | NeedTypesInfo`
3. 在包的 `Scope()` 找名为 `funcName` 的对象，必须是 `*types.Func` 且导出
4. 检查签名：
   - `func() *fox.Engine` ✓
   - `func() (*fox.Engine, error)` ✓
   - 其他 → 报错退出 1
5. 输出 `EntryImportPath` / `EntryFuncName` / `EntryReturnsError` 给 driver 模板

> **怎么判断返回类型是 `*fox.Engine`**：检查 result type 是 `*types.Pointer`，其 elem 是 `*types.Named`，名称为 `Engine` 且属于 fox module。即对 `Named.Obj().Pkg().Path() == "github.com/fox-gonic/fox"` 做匹配。注意用户可能 vendor 了 fox，此时 path 不同——首版可放宽为只匹配 `Pkg().Name() == "fox"` && type name == "Engine"，并在 verbose 模式打印 warning。

### 5.5 Driver 执行

```go
func runDriver(workdir, driverDir string) ([]byte, error) {
    cmd := exec.Command("go", "run", "./"+filepath.Base(driverDir))
    cmd.Dir = workdir
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        return nil, &driverError{exitCode: cmd.ProcessState.ExitCode(), stderr: stderr.String(), cause: err}
    }
    forwardWarnings(stderr.String())
    return stdout.Bytes(), nil
}
```

注意：
- 用 `go run`，不要 `go build`+exec。`go run` 自动管理临时编译产物
- 必须设置 `cmd.Dir = workdir`，否则解析不了用户项目的 `go.mod`
- driver 的 stderr 用作 warning / 错误通道，stdout 严格只放 spec 字节
- 一般继承用户环境（含 `GOFLAGS`、`GOPROXY` 等）

### 5.6 Format & Write

- `--out` 父目录不存在时自动创建（`os.MkdirAll`）
- 写入用临时文件 + rename，避免 spec 写到一半 CI 读到部分内容
- check 模式：写到内存 buffer，与 `--out` 内容做字节比较；不一致退出 4

### 5.7 包结构

```
openapi/                            # 既有 library module
├── go.mod
├── openapi.go                      # 反射核心
├── ...                             # 其它 library 反射逻辑
├── cmd/
│   └── fox-openapi/
│       └── main.go                 # CLI entry，cobra 或 flag.NewFlagSet
└── internal/
    └── cli/
        ├── config.go               # Config struct + loader
        ├── resolve.go              # Entry resolver（go/packages）
        ├── driver.go               # 模板渲染 + 临时目录管理
        ├── modfile.go              # go.mod 解析与重写
        ├── runner.go               # exec go run + 输出捕获
        ├── writer.go               # 原子写入 + check diff
        ├── templates/
        │   └── driver.go.tmpl
        └── cli_test.go
```

CLI 与 library 在**同一 module**：CLI 直接 `import "github.com/fox-gonic/fox-openapi"` 复用 generator；driver 模板里 import 的也是这个 module。

### 5.8 Entry 函数的约定（用户文档示例）

```go
// internal/server/server.go
package server

import "github.com/fox-gonic/fox"

// NewEngine constructs the HTTP engine with all routes registered.
// fox-openapi CLI uses this as its entry. It must:
//   1. Register all routes you want documented
//   2. NOT start listening (no engine.Run)
//   3. NOT depend on external services (DB, cache, etc.) at construction time
func NewEngine() *fox.Engine {
    e := fox.New()
    registerRoutes(e)
    return e
}
```

如果用户的 main 把"注册路由"和"连数据库"耦合在一起，要建议他重构出无副作用的 `NewEngine`。

---

## 6. Library 内部反射机制（CLI 在 driver 进程中调用）

以下是 library 已实现的反射逻辑——CLI 不需要重写，但实现者需要理解，以便排查为什么生成的 spec 长这样。

### 6.1 Route Walker

`gin.Engine.Routes()` 返回 `[]gin.RouteInfo`，但 fox 的 handler 被 `handleWrapper` 包成 `gin.HandlerFunc`，原 handler 的 `reflect.Value` 已丢失。

**解决**：fox core 在 `routergroup.go` 的 `Handle` 中，把原 `handler` 注册到 `engine.handlerRoutes`：

```go
type RouteInfo struct {
    Method      string
    Path        string
    Handler     HandlerFunc
    HandlerType reflect.Type
    HandlerName string
}
func (engine *Engine) HandlerRoutes() []RouteInfo
```

这是 fox core 的**唯一侵入式改动**。可通过 `engine.DisableRouteRegistry()` 关闭采集（CLI 一次性使用，不需要调用此方法）。

路径转换规则：
- gin path `:id` → OpenAPI `{id}`
- gin wildcard `*filepath` → `{filepath}`
- `uri:"id"` 字段必须能匹配路径参数；不一致生成 warning
- 当 handler 没有对应 `uri` 入参时，从路由占位符补充 `string` 类型 path parameter

### 6.2 Handler 反射切入点

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
- **handler 唯一标识**：用 `runtime.FuncForPC(funcValue.Pointer()).Name()` 作为 `operationId` 默认值（library 内部 `cleanHandlerName` 会去掉 `-fm` / `.funcN` 后缀）

### 6.3 入参字段分类

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
3. 默认 `application/json`

### 6.4 返回类型映射

| handler 返回 | OpenAPI 响应 |
|---|---|
| `T`（具体类型） | `200: { schema: T }` |
| `(T, error)` | `200: T` + 默认错误响应（见 §8） |
| `string` | `200: text/plain` |
| 无返回值 | `200`，无响应体 |
| `error` 单返回 | 仅默认错误响应 |
| `render.Render` / `render.Redirect` | 跳过 schema 推断 |
| `any` / `interface{}` / `map[string]any` | `additionalProperties: true`；记录 warning |

---

## 7. Tag → OpenAPI 映射

### 7.1 字段位置 tag

| 源 tag | OpenAPI 字段 |
|---|---|
| `json:"x,omitempty"` | `properties.x`；是否 required 由 `binding:"required"` 决定 |
| `query:"x"` | `parameters[in=query].name=x` |
| `uri:"x"` | `parameters[in=path].name=x` |
| `header:"x"` | `parameters[in=header].name=x` |
| `form:"x"` | requestBody 的 form schema 字段 |

### 7.2 `binding` 验证 tag

借鉴 `go-playground/validator` 规则集：

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

### 7.3 类型映射

| Go 类型 | OpenAPI type / format |
|---|---|
| `string` | `string` |
| `bool` | `boolean` |
| `int` / `int32` | `integer` / `int32` |
| `int64` | `integer` / `int64` |
| `float32` / `float64` | `number` / `float` / `double` |
| `time.Time` | `string` / `date-time` |
| `[]byte` | `string` / `byte` |
| `*T` | `T` 但允许 `nullable: true` |
| `[]T` | `array`，`items: T` |
| `map[string]T` | `object`，`additionalProperties: T` |
| 自定义 struct | `$ref: "#/components/schemas/<TypeName>"` |
| 实现 `json.Marshaler` 但非 struct | `additionalProperties: true` + warning |

**命名策略**：`pkg.Type` → `pkg_Type`；同名冲突时第一个保留短名，后来者使用包路径全名 + 输出 warning。

---

## 8. 元数据 API 与错误模型

### 8.1 已实现的 Option 集合（CLI 通过 driver 透传）

```go
package openapi

func New(engine *fox.Engine, opts ...Option) *Generator

// 全局
func Info(title, version string) Option
func Server(url string) Option
func SecurityScheme(name string, scheme *openapi3.SecurityScheme) Option
func HTTPBearerSecurity(description string) *openapi3.SecurityScheme
func RegisterFormatter(typ reflect.Type, schema *openapi3.Schema) Option
func SetErrorSchema(body any) Option

// 分组与单路由
func Group(prefix string, opts ...OperationOption) Option
func Operation(method, path string, opts ...OperationOption) Option

// 单路由 option
func Summary(value string) OperationOption
func Description(value string) OperationOption
func OperationID(value string) OperationOption
func Tags(values ...string) OperationOption
func Deprecated(value bool) OperationOption
func Response(status int, body any, description string) OperationOption
func Security(name string, scopes ...string) OperationOption

// 注释提取
func Source(paths []string, opts ...SourceOption) Option
func IncludeTestFiles() SourceOption

// 输出
func (g *Generator) Spec() *openapi3.T
func (g *Generator) YAML() ([]byte, error)
func (g *Generator) JSON() ([]byte, error)
func (g *Generator) WriteYAML(w io.Writer) error
func (g *Generator) Warnings() []string

// dev-time mount（CLI 不使用）
func Mount(router Router, g *Generator, opts ...MountOption)
func YAMLHandler(g *Generator) fox.HandlerFunc
func JSONHandler(g *Generator) fox.HandlerFunc
```

**这些 API 已经稳定，CLI 实现期间不要改签名**。如果 CLI 实现过程中发现 library bug，**先在 library 修，CLI 不要绕过**。

### 8.2 注释提取（Source）

`Source` 让反射回答"有什么"，源码注释回答"它是什么意思"：

- handler 函数注释第一段 → operation `summary`
- handler 函数完整注释 → operation `description`
- request / response struct 字段注释 → schema property `description`

实现基于标准库 `go/parser`，可读取普通 `.go` 文件；`IncludeTestFiles()` 时也读 `*_test.go`。

### 8.3 默认错误响应 schema

`httperrors.Error.MarshalJSON` 已经定义了稳定的 JSON 结构。生成器自动注册 `components/schemas/HTTPError`：

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

凡是返回 `(T, error)` 的 handler，自动添加 `default` 响应指向 `HTTPError`。

如果用户设置了 `engine.RenderErrorFunc`，生成器无法推断真实 schema，默认仍用 `HTTPError`，并在 spec 顶部加一条 `x-fox-warning`；用户可通过 `openapi.SetErrorSchema(myErrorType)` 覆盖。

---

## 9. 依赖

| 候选 | 选用 |
|---|---|
| `github.com/getkin/kin-openapi/openapi3` | ✓ 完整 OpenAPI 3.0/3.1 模型 |
| `github.com/goccy/go-yaml` | ✓ YAML 序列化 + 配置文件解析 |
| `golang.org/x/tools/go/packages` | ✓ Entry resolver |
| `golang.org/x/mod/modfile` | ✓ go.mod 重写 |
| `github.com/spf13/cobra` 或标准 `flag` | 二选一，建议标准 `flag.NewFlagSet`（无额外依赖） |

---

## 10. CLI 实施 TODO

每项独立可验证，按顺序推进。

### TODO 1：CLI 骨架 + generate 最小路径

- **目标**：`fox-openapi generate --entry pkg.Func` 跑通最简单 case
- **交付**：
  - `cmd/fox-openapi/main.go` 解析参数
  - `internal/cli/cli.go` 串起 config → resolve → driver → run → write
  - `<workdir>/.fox-openapi/driver/` 创建逻辑
  - driver 模板 hardcode 最小版本（不支持 Source）
- **验收**：
  - `examples/08-openapi/` 上运行能输出非空 yaml
  - 与 `examples/08-openapi/expected.yaml` 一致
  - `--keep-driver` 时目录保留；否则清理

### TODO 2：Config 文件加载

- **目标**：`fox-openapi generate` 不带任何 flag 也能从 `fox-openapi.yaml` 读全部参数
- **交付**：`internal/cli/config.go` + 单测
- **验收**：覆盖矩阵单测（仅 flag / 仅配置 / 都有 flag 优先 / 都没默认值）；`--config` 指定不存在文件时报错；未指定时静默

### TODO 3：Entry Resolver

- **目标**：从 `module/path/pkg.FuncName` 安全解析出 import path、函数名、是否返回 error
- **交付**：`internal/cli/resolve.go`，使用 `golang.org/x/tools/go/packages`
- **验收**：单测覆盖合法 entry / 不存在的包 / 不存在的函数 / 函数签名不匹配 / 函数未导出；错误信息能告诉用户具体哪一步失败、期望签名是什么

### TODO 4：Driver 模板与 go.mod 重写

- **目标**：能为任意用户项目生成可编译的 driver，包括有 replace 指令的项目
- **交付**：
  - `internal/cli/templates/driver.go.tmpl`
  - `internal/cli/driver.go` 渲染
  - `internal/cli/modfile.go` 用 `x/mod/modfile` 解析+重写 go.mod，复制 replace 指令并 rebase 路径
- **验收**：含 `replace` 的 fixture 项目上跑 generate 成功；`--keep-driver` 后手动 `cd .fox-openapi/driver && go build .` 也能成功

### TODO 5：Driver 执行与 stderr/stdout 分离

- **目标**：spec 字节走 stdout 干净；warning / 错误走 stderr 转发到 CLI
- **交付**：`internal/cli/runner.go`
- **验收**：driver panic 时 CLI 退出 3，stderr 含 panic 栈；entry 返回 error 时 CLI 退出 3，stderr 含 entry error；`WARN:` 行被前缀化打到 CLI 自身 stderr，不污染输出文件

### TODO 6：Source / Info / Server option 透传

- **目标**：CLI 收到的配置都能反映到最终 spec
- **交付**：扩展 driver 模板，按配置渲染对应 `openapi.Option`
- **验收**：配置 `info: { title: Acme }` → 生成 yaml `info.title == "Acme"`；source 配置生效（handler 注释出现在 operation summary 中）

### TODO 7：原子写入 + 目录创建

- **目标**：spec 写入安全
- **交付**：`internal/cli/writer.go`
- **验收**：父目录不存在时自动创建；中途 panic 不留半截目标文件；format 自动从扩展名推断

### TODO 8：check 子命令

- **目标**：`fox-openapi check` 在 spec 与代码不一致时退出 4
- **交付**：subcmd 入口 + diff 逻辑
- **验收**：一致 → 退出 0，stdout 一行 OK；不一致 → 退出 4，stderr 输出"out of date, run generate"；`--out` 文件不存在 → 退出 4

### TODO 9：serve 子命令（本地预览 + 内嵌 UI）

- **目标**：`fox-openapi serve` 起本地 HTTP 服务，提供 spec endpoint 与至少一种内嵌 UI；watch 模式下源码改动自动重新生成
- **交付**：
  - `internal/cli/serve.go` 实现 HTTP server + 路由：
    - `GET /openapi.yaml`、`GET /openapi.json` 返回当前 spec 字节
    - `GET /docs`（Swagger UI，默认）、`GET /scalar`（Scalar）、`GET /redoc`（Redoc）
    - 静态资源用 `embed` 内置（不上网拉 CDN）；HTML 模板里把 spec URL 指向同进程的 `/openapi.yaml`
  - `internal/cli/watcher.go` 用 `github.com/fsnotify/fsnotify` 监听 entry 包目录与 `--source` 路径下 `.go` 文件，去抖 300ms 后重跑 driver
  - 重新生成失败时**不替换**当前内存中 spec，stderr 打印错误，UI 继续展示上一份可用 spec
  - `--open` 时启动后用 `os/exec` 调系统命令打开浏览器（macOS `open`、Linux `xdg-open`、Windows `rundll32`）
  - library 侧需要导出可复用的 UI HTML 渲染函数（或 CLI 直接内置 HTML 模板，二选一；建议 CLI 内置避免污染 library）
- **验收**：
  - `fox-openapi serve --addr :8765 --ui swagger --ui scalar --ui redoc` 启动后 `curl localhost:8765/openapi.yaml` 拿到 spec；浏览器打开 `/docs`、`/scalar`、`/redoc` 都能正常渲染且无外网请求（断网情况下也能用）
  - 修改 handler 注释 → 1 秒内 UI 刷新后看到新 summary
  - 故意写错代码导致编译失败 → server 不挂、UI 显示上一份 spec、CLI stderr 打印编译错误
  - `Ctrl+C` 优雅退出（关闭 listener、停 watcher、清理临时 driver 目录）
  - 端口被占用时退出 1 并给出清晰提示

### TODO 10：端到端测试 + CI

- **目标**：把 `examples/08-openapi/` 接入 GitHub Actions
- **交付**：
  - `examples/08-openapi/api/openapi.yaml` 入库
  - `.github/workflows/openapi.yml`
  - `internal/cli/cli_test.go` 端到端：起子进程跑 CLI，断言产物
- **验收**：故意改 handler 签名 → PR diff 命中 yaml 变化；CI workflow 在 main baseline 跑 check 通过

### TODO 11：文档

- **目标**：用户文档 + 迁移指南
- **交付**：
  - `openapi/cmd/fox-openapi/README.md` — quickstart
  - 在本文档追加 CLI 实战 troubleshooting
- **验收**：同事按 README 跑通一次 generate；文档涵盖安装、entry 函数怎么写、CI 集成示例、5 个常见错误

---

## 11. 多域名 / DomainEngine 支持（Future Work）

`DomainEngine` 包含多个独立 `*Engine`。两种输出策略：

- **方案 A（默认）**：每个域名单独一份 spec
  ```
  GET  /openapi.yaml          → 主 engine
  GET  /openapi.yaml?host=api.example.com → 指定域名
  ```
- **方案 B**：合并 spec，使用 `servers` 区分

首版 CLI 不实现。第二版可让 entry 返回 `*DomainEngine`，CLI 输出多份 spec（per-host 文件名）。

---

## 12. UI 集成（serve 子命令必做）

`fox-openapi serve` 是首版必须交付的子命令，目标是让开发者本地一条命令即可获得：

- 当前代码对应的 spec endpoint：`/openapi.yaml`、`/openapi.json`
- 至少 Swagger UI / Scalar / Redoc 三种 UI 中的一种（建议默认 Swagger UI，其他两种通过 `--ui` 同时开启）
- 源码变化自动刷新（默认 watch 模式）

### 12.1 内嵌资源策略

UI 静态资源用 `embed` 打进 CLI 二进制，**不要在 HTML 里 `<script src="https://cdn...">`**。原因：

- 离线环境（公司内网、飞机上）也要可用
- 避免 CDN 失败 / 版本漂移导致 UI 行为不可控
- 服务端版本与前端 UI 版本对齐，减少调试噪声

具体路径建议：

```
internal/cli/ui/
├── ui.go                 # 路由注册：/docs → swagger, /scalar → scalar, /redoc → redoc
├── swagger/              # Swagger UI dist 文件（或单文件 swagger-ui-bundle.js）
├── scalar/
├── redoc/
└── templates/
    ├── swagger.html.tmpl # 引用 /openapi.yaml 作为 spec URL
    ├── scalar.html.tmpl
    └── redoc.html.tmpl
```

### 12.2 watch 行为

- fsnotify 监听 entry 包所在目录及 `--source` 路径下递归的 `.go` 文件
- 单次事件触发后 300ms 去抖，再串行重跑 driver
- 重跑成功 → 替换内存中 spec；失败 → 保留上一份，stderr 打印 cause
- watch 默认 on，`--watch=false` 关闭（CI 中不应该用 serve，但保留这个 flag 便于排查）

### 12.3 与 generate 的关系

serve 内部复用与 generate 相同的 driver pipeline（resolve → driver → run），只是输出端从"写文件"换成"放进 HTTP handler 的内存"。**不要为 serve 重写一套 driver**。把 driver pipeline 抽成 `internal/cli/pipeline.go` 的 `Run(ctx) ([]byte, error)`，generate 和 serve 都调它。

### 12.4 library 侧的 dev-time mount（保留）

library 仍保留以下 API 作为 dev-time 工具，业务代码可以在自己的 main 中挂载（适合本地手工写 `engine.Run` 起服务的场景）：

```go
router.GET("/openapi.yaml", openapi.YAMLHandler(spec))
router.GET("/openapi.json", openapi.JSONHandler(spec))
openapi.Mount(router, spec)
```

但**生产推荐路径仍是 `fox-openapi serve` 而非把 mount 写进业务 main**，原因与 §1.3 一致。

---

## 13. 风险与已知陷阱

| 风险 | 触发场景 | 缓解 |
|---|---|---|
| 用户 entry 函数有副作用（连 DB / 起 goroutine / `os.Exit`） | 非纯函数 entry | 文档警示；考虑加 `--timeout` |
| 用户项目用 vendor | vendor 下没有 fox-openapi 包 | 提示用户 `go mod vendor` 后重跑，或不用 vendor |
| 用户项目用 workspace（`go.work`） | driver 临时目录看不到 workspace | 把 `go.work` 复制到 driver 目录或在 driver 目录写 `go.work` 引用 workspace 根 |
| 路由路径包含动态片段 | spec 出现意料之外的 path | library 已照实记录，是用户行为预期 |
| handler 是闭包或方法值 | runtime function name 带 `.func1` / `-fm` 后缀 | library `cleanHandlerName` + `normalizeRuntimeFuncName` 已处理 |
| 用户 `replace` 指向相对路径 | rebase 到 driver 临时目录后路径错 | TODO 4 必须正确转换为绝对路径 |
| Windows 路径分隔符 | `filepath.Join` vs `path.Join` 混用 | 一律用 `filepath`；模板里 import path 用 `path` |
| go.sum 校验失败 | driver go.mod 引入版本 sum 缺失 | driver 临时目录跑 `go mod download`；或复用用户 go.sum |
| 用户 fox 版本与 CLI 期望版本不一致 | API 不兼容 | driver 编译失败；CLI 提示用户升级 fox / fox-openapi |
| 大量 stdout 把 buffer 撑爆 | 极大型 API spec | `cmd.Stdout` 接 `*os.File` 而非内存 buffer |
| handler 返回 `any` | spec 信息缺失 | library 输出 warning 列表；推荐用具体类型 |
| 循环引用 struct | 栈溢出 | library schema cache 在递归前先放占位符 `$ref` |

---

## 14. Troubleshooting Cheat Sheet（写进用户文档）

| 错误 | 含义 | 解决 |
|---|---|---|
| `entry function not found: pkg.Func` | resolver 找不到 | 检查 import path 是否在 go.mod 里、函数是否导出 |
| `entry function signature mismatch` | 签名不允许 | 改成 `func() *fox.Engine` 或 `func() (*fox.Engine, error)` |
| `driver build failed` (退出 2) | 用户代码编译错 | 先 `go build ./...` 修复 |
| `entry returned error` (退出 3) | entry 返回 non-nil error | 看 stderr cause |
| `out of date` (退出 4) | check 模式 spec 不一致 | 跑 `fox-openapi generate` 提交 |

---

## 15. 验收 Checklist（实现完成后逐条勾）

- [ ] `fox-openapi generate` 在 `examples/08-openapi/` 上输出与预期 YAML 一致
- [ ] `fox-openapi check` 在一致 / 不一致两种状态下退出码正确
- [ ] `fox-openapi serve` 起服务后 `/openapi.yaml`、`/docs`、`/scalar`、`/redoc` 全部可访问且离线可用
- [ ] serve watch 模式下改 handler 注释 1 秒内 UI 反映
- [ ] serve 编译失败时不挂、保留旧 spec、stderr 打印 cause
- [ ] 用户项目带 `replace` 指令时仍可生成
- [ ] 用户项目未 require fox-openapi 时 CLI 给出清晰提示
- [ ] driver 临时文件成功后清理，`--keep-driver` 时保留
- [ ] WARN 不污染 spec 输出
- [ ] 端到端测试在 CI 中跑通
- [ ] README 含安装、5 行 quickstart、5 个 troubleshooting

---

## 16. Future Work（不在首版范围）

- `*DomainEngine` 多 engine：entry 可返回 `[]*fox.Engine` 或 `*DomainEngine`，CLI 输出多份 spec
- Spec diff：`fox-openapi diff old.yaml new.yaml` 列破坏性变更
- 增量生成：缓存已解析 schema 加速重复运行
- `fox-openapi lint`：检查命名一致性、缺 description 的 operation
- OpenAPI 3.1 webhooks
- `oneOf` / `anyOf` 支持（接口类型显式声明）

---

## 17. 给实现者的最后建议

1. **先做 happy path**：TODO 1 跑通后再考虑边界情况，不要一开始就处理 vendor / workspace
2. **driver 模板用 `text/template`**，不要拼字符串
3. **entry resolver 出错的提示要带行号**：用户写错 entry 字符串是高频场景
4. **每个 TODO 写完先跑一次 `go test ./internal/cli/...`，再进下一个**
5. **不要在 CLI 里复制 library 已有逻辑**：CLI 的职责只是"把用户代码包起来跑一次"，所有反射和 spec 拼装都在 driver 进程内由 library 完成

祝顺利。
