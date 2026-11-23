# echo-template

基于 **TypeSpec + 自研 Go Emitter + Go 1.22+ 标准库 `net/http`** 的现代 Go RESTful API 项目模板（Connect-Go v2 风格纯业务接口），集成了生产就绪的 RFC 9457 错误处理、i18n、JWT 认证、Ent ORM、多协议 PATCH 与可观测性基础设施。

## 技术栈

| 分层 | 选型 |
|---|---|
| API 契约 & 代码生成 | TypeSpec 1.18 + `@typespec/openapi3` (OpenAPI 3.1) + 自研 `go-rest` Emitter |
| HTTP 运行时 | Go 1.22+ 标准库 `net/http` (`*http.ServeMux`) + 极薄 `pkg/httpruntime`（零第三方 Web 框架）|
| Handler 接口风格 | Connect-Go v2 风格无泛型纯 Go 方法 `(ctx context.Context, req *api.Req) (*api.Resp, error)` |
| 依赖注入 | samber/do v2 |
| ORM | entgo v0.14 + SQLite（可换 PostgreSQL/MySQL）|
| 认证 | JWT（access + refresh token，数据库存储）|
| 错误模型 | RFC 9457 Problem Details（`application/problem+json`，扁平强类型 + i18n）|
| 结构化日志 | log/slog + ContextHandler（自动注入 `request_id`）|
| ID 生成 | ULID |
| 配置管理 | Viper（TOML + 环境变量）|
| 外部 API 客户端 | 接口隔离 + 共享 HTTP 传输 + 指数退避带抖动重试 |
| API 风格 | 现代务实 REST：POST/GET/PUT/PATCH/DELETE；`@autoPatch` 自动支持三协议 PATCH（RFC 6902 / 7386 / Shorthand）|
| API 文档 | TypeSpec → OpenAPI 3.1 YAML → Stoplight Elements（`embed.FS`）|

## 项目结构

```text
api/
  spec/                 TypeSpec 契约单一事实来源（main.tsp、common/*、domains/*）
  emitters/go-rest/     自研 TypeSpec Go Emitter（同步生成 pkg/api/*.gen.go）
cmd/                    应用入口（main.go）
config/                 配置定义与加载（Viper）
ent/
  schema/               实体 schema 定义（改这里，然后 make ent）
  *.go                  ent 自动生成代码（不要手改）
pkg/
  api/                  TypeSpec 生成的平铺 DTO、Validate()、Handler 接口与 ServeMux 挂载函数
  appctx/               context 工具（user、request_id、locale）
  clients/              第三方 API 客户端（每个 API 一个子包）
    <apiname>/
      interface.go      API interface + 响应 DTO（service 层唯一依赖）
      client.go         具体实现（从 DI 取 config + HTTPClient）
      errors.go         oops 错误构造辅助
      package.go        do.Package(do.Lazy(NewXxxClient))
  codes/                业务错误码 → HTTP 状态码注册表
  filter/               AIP-160 过滤表达式解析器 + 泛型 FilterTranslator[P]
  handlers/             三层编排 Handler（实现 api.XxxHandler）+ router + docs + pagination
  httpruntime/          极薄 net/http 运行时（编解码、RFC 9457 Problem、中间件链、优雅停机 Server）
  i18n/                 国际化翻译器（embed.FS TOML + Accept-Language 协商）
  infra/                DB / ORM / 共享 HTTPClient 基础设施（DI 生命周期）
  log/                  slog ContextHandler（日志自动带 request_id）
  meta/v1/              通用查询选项（ListOptions）
  middleware/           标准 net/http RequireAuth 中间件
  patch/                多协议 PATCH 实现（RFC 6902 / 7386 / Shorthand）
  services/             领域服务（业务逻辑）
    <domain>_filter.go  AIP-160 过滤器 → ent predicate 翻译
  testutil/             统一测试环境（DI 容器 + 独立内存 SQLite + HTTP 请求/断言辅助）
  utils/                password（bcrypt cost=12）、ULID、Retry
  validator/            FieldViolations + 原子校验规则
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
go run ./cmd/

# 3. 查看 API 文档（非生产环境）
open http://localhost:8000/docs

# 4. 如需修改 api/spec/*.tsp，安装一次 TypeSpec 开发依赖后运行 make api
pnpm --dir api install
make api
```

配置文件为 `config/config.toml`，所有字段均可通过环境变量覆盖，前缀 `ECHO_TEMPLATE_`，例如：

```bash
ECHO_TEMPLATE_JWT_SECRET=mysecret go run ./cmd/
```

## API 风格与 PATCH 多协议

资源以集合 URL 暴露，HTTP 方法表达动作；自定义动词作为 kebab-case 子路径段挂在资源下。

| 方法    | 用途                              | 幂等   | 请求体                                                         |
|---------|-----------------------------------|--------|----------------------------------------------------------------|
| POST    | 创建 / 非幂等自定义动词           | 否     | `CreateXxxRequest`（或自定义动词 DTO）                         |
| GET     | 读取单资源或集合                  | 是     | 无                                                             |
| PUT     | 全量替换（覆盖所有可写字段）      | 是     | `ReplaceXxxRequest`                                            |
| PATCH   | 局部更新（按 Content-Type 分派）  | 视协议 | RFC 6902 / RFC 7386 / Shorthand                                |
| DELETE  | 删除                              | 是     | 无                                                             |

