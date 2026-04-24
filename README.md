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
| 外部 API 客户端 | 接口隔离 + 共享 HTTP 传输 + 线性退避重试 |
| API 风格 | AIP-131 Get · AIP-132 List（分页/排序/过滤）· AIP-133 Create · AIP-134 Update · AIP-135 Delete · AIP-160 Filter |
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
  handlers/             HTTP handler + router + error_handler
  infra/                DB / ORM / 共享 HTTPClient 基础设施（DI 生命周期）
  log/                  slog ContextHandler（日志自动带 request_id）
  middleware/            RequireAuth、RequestContext
  services/             领域服务（业务逻辑）
    <domain>_filter.go  AIP-160 过滤器 → ent predicate 翻译（每个领域一个文件）
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

## 增加一个新领域（Checklist）

以增加 `Order` 领域为例：

**1. ent schema**

```bash
# 新建 ent/schema/order.go
# 参考 ent/schema/book.go，使用 DefaultMixin（含 ULID 主键、软删除、时间戳）
cd ent && go generate   # 重新生成 ORM 代码
```

**2. 错误码**（`pkg/codes/order.go`）

```go
const (
    OrderNotFound     Code = "ORDER_NOT_FOUND"
    OrderAlreadyExists Code = "ORDER_ALREADY_EXISTS"  // AIP-133
    OrderInternal     Code = "ORDER_INTERNAL_ERROR"
)

func init() {
    Register(OrderNotFound,      rpccodes.NotFound)
    Register(OrderAlreadyExists, rpccodes.AlreadyExists)
    Register(OrderInternal,      rpccodes.Internal)
}
```

**3. DTO**（`pkg/types/order.go`）

```go
// AIP-133 Create：order_id 可选；提供则用作资源 ID（ULID），省略则自动生成
type CreateOrderRequest struct {
    OrderID string `json:"order_id,omitempty"`
    // ...
}
func (r *CreateOrderRequest) Validate() error { ... }

type OrderResponse struct { ... } // 不含 deleted_at 等内部字段

// AIP-131 Get / AIP-135 Delete：路径参数绑定到专属结构体，禁止 c.Param 直接调用
type GetOrderRequest    struct { Order string `param:"order"` }
type DeleteOrderRequest struct { Order string `param:"order"` }

// AIP-134 Update：PATCH + 字段掩码
type UpdateOrderRequest struct {
    Order      string   `param:"order"`
    UpdateMask []string `json:"update_mask,omitempty"`
    // 可更新字段均为指针类型（nil = 未提供）
}
func (r *UpdateOrderRequest) Validate() error { ... }

// AIP-132 List：分页 + 排序 + 过滤
type ListOrdersRequest struct {
    PageSize  int32  `query:"page_size"`
    PageToken string `query:"page_token"`
    OrderBy   string `query:"order_by"`
    Filter    string `query:"filter"`
}

type ListOrdersResponse struct {
    Orders        []OrderResponse `json:"orders"`
    NextPageToken string          `json:"next_page_token,omitempty"`
    TotalSize     int32           `json:"total_size"`
}
```

**4. Service**（`pkg/services/order.go`）

```go
type OrderService struct { orm *ent.Client }

func NewOrderService(i do.Injector) (*OrderService, error) { ... }

// Create（AIP-133）：优先使用 req.OrderID，否则自动生成；ent.IsConstraintError → OrderAlreadyExists
func (s *OrderService) Create(ctx context.Context, req *types.CreateOrderRequest) (*ent.Order, error) { ... }

// Update（AIP-134）：委托给 applyOrderUpdateMask(q, req) 实现三路 mask 逻辑
func (s *OrderService) Update(ctx context.Context, id string, req *types.UpdateOrderRequest) (*ent.Order, error) { ... }
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
type OrderHandler struct { order *services.OrderService; auth *services.AuthService }

func NewOrderHandler(i do.Injector) (*OrderHandler, error) { ... }
func (h *OrderHandler) Routes(g *echo.Group) { ... }
```

注册到 `pkg/handlers/package.go` 和 `BuildRouter`。

**6. 路由命名规范**（AIP-131/134/135）

路由路径参数以资源类型单数命名，而非 `:id` 或 `:name`；更新使用 `PATCH`（非 `PUT`）：

```go
g.POST("",         h.Create)
g.GET("",          h.List)
g.GET("/:order",   h.Get)
g.PATCH("/:order", h.Update)   // AIP-134：必须用 PATCH
g.DELETE("/:order", h.Delete)
```

所有含路径参数的 handler 均通过专属请求结构体绑定，**禁止** `c.Param("order")` 直接调用：

```go
// Get / Delete
var req types.GetOrderRequest   // 或 DeleteOrderRequest
c.Bind(&req)
// req.Order 即路径参数值

// Update
var req types.UpdateOrderRequest
c.Bind(&req)   // 同时绑定路径参数和 JSON 体（含 update_mask）
```

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
    Stripe  ExternalClientConfig  // 新增
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
    stripe.Package,  // 追加
)
```

**5. Service 层使用**

Service 字段类型为 interface，不依赖 `*Client`：

```go
type PaymentService struct {
    stripe stripeapi.API   // interface
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
cd ent && go generate  # 重新生成 ent ORM 代码
go test ./...          # 运行测试
go build ./...         # 编译
```
