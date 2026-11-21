# echo-template

基于 [Echo v5](https://github.com/labstack/echo) 的 Go RESTful API 项目模板，集成了生产就绪的错误处理、认证、ORM 和可观测性基础设施。

## 技术栈

| 分层 | 选型 |
|---|---|
| HTTP 框架 | Echo v5 |
| 依赖注入 | samber/do v2 |
| ORM | entgo v0.14 + SQLite（可换 PostgreSQL/MySQL）|
| 认证 | JWT（access + refresh token，数据库存储）|
| 错误模型 | AIP-193（google.rpc.Code + errdetails）|
| 结构化日志 | log/slog + ContextHandler（自动注入 request_id）|
| ID 生成 | ULID |
| 配置管理 | Viper（TOML + 环境变量）|
| 外部 API 客户端 | 接口隔离 + 共享 HTTP 传输 + 指数退避带抖动重试 |
| API 风格 | Kubernetes 式 REST：POST/GET/PUT/PATCH/DELETE；多协议 PATCH（RFC 6902 / RFC 7386 / Shorthand）；自定义动词以 `/:resource/<verb>` 形式挂在资源下 |
| API 文档 | Swaggo → OpenAPI YAML → Stoplight Elements |

## 项目结构

```
cmd/                    应用入口（main.go）
config/                 配置定义与加载（Viper）
ent/
  schema/               实体 schema 定义（改这里，然后 go generate）
  *.go                  ent 自动生成代码（不要手改）
pkg/
  appctx/               context 工具（user、request_id）
  clients/              第三方 API 客户端（每个 API 一个子包）
    <apiname>/
      interface.go      API interface + 响应 DTO（service 层唯一依赖）
      client.go         具体实现（从 DI 取 config + HTTPClient）
      errors.go         oops 错误构造辅助
      package.go        do.Package(do.Lazy(NewXxxClient))
  codes/                业务错误码 + RPC 状态码注册表
  filter/               AIP-160 过滤表达式解析器 + 泛型 FilterTranslator[P]
  handlers/             HTTP handler + router + error_handler + pagination
  i18n/                 国际化翻译器（embed.FS TOML + Accept-Language 协商）
  infra/                DB / ORM / 共享 HTTPClient 基础设施（DI 生命周期）
  log/                  slog ContextHandler（日志自动带 request_id）
  meta/v1/              通用查询选项（ListOptions）
  middleware/           RequireAuth、RequestContext、Locale
  patch/                多协议 PATCH 实现（RFC 6902 / 7386 / Shorthand）
  services/             领域服务（业务逻辑）
    <domain>_filter.go  AIP-160 过滤器 → ent predicate 翻译（每个领域一个文件）
  testutil/             统一测试环境（DI 容器 + 独立内存 SQLite + HTTP 请求/断言辅助）
  types/                Request / Response DTO + Validate()
  utils/                password（bcrypt cost=12）、ULID、Retry
  validator/            FieldViolations + 原子校验规则
transport/
  rpc/codes             google.rpc.Code 定义
  rpc/status            不可变 Status（code + message + details）
  rpc/errdetails        AIP-193 detail payload 类型
  http/status           rpc.Code ↔ HTTP 状态码双向映射
static/                 Stoplight Elements + OpenAPI YAML（embed.FS）
.apitest/               IDE HTTP Client 测试文件
```

## 快速开始

```bash
# 1. 克隆并安装依赖
git clone https://github.com/liukeshao/echo-template
cd echo-template
go mod download

# 2. 启动（首次运行自动建表）
go run ./cmd/

# 3. 查看 API 文档（非生产环境）
open http://localhost:8000/docs
```

配置文件为 `config/config.toml`，所有字段均可通过环境变量覆盖，前缀 `ECHO_TEMPLATE_`，例如：

```bash
ECHO_TEMPLATE_JWT_SECRET=mysecret go run ./cmd/
```

## API 风格 — Kubernetes 式 REST

资源以集合 URL 暴露，HTTP 方法表达动作；自定义动词作为子路径段挂在资源下。**没有** AIP-134 的 `update_mask`；局部更新统一通过 PATCH + Content-Type 选择三种 patch 协议之一。

### 方法语义

| 方法    | 用途                              | 幂等 | 请求体                                                              |
|---------|-----------------------------------|------|---------------------------------------------------------------------|
| POST    | 创建 / 非幂等自定义动词           | 否   | `CreateXxxRequest`（或自定义动词的专属 DTO）                        |
| GET     | 读取单资源或集合                  | 是   | 无                                                                  |
| PUT     | 全量替换（覆盖所有可写字段）      | 是   | `ReplaceXxxRequest`                                                 |
| PATCH   | 局部更新（按 Content-Type 分派）  | 视协议 | RFC 6902 / RFC 7386 / Shorthand                                   |
| DELETE  | 删除                              | 是   | 无                                                                  |

单例资源（如 `/api/v1/me`，隐式绑定到当前用户）只有 GET / PUT / PATCH 与自定义动词，没有 POST / DELETE。

### PATCH 多协议

PATCH handler 根据 `Content-Type` 选择 patch 算法，然后走"GET → apply → Replace"流程把结果写回。所有协议最终共享同一份服务端校验与持久化逻辑。

| Content-Type                              | 主体格式                                                 | 示例                                                                 |
|-------------------------------------------|----------------------------------------------------------|----------------------------------------------------------------------|
| `application/json-patch+json`             | RFC 6902：`{op,path,value}` 数组                          | `[{"op":"replace","path":"/title","value":"New"}]`                  |
| `application/merge-patch+json`            | RFC 7386：JSON 对象；`null` 表示删除字段                  | `{"title":"New","price":null}`                                       |
| `application/merge-patch+shorthand`       | Shorthand DSL（可省略外层 `{}`），叠加到原资源            | `title: "New", price: 19.99`                                         |
| `application/json` / 缺省                 | 兼容：按 Merge Patch 处理                                 | `{"title":"New"}`                                                     |

### 路由示例

```go
g.POST("",                  h.Create)
g.GET("",                   h.List)
g.GET("/:book",             h.Get)
g.PUT("/:book",             h.Replace)        // 幂等全量替换
g.PATCH("/:book",           h.Patch)          // RFC 6902 / 7386 / Shorthand
g.DELETE("/:book",          h.Delete)

// 自定义动词（K8s 子资源风格）：动词作为资源后的 path 段
g.POST("/:book/archive",    h.Archive)        // 资源级动作
g.POST("/batch-get",        h.BatchGet)       // 集合级动作
```

约束：
- 路径参数命名为资源类型的单数（`/:book` 而不是 `/:id`）。
- 自定义动词使用 **kebab-case**（`reset-password`、`batch-get`），不要用 camelCase。
- 幂等状态转移用 `PUT`，非幂等动作用 `POST`，子状态读取用 `GET`。

## 增加一个新领域（Checklist）

以增加 `Order` 领域为例：

**1. ent schema**

```bash
# 新建 ent/schema/order.go
# 参考 ent/schema/book.go，使用 DefaultMixin（含 ULID 主键、软删除、时间戳）
make ent
```

**2. 错误码**（`pkg/codes/order.go`）

```go
const (
    OrderNotFound      Code = "ORDER_NOT_FOUND"
    OrderAlreadyExists Code = "ORDER_ALREADY_EXISTS"
    OrderInternal      Code = "ORDER_INTERNAL_ERROR"
)

func init() {
    Register(OrderNotFound,      rpccodes.NotFound)
    Register(OrderAlreadyExists, rpccodes.AlreadyExists)
    Register(OrderInternal,      rpccodes.Internal)
}
```

**3. DTO**（`pkg/types/order.go`）

```go
// 创建：order_id 可选；提供则用作资源 ID（ULID），省略则自动生成
type CreateOrderRequest struct {
    OrderID string `json:"order_id,omitempty"`
    // ...
}
func (r *CreateOrderRequest) Validate() error { ... }

// 响应 DTO，不含 deleted_at / 密码 / 内部状态等字段；
// 同时作为 PATCH "read-modify-write" 流程中的 JSON 中间表示
type OrderResponse struct { ... }

// 单资源读 / 删
type GetOrderRequest    struct { Order string `param:"order"` }
type DeleteOrderRequest struct { Order string `param:"order"` }

// 仅用作 PATCH handler 的路径参数容器（不绑定 body）
type PatchOrderRequest  struct { Order string `param:"order"` }

// PUT：全量替换（所有可写字段；可选字段未提供视为清除）
type ReplaceOrderRequest struct {
    Order  string `param:"order" json:"-"`
    Title  string `json:"title"`
    Amount float64 `json:"amount,omitempty"`
    // ...
}
func (r *ReplaceOrderRequest) Validate() error { ... }

// 列表
type ListOrdersRequest struct {
    Page     int32  `query:"page"`
    PageSize int32  `query:"page_size"`
    OrderBy  string `query:"order_by"`
    Filter   string `query:"filter"`
}

type ListOrdersResponse struct {
    Orders    []OrderResponse `json:"orders"`
    Page      int32           `json:"page"`
    PageSize  int32           `json:"page_size"`
    TotalSize int32           `json:"total_size"`
}
```

**4. Service**（`pkg/services/order.go`）

```go
type OrderService struct { orm *ent.Client }

func NewOrderService(i do.Injector) (*OrderService, error) { ... }

// Get / List / Delete / Create / Replace
// 注意：service 层没有 Patch 方法；多协议 PATCH 在 handler 层组合 Get + Replace 完成。
func (s *OrderService) Create(ctx context.Context, req *types.CreateOrderRequest) (*ent.Order, error) { ... }
func (s *OrderService) Replace(ctx context.Context, id string, req *types.ReplaceOrderRequest) (*ent.Order, error) { ... }
```

注册到 `pkg/services/package.go`：
```go
do.Lazy(NewOrderService),
```

**4.5 过滤器翻译**（`pkg/services/order_filter.go`）

为 List 方法提供 AIP-160 过滤器到 ent predicate 的翻译，使用 `pkg/filter.FilterTranslator[P]`：

```go
var orderFilter = filter.NewTranslator(order.And, order.Or, order.Not).
    String("status",
        order.StatusEQ, order.StatusNEQ,
        order.StatusLT, order.StatusLTE, order.StatusGT, order.StatusGTE,
        order.StatusContainsFold, order.StatusHasPrefix, order.StatusHasSuffix,
    ).
    Float64("amount",
        order.AmountEQ, order.AmountNEQ,
        order.AmountLT, order.AmountLTE, order.AmountGT, order.AmountGTE,
    )

func translateOrderFilter(expr filter.Expr) (predicate.Order, error) {
    return orderFilter.Translate(expr)
}
```

参考 `pkg/services/book_filter.go`。

**5. Handler**（`pkg/handlers/order.go`）

```go
type OrderHandler struct {
    order *services.OrderService
}

func NewOrderHandler(i do.Injector) (*OrderHandler, error) { ... }

func (h *OrderHandler) Routes(g *echo.Group, auth echo.MiddlewareFunc) {
    r := g.Group("/orders", auth)

    r.POST("",          h.Create)
    r.GET("",           h.List)
    r.GET("/:order",    h.Get)
    r.PUT("/:order",    h.Replace)
    r.PATCH("/:order",  h.Patch)
    r.DELETE("/:order", h.Delete)
}
```

PATCH handler 直接复用 `pkg/patch` 提供的 `patch.ApplyInto`；具体实现见 `pkg/handlers/book.go` 的 `Patch` 方法。

注册到 `pkg/handlers/package.go`（`BuildRouter` 会自动挂载，无需修改 `router.go`）：

```go
provideHandler(NewOrderHandler),
```

**6. 自定义方法（资源动作）**

K8s 子资源风格：动词作为 kebab-case 路径段挂在资源后。

```go
type CancelOrderRequest struct {
    Order  string `param:"order"`
    Reason string `json:"reason,omitempty"`
}

protected.POST("/:order/cancel", h.Cancel)
```

handler 方法名为动词的 PascalCase 形式（`Cancel`、`Refund`、`ResetPassword`）。

**7. 更新 OpenAPI 注解**

```bash
make swag
```

## 错误处理约定

所有 service 层错误通过 `oops` 构造，格式固定：

```go
// 领域内部错误（对外隐藏细节）
oops.Code(codes.OrderInternal.String()).Tags("order").Wrapf(err, "%s", op)

// 资源未找到（携带 AIP-193 ResourceInfo）
oops.Code(codes.OrderNotFound.String()).
    With(codes.CtxKeyResourceInfo, errdetails.ResourceInfo{...}).
    Public(fmt.Sprintf("Order %q not found.", id)).
    Errorf("order not found")
```

`error_handler` 会自动将 oops 错误转换为 AIP-193 JSON 响应，无需在 handler 层手动处理。

## 增加一个新的第三方 API 对接（Checklist）

以接入 `Stripe` 支付 API 为例：

**1. 配置**（`config/config.go` + `config/config.toml`）

```go
// config/config.go
type ClientsConfig struct {
    Weather ExternalClientConfig
    Stripe  ExternalClientConfig
}
```

```toml
# config/config.toml
[clients.stripe]
baseURL = "https://api.stripe.com/v1"
apiKey  = "sk_test_..."
timeout = "10s"
retry   = 1
```

**2. 错误码**（`pkg/codes/stripe.go`）

```go
const (
    StripeUnavailable Code = "STRIPE_UNAVAILABLE"
    StripeRateLimited Code = "STRIPE_RATE_LIMITED"
    StripeInternal    Code = "STRIPE_INTERNAL_ERROR"
)

func init() {
    Register(StripeUnavailable, rpccodes.Unavailable)
    Register(StripeRateLimited, rpccodes.ResourceExhausted)
    Register(StripeInternal,    rpccodes.Internal)
}
```

**3. 客户端包**（`pkg/clients/stripe/`）

复制 `pkg/clients/weather/` 为起点，修改：
- `interface.go` — 定义 `API` interface 和响应 DTO
- `client.go` — 实现 API 调用（从 DI 取 `config.Config.Clients.Stripe` 和 `*infra.HTTPClient`）
- `errors.go` — `stripeUnavailable`、`stripeRateLimited`、`stripeInternal` 三个辅助函数
- `package.go` — `do.Package(do.Lazy(NewStripeClient))`

**4. 注册到 DI**（`pkg/clients/package.go`）

```go
var Package = do.Package(
    weather.Package,
    stripe.Package,
)
```

**5. Service 层使用**

Service 字段类型为 interface，不依赖 `*Client`：

```go
type PaymentService struct {
    stripe stripeapi.API
}

func NewPaymentService(i do.Injector) (*PaymentService, error) {
    return &PaymentService{
        stripe: do.MustInvoke[*stripeapi.Client](i),
    }, nil
}
```

---

## 命令

```bash
make swag              # 生成 OpenAPI YAML
make ent               # 重新生成 ent ORM 代码
make test              # 运行测试
make build             # 编译
```
