# go-rest-template

基于 **TypeSpec + 自研 Go Emitter + Go 1.22+ 标准库 `net/http`** 的现代 Go RESTful API 项目模板（Connect-Go v2 风格纯业务接口），集成了生产就绪的 RFC 9457 错误处理、i18n、JWT 认证、Ent ORM、RFC 7386 JSON Merge Patch、健康探针、可选 CORS 与可观测性基础设施。

## 技术栈

| 分层 | 选型 |
|---|---|
| API 契约 & 代码生成 | TypeSpec 1.18 + `@typespec/openapi3` (OpenAPI 3.1) + 自研 `go-rest` Emitter（支持跨接口共享类型 `types.gen.go`、嵌套/数组校验、全类型 Query 解析、`@autoPatch`）|
| HTTP 运行时 | Go 1.22+ 标准库 `net/http` (`*http.ServeMux`) + 零依赖 `pkg/httpkit`（`Server` / `Router` / `Handler` / `@useAuth` 默认 fail-closed / `CORS` / 中间件 / 无状态编解码 / RFC 7386）+ `pkg/httperr`（RFC 9457 / 有限状态码 / AIP-193 Metadata）+ `internal/server` |
| Handler 接口风格 | Connect-Go v2 风格无泛型纯 Go 方法 `(ctx context.Context, req *api.Req) (*api.Resp, error)` |
| 依赖注入 | samber/do v2 |
| ORM | entgo v0.14 + SQLite（可换 PostgreSQL/MySQL）|
| 认证 | 短生命周期无状态 Access JWT（15m）+ 数据库托管 SHA-256 哈希 Refresh Token 轮换与重放检测 |
| 错误模型 | RFC 9457 Problem Details（`application/problem+json`，扁平强类型 + 终端统一 i18n）|
| 结构化日志 | log/slog + ContextHandler（顶层自动注入 `request_id`，`local` 文本 / 非本地 JSON）|
| ID 生成 | Go 标准库 `uuid`（UUID v7，时间有序）|
| 配置管理 | Viper（TOML + 环境变量，支持 `-config` 指定路径）|
| 外部 API 客户端 | 接口隔离 + 共享 HTTP 传输 + 指数退避带抖动重试（支持 `Retry-After`）|
| API 风格 | 现代务实 REST：POST/GET/PUT/PATCH/DELETE；`@autoPatch` 自动支持 RFC 7386 JSON Merge Patch |
| API 文档 | TypeSpec → OpenAPI 3.1 YAML → Stoplight Elements（`embed.FS`）|

## 项目结构

