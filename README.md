# echo-template

基于 **TypeSpec + 自研 Go Emitter + Go 1.22+ 标准库 `net/http`** 的现代 Go RESTful API 项目模板（Connect-Go v2 风格纯业务接口），集成了生产就绪的 RFC 9457 错误处理、i18n、JWT 认证、Ent ORM、RFC 7386 JSON Merge Patch 与可观测性基础设施。

## 技术栈

| 分层 | 选型 |
|---|---|
| API 契约 & 代码生成 | TypeSpec 1.18 + `@typespec/openapi3` (OpenAPI 3.1) + 自研 `go-rest` Emitter |
| HTTP 运行时 | Go 1.22+ 标准库 `net/http` (`*http.ServeMux`) + 零依赖 `pkg/rest`（`Router` / `Handler` / `ErrorEncoder` / 无状态编解码 / RFC 9457 / RFC 7386）+ `internal/server` |
| Handler 接口风格 | Connect-Go v2 风格无泛型纯 Go 方法 `(ctx context.Context, req *api.Req) (*api.Resp, error)` |
| 依赖注入 | samber/do v2 |
| ORM | entgo v0.14 + SQLite（可换 PostgreSQL/MySQL）|
| 认证 | JWT（access + refresh token，数据库存储）|
| 错误模型 | RFC 9457 Problem Details（`application/problem+json`，扁平强类型 + i18n）|
| 结构化日志 | log/slog + ContextHandler（自动注入 `request_id`）|
| ID 生成 | Go 标准库 `uuid`（UUID v7，时间有序）|
| 配置管理 | Viper（TOML + 环境变量）|
| 外部 API 客户端 | 接口隔离 + 共享 HTTP 传输 + 指数退避带抖动重试 |
| API 风格 | 现代务实 REST：POST/GET/PUT/PATCH/DELETE；`@autoPatch` 自动支持 RFC 7386 JSON Merge Patch |
| API 文档 | TypeSpec → OpenAPI 3.1 YAML → Stoplight Elements（`embed.FS`）|

## 项目结构

```text
api/
  spec/                 TypeSpec 契约单一事实来源（main.tsp、common/*、domains/*）
  emitters/go-rest/     自研 TypeSpec Go Emitter（同步生成 pkg/api/*.gen.go）
cmd/
  server/               应用入口（main.go）
config/                 配置定义与加载（Viper）
ent/
  schema/               实体 schema 定义（改这里，然后 make ent）
  *.go                  ent 自动生成代码（不要手改）
pkg/                    纯公共、零 internal/ 与零三方依赖的可复用包
  api/                  TypeSpec 生成的平铺 DTO、Validate()、Handler 接口与 api.NewXxxHandler 适配器
  httpclient/           共享 HTTPClient 连接池与内置超时/重试 RoundTripper（通过 Config/WithOptions 参数注入，零外部依赖）
  logging/              slog ContextHandler（日志自动带 request_id + request_id context，零外部依赖）
  rest/                 纯标准库 net/http REST 原语（Router、Handler、有限标准 HTTP 错误码 rest.Code、AIP-193 Metadata、RFC 9457 Problem、RFC 7386 Merge Patch）
  validator/            FieldViolations + 原子校验规则（含 UUID / Email / Username / Password）
internal/               应用私有实现
  client/               第三方 API 客户端（每个 API 一个子包）
    <apiname>/
      interface.go      API interface + 响应 DTO（service 层唯一依赖）
      client.go         具体实现（从 DI 取 config + httpclient.Client）
      errors.go         oops 错误构造辅助
      package.go        do.Package(do.Lazy(NewClient), do.Bind[*Client, API]())
  database/             DB / ORM 基础设施（DI 生命周期）
  handler/              纯 Go 业务编排与 DTO 投影 Handler（Auth / Me / Book，彻底零 net/http 依赖）
  i18n/                 国际化翻译器（embed.FS TOML + Accept-Language 协商 + locale context）
  server/               HTTP 传输装配层（Server 优雅停机、NewProblemEncoder、RequestID/Locale 中间件、BuildRouter、BearerAuth、Docs）
  service/              领域服务（业务逻辑、分页规范化、bcrypt 密码策略与各自资源的 ent 查询条件）
  testutil/             统一测试环境（DI 容器 + 独立内存 SQLite + HTTP 请求/断言辅助）
static/                 Stoplight Elements + OpenAPI 3.1 YAML（embed.FS）
.apitest/               IDE HTTP Client 测试文件
```

## 快速开始