在 TypeSpec 中为 `patch` 操作标注 `@autoPatch(Domain.get, Domain.replace)` 后：
- OpenAPI 3.1 会完整记录 3 种 PATCH 协议及 `415 Unsupported Media Type`。
- 生成的 `api.<Domain>Handler` 接口**无需**手写 `Patch` 方法，`Mount<Domain>Handler` 会自动串联 `Get → rt.ApplyPatch → Validate → Replace`。

| Content-Type                        | 主体格式                                       | 示例                                                       |
|-------------------------------------|------------------------------------------------|------------------------------------------------------------|
| `application/json-patch+json`       | RFC 6902：`{op,path,value}` 数组               | `[{"op":"replace","path":"/title","value":"New"}]`         |
| `application/merge-patch+json`      | RFC 7386：JSON 对象；`null` 表示删除字段       | `{"title":"New","price":null}`                             |
| `application/merge-patch+shorthand` | Shorthand DSL（可省略外层 `{}`），叠加到原资源 | `title: "New", price: 19.99`                               |
| `application/json` / 缺省           | 兼容：按 Merge Patch 处理                      | `{"title":"New"}`                                          |

## 增加一个新领域（Checklist）

以增加 `Order` 领域为例：

**1. TypeSpec 契约**（`api/spec/domains/orders.tsp` + 在 `api/spec/main.tsp` 中 `import`）

```tsp
model Order {
  id: ULID;
  title: string;
  amount?: float64;
  created_at: utcDateTime;
  updated_at: utcDateTime;
}

model OrderList is Page<Order>;

model CreateOrderRequest {
  order_id?: ULID;
  @minLength(1) @maxLength(200) title: string;
  @minValue(0) amount?: float64;
}

model ReplaceOrderRequest {
  @minLength(1) @maxLength(200) title: string;
  @minValue(0) amount?: float64;
}

@route("/api/v1/orders")
@tag("orders")
@useAuth(BearerAuth)
interface Orders {
  @post create(@body body: CreateOrderRequest): { @statusCode statusCode: 201; @body body: Order } | BadRequestProblem | UnauthorizedProblem | ConflictProblem;
  @get list(...ListQuery): OrderList | BadRequestProblem | UnauthorizedProblem;
  @get @route("/{order}") get(@path order: ULID): Order | UnauthorizedProblem | NotFoundProblem;
  @put @route("/{order}") replace(@path order: ULID, @body body: ReplaceOrderRequest): Order | BadRequestProblem | UnauthorizedProblem | NotFoundProblem;
  @autoPatch(Orders.get, Orders.replace) @patch @route("/{order}") patch(@path order: ULID, @header contentType: "application/merge-patch+json" | "application/json-patch+json" | "application/merge-patch+shorthand" | "application/json", @body body: Record<unknown> | JsonPatchOperation[] | string): Order | BadRequestProblem | UnauthorizedProblem | NotFoundProblem | UnsupportedMediaTypeProblem;
  @delete @route("/{order}") delete(@path order: ULID): { @statusCode statusCode: 204 } | UnauthorizedProblem | NotFoundProblem;
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

**3. 错误码与国际化**（`pkg/codes/order.go` + `pkg/i18n/messages/{en,zh-CN}.toml`）

```go
const (
    OrderNotFound      Code = "ORDER_NOT_FOUND"
    OrderAlreadyExists Code = "ORDER_ALREADY_EXISTS"
    OrderInternal      Code = "ORDER_INTERNAL_ERROR"
)

func init() {
    Register(OrderNotFound,      http.StatusNotFound)
    Register(OrderAlreadyExists, http.StatusConflict)
    Register(OrderInternal,      http.StatusInternalServerError)
}
```

**4. Service**（`pkg/services/order.go` + 注册到 `pkg/services/package.go`）

实现 `Create` / `Get` / `List` / `Replace` / `Delete`（返回 `*ent.Order`），以及 `pkg/services/order_filter.go`。

**5. Handler**（`pkg/handlers/order.go` + 注册到 `pkg/handlers/package.go`）

实现生成的 `api.OrdersHandler` 纯 Go 接口（编排多 Service + `toOrder` 投影），在 `Mount` 中调用 `api.MountOrdersHandler(mux, h, rt, auth)`，注册 `provideHandler(NewOrderHandler)` 即可自动挂载。

## 错误处理约定

所有 service 层错误通过 `oops` 构造：

```go
// 领域内部错误（对外隐藏细节与 params）
oops.Code(codes.OrderInternal.String()).Tags("order").Wrapf(err, "%s", op)

// 客户端错误（携带 params 供 i18n 模板插值与 Problem.Params 暴露）
oops.Code(codes.OrderNotFound.String()).
    With(codes.CtxKeyParams, map[string]string{"order_id": id}).
    Public(fmt.Sprintf("Order %q not found.", id)).
    Errorf("order not found")
```

`httpruntime.Runtime.WriteProblem` 自动将错误转换为 RFC 9457 `application/problem+json` 响应，无需在 handler 层手动处理。

## 命令

```bash
make api               # 编译 TypeSpec → OpenAPI 3.1 + pkg/api/*.gen.go
make ent               # 重新生成 ent ORM 代码
make test              # 运行测试（-race）
make cover             # 统计业务包单元测试覆盖率
make build             # 编译
```