```text
api/
  spec/                 TypeSpec 契约单一事实来源（main.tsp、common/*、domains/*）
  emitters/go-rest/     自研 TypeSpec Go Emitter（同步生成 pkg/api/*.gen.go）
cmd/
  server/               应用入口（main.go，支持 -config 参数与优雅停机超时兜底）
config/                 应用配置定义与加载（Viper，config/config.toml）
ent/
  schema/               实体 schema 定义（改这里，然后 make ent）
  *.go                  ent 自动生成代码（不要手改）
pkg/                    纯公共、零 internal/ 与零三方依赖的可复用包
  api/                  TypeSpec 生成的平铺 DTO、Validate()、Handler 接口与 api.NewXxxHandler 适配器
  httpclient/           共享出站 HTTP 客户端连接池与内置超时/重试 RoundTripper（支持 Retry-After，零外部依赖）
  httperr/              13 个有限 HTTP 错误状态码哨兵（httperr.NotFound 等）、AIP-193 Metadata、纯 RFC 9457 Problem 与可插拔三段式 NewEncoder
  httpkit/              纯标准库 net/http 入站服务端原语套件（Server 优雅停机、Router 与 Static/File 静态资源挂载、Handler、@useAuth 默认 fail-closed 与 BearerAuth 鉴权、CORS、RequestID/AccessLog/Recover/BodyLimit 中间件、编解码、RFC 7386 Merge Patch）
  logging/              slog ContextHandler（跨 WithGroup 保持顶层 request_id，支持 Text/JSON 格式）
  validator/            强类型 Rule / Param、FieldViolations（含 Merge 嵌套路径合并）、声明式校验原语与通用 ParseOrderBy
internal/               应用私有实现
  auth/                 认证基础设施（不可变 Principal 上下文、无状态 Access JWT 签发/验签与 BearerAuthenticator）
  client/               第三方 API 客户端（每个 API 一个子包）
    <apiname>/
      interface.go      API interface + 响应 DTO（service 层唯一依赖）
      client.go         具体实现（从 DI 取 config + httpclient.Client）
      errors.go         oops 错误构造辅助
      package.go        do.Package(do.Lazy(NewClient), do.Bind[*Client, API]())
  database/             DB / ORM 基础设施（DI 生命周期，支持 sqlite/pgx/postgres/mysql）
  handler/              纯 Go 业务编排与 DTO 投影 Handler（Auth / Me / Books，彻底零 net/http 依赖）
  i18n/                 国际化翻译器（embed.FS TOML + Accept-Language 协商 + locale context）
  server/               极薄的 3 文件 HTTP 装配层（package.go 装配 httpkit.Server、router.go 路由/探针/CORS/文档装配、problem.go 胶水）
  service/              领域服务（Auth 凭证与哈希刷新令牌轮换、User 资料、Book 业务、分页规范化与 bcrypt 密码策略）
  testkit/              统一测试环境（DI 容器 + 独立内存 SQLite/配置文件初始化 + HTTP 请求/断言辅助）
static/                 Stoplight Elements + OpenAPI 3.1 YAML（embed.FS）
Dockerfile              多阶段无 CGo 精简容器构建（distroless nonroot）
.golangci.yml           克制的静态检查配置
.apitest/               IDE HTTP Client 测试文件
```

## 快速开始

```bash
# 1. 克隆并安装 Go 依赖
git clone https://github.com/liukeshao/go-rest-template
cd go-rest-template
go mod download

# 2. 启动（首次运行自动建表；运行时零 Node 依赖）
go run ./cmd/server
# 或显式指定配置文件路径
go run ./cmd/server -config config/config.toml

# 3. 健康探针与 API 文档（非生产环境）
curl http://localhost:8000/healthz
curl http://localhost:8000/readyz
open http://localhost:8000/docs

# 4. 如需修改 api/spec/*.tsp，安装一次 TypeSpec 开发依赖后运行 make api
pnpm --dir api install
make api
```

配置文件为 `config/config.toml`，所有字段均可通过环境变量覆盖，前缀 `GO_REST_TEMPLATE_`（集中定义在 `config.EnvPrefix`），例如：

```bash
GO_REST_TEMPLATE_AUTH_JWT_SECRET=mysecret go run ./cmd/server
```

## 基于本模板初始化新项目（清单）

1. **重命名模块与项目标识**（假设新项目名为 `my-service`）：
   - 修改 `go.mod` 与全仓 import：`github.com/liukeshao/go-rest-template` → `<your-module>`
   - 修改 `config/config.go` 顶部常量 `EnvPrefix`（`"GO_REST_TEMPLATE"` → `"MY_SERVICE"`）与 `config/config.toml` 的 `app.name` / `auth.jwt.issuer`
   - 修改 `api/tspconfig.yaml` 中的 `runtime-import` / `validator-import`、`api/spec/main.tsp` 的 `@service` 与 `namespace`、`api/package.json` 的 `name` 及 `static/index.html` 的 `<title>`
   - 运行 `make api && make ent && make test` 确认全部通过