```bash
# 1. 克隆并安装 Go 依赖
git clone https://github.com/liukeshao/echo-template
cd echo-template
go mod download

# 2. 启动（首次运行自动建表；运行时零 Node 依赖）
go run ./cmd/server

# 3. 查看 API 文档（非生产环境）
open http://localhost:8000/docs

# 4. 如需修改 api/spec/*.tsp，安装一次 TypeSpec 开发依赖后运行 make api
pnpm --dir api install
make api
```

配置文件为 `config/config.toml`，所有字段均可通过环境变量覆盖，前缀 `ECHO_TEMPLATE_`，例如：

```bash
ECHO_TEMPLATE_JWT_SECRET=mysecret go run ./cmd/server
```

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
- 生成的 `api.<Domain>Handler` 接口**无需**手写 `Patch` 方法，`api.New<Domain>Handler` 会自动串联 `Get → rest.ApplyPatch → Validate → Replace`。

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
  @operationId("getOrder") @get @route("/{order}") getOrder(@path order: UUID): Order | UnauthorizedProblem | NotFoundProblem;
  @operationId("replaceOrder") @put @route("/{order}") replaceOrder(@path order: UUID, @body body: ReplaceOrderRequest): Order | BadRequestProblem | UnauthorizedProblem | NotFoundProblem;
  @operationId("patchOrder") @autoPatch(Orders.getOrder, Orders.replaceOrder) @patch @route("/{order}") patchOrder(@path order: UUID, @header contentType: "application/merge-patch+json" | "application/json", @body body: PatchOrderRequest): Order | BadRequestProblem | UnauthorizedProblem | NotFoundProblem | UnsupportedMediaTypeProblem;
  @operationId("deleteOrder") @delete @route("/{order}") deleteOrder(@path order: UUID): { @statusCode statusCode: 204 } | UnauthorizedProblem | NotFoundProblem;
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
[details.NOT_FOUND]
order = "未找到 ID 为 {resource_name} 的订单。"

[details.CONFLICT]
order.order_id = "订单 {resource_name} 已存在，请换一个 order_id。"
```

**4. Service**（`internal/service/order.go` + 注册到 `internal/service/package.go`）

实现 `Create` / `Get` / `List` / `Replace` / `Delete`（返回 `*ent.Order`），各资源的列表查询条件直接按对应 `ListXxxRequest` 字段构造 ent predicate。

**5. Handler & 路由挂载**（`internal/handler/order.go` + `internal/handler/package.go` + `internal/server/router.go`）

实现生成的 `api.OrdersHandler` 纯 Go 接口（编排多 Service + `toOrder` 投影，无需依赖 `net/http` 或写 `Mount` 方法）；在 `internal/handler/package.go` 注册 `do.Lazy(NewOrder)`，在 `internal/server/router.go` 中挂载 `api.NewOrdersHandler(do.MustInvoke[*handler.Order](i), opts...)`。

## 错误处理约定

全仓采用 **13 个有限标准 HTTP 错误码（`rest.Code`，自身即不可变 `errors.Is` 哨兵）+ 原生 `oops` + AIP-193 强类型 `rest.Metadata` 结构体**，**禁止使用 `.Public(...)`**，由终端 `server.NewProblemEncoder` 统一做分层国际化渲染：

```go
var errOrder = oops.In("order")

// 领域内部错误（5xx：With 上下文仅打入 slog 日志，不对外暴露；Join 保留哨兵与根因链）
errOrder.Tags("database").Code(rest.CodeInternal).With("order_id", id, "op", op).Join(rest.CodeInternal, err)

// 客户端错误（4xx：Wrap 哨兵支持 errors.Is，仅 rest.MetaKey 下实现 rest.Metadata 的结构体对外暴露并供给分层 i18n）
errOrder.Code(rest.CodeNotFound).
    With(rest.MetaKey, rest.ResourceInfo{
        ResourceType: "order",
        ResourceName: id,
    }).
    Wrap(rest.CodeNotFound)
```

`server.NewProblemEncoder` 自动按 `Classify（优先识别请求 Context 取消/超时）→ Sanitize & Log → Unified Localize（[details.<CODE>] → [errors] + Content-Language / Retry-After 头）` 流水线输出 RFC 9457 `application/problem+json` 响应，无需在 handler 层手动处理。

## 命令

```bash
make api               # 编译 TypeSpec → OpenAPI 3.1 + pkg/api/*.gen.go
make ent               # 重新生成 ent ORM 代码
make test              # 运行测试（-race）
make cover             # 统计业务包单元测试覆盖率
make build             # 编译
```