2. **按需清理示例业务**：
   - 如不需要 `books` CRUD 示例：删除 `api/spec/domains/books.tsp`（及 `main.tsp` 中的 import）、`ent/schema/book.go`、`internal/service/book*.go`、`internal/handler/books*.go`、取消 `router.go` 挂载并清理 `en.toml`/`zh-CN.toml` 中 `book` 词条，再执行 `make api && make ent`
   - 如不需要 `weather` 外部客户端示例：删除 `internal/client/weather/`、`internal/client/package.go` 中注册、`config/config.go` & `config/config.toml` 中 `weather` 项、`internal/testkit/env.go` 中默认值及 `en.toml`/`zh-CN.toml` 中 `[details.429] weather`

## API 风格与 PATCH（RFC 7386）

资源以集合 URL 暴露，HTTP 方法表达动作；自定义动词作为 kebab-case 子路径段挂在资源下。

| 方法    | 用途                              | 幂等   | 请求体                                                         |
|---------|-----------------------------------|--------|----------------------------------------------------------------|
| POST    | 创建 / 非幂等自定义动词           | 否     | `CreateXxxRequest`（或自定义动词 DTO）                         |
| GET     | 读取单资源或集合                  | 是     | 无                                                             |
| PUT     | 全量替换（覆盖所有可写字段）      | 是     | `ReplaceXxxRequest`                                            |
| PATCH   | 局部更新（RFC 7386 Merge Patch）  | 是     | `PatchXxxRequest`（省略字段保持原值，`null` 清空可选字段）     |
| DELETE  | 删除                              | 是     | 无                                                             |

在 TypeSpec 中为 `patch` 操作标注 `@autoPatch(Domain.getXxx, Domain.replaceXxx)` 后：
- OpenAPI 3.1 会记录强类型的 `PatchXxxRequest` Schema 及 `415 Unsupported Media Type`。
- 生成的 `api.<Domain>Handler` 接口**无需**手写 `Patch` 方法，`api.New<Domain>Handler` 会自动先通过 `httpkit.ParsePatchRequest` 校验 Content-Type 与 JSON 语法（fail-fast，避免无谓查库），再串联 `Get → patchDoc.Apply → Validate → Replace`。

| Content-Type                   | 主体格式                                 | 示例                                      |
|--------------------------------|------------------------------------------|-------------------------------------------|
| `application/merge-patch+json` | RFC 7386：JSON 对象，`null` 表示清空字段 | `{"title":"New","price":null}`            |
| `application/json` / 缺省      | 兼容写法：按 Merge Patch 处理            | `{"title":"New"}`                         |

## 增加一个新领域（Checklist）

以增加 `Order` 领域为例：

**1. TypeSpec 契约**（`api/spec/domains/orders.tsp` + 在 `api/spec/main.tsp` 中 `import`）

```tsp
model Order {
  id: UUID;
  title: string;
  amount?: float64;
  created_at: utcDateTime;
  updated_at: utcDateTime;
}

model OrderList is Page<Order>;

model CreateOrderRequest {
  order_id?: UUID;
  @minLength(1) @maxLength(200) title: string;
  @minValue(0) amount?: float64;
}

model ReplaceOrderRequest {
  @minLength(1) @maxLength(200) title: string;
  @minValue(0) amount?: float64;
}

model PatchOrderRequest {
  @minLength(1) @maxLength(200) title?: string;
  @minValue(0) amount?: float64 | null;
}

@route("/api/v1/orders")
@tag("orders")
@useAuth(BearerAuth)
interface Orders {
  @operationId("createOrder") @post createOrder(@body body: CreateOrderRequest): { @statusCode statusCode: 201; @body body: Order } | BadRequestProblem | UnauthorizedProblem | ConflictProblem;
  @operationId("listOrders") @get listOrders(...ListQuery): OrderList | BadRequestProblem | UnauthorizedProblem;
  @operationId("getOrder") @get @route("/{order_id}") getOrder(@path order_id: UUID): Order | UnauthorizedProblem | NotFoundProblem;
  @operationId("replaceOrder") @put @route("/{order_id}") replaceOrder(@path order_id: UUID, @body body: ReplaceOrderRequest): Order | BadRequestProblem | UnauthorizedProblem | NotFoundProblem;
  @operationId("patchOrder") @autoPatch(Orders.getOrder, Orders.replaceOrder) @patch @route("/{order_id}") patchOrder(@path order_id: UUID, @header contentType: "application/merge-patch+json" | "application/json", @body body: PatchOrderRequest): Order | BadRequestProblem | UnauthorizedProblem | NotFoundProblem | UnsupportedMediaTypeProblem;
  @operationId("deleteOrder") @delete @route("/{order_id}") deleteOrder(@path order_id: UUID): { @statusCode statusCode: 204 } | UnauthorizedProblem | NotFoundProblem;
}
```

编译生成 OpenAPI 3.1 与 Go 协议代码：
```bash
make api
```

**2. Ent Schema**（`ent/schema/order.go`，使用 `DefaultMixin`）

```bash
make ent
```

**3. 按需补充细化国际化文案**（`internal/i18n/messages/{en,zh-CN}.toml`，无需注册新错误码）

```toml
[details.404]
order = "未找到 ID 为 {resource_name} 的订单。"

[details.409]
order.order_id = "订单 {resource_name} 已存在，请换一个 order_id。"
```

**4. Service**（`internal/service/order.go` + 注册到 `internal/service/package.go`）

实现 `Create` / `Get` / `List` / `Replace` / `Delete`（返回 `*ent.Order`），各资源的列表查询条件直接按对应 `ListXxxRequest` 字段构造 ent predicate。

**5. Handler & 路由挂载**（`internal/handler/orders.go` + `internal/handler/package.go` + `internal/server/router.go`）

实现生成的 `api.OrdersHandler` 纯 Go 接口（编排多 Service + `toOrder` 投影，无需依赖 `net/http` 或写 `Mount` 方法）；在 `internal/handler/package.go` 注册 `do.Lazy(NewOrders)`，在 `internal/server/router.go` 中挂载 `api.NewOrdersHandler(do.MustInvoke[*handler.Orders](i), opts...)`。

## 错误处理约定

全仓采用 **纯 RFC 9457 + 13 个有限 HTTP 错误状态码（`httperr.Status`，底层即 `int` HTTP 状态码且自身实现 `errors.Is` 哨兵）+ 原生 `oops` + AIP-193 强类型 `httperr.Metadata` 结构体**，无冗余 `code` 字段、**禁止使用 `.Public(...)`**，领域服务与外部客户端只依赖轻量的 `pkg/httperr`，由终端 `server.NewProblemEncoder` 统一做分层国际化渲染：

```go
var errOrder = oops.In("order")

// 领域内部错误（5xx：With 上下文仅打入 slog 日志，不对外暴露；Join 保留哨兵与根因链）
errOrder.Tags("database").With("order_id", id, "op", op).Join(httperr.Internal, err)

// 客户端错误（4xx：Wrap 哨兵作为唯一状态码事实来源并支持 errors.Is，仅 httperr.MetaKey 下实现 httperr.Metadata 的结构体对外暴露并供给分层 i18n）
errOrder.
    With(httperr.MetaKey, httperr.ResourceInfo{
        ResourceType: "order",
        ResourceName: id,
    }).
    Wrap(httperr.NotFound)
```

`server.NewProblemEncoder` 自动按 `Classify（优先识别请求 Context 取消/超时）→ Sanitize & Log → Unified Localize（[details.<STATUS>] → [errors] + Content-Language / Retry-After / WWW-Authenticate 头）` 流水线输出 RFC 9457 `application/problem+json` 响应，无需在 handler 层手动处理。

## 命令

```bash
make api               # 编译 TypeSpec → OpenAPI 3.1 + pkg/api/*.gen.go
make ent               # 重新生成 ent ORM 代码
make test              # 运行测试（-race）
make cover             # 统计业务包单元测试覆盖率
make lint              # 运行 golangci-lint 静态检查
make build             # 编译
```
