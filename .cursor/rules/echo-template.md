# Echo Template 项目 Cursor 规则

## 项目概述
这是一个基于 Go 的高性能 Web 应用模板，使用以下技术栈：
- **Web 框架**: Echo v4 - 高性能、可扩展、极简的 Go Web 框架
- **ORM**: Ent - 简单而强大的实体框架，用于数据建模和查询
- **数据库**: SQLite - 轻量级、高可靠性的嵌入式数据库
- **配置管理**: Viper - 灵活的配置管理库
- **验证框架**: Zog - 类型安全的验证库
- **测试**: Testify - Go 测试断言库

## 核心设计规范

### 1. 数据传输对象 (DTO) 命名规范

#### Input/Output 命名约定
所有API数据传输对象必须遵循以下命名规范：
- **输入类型**：使用 `xxxInput` 后缀，例如：`RegisterInput`、`LoginInput`、`CreateUserInput`
- **输出类型**：使用 `xxxOutput` 后缀，例如：`AuthOutput`、`UserOutput`、`ListUsersOutput`

#### 类型定义位置
- 所有输入输出类型统一定义在 `pkg/types/` 目录下
- 按功能模块分文件组织，例如：
  - `pkg/types/auth.go` - 认证相关的输入输出类型
  - `pkg/types/user.go` - 用户相关的输入输出类型
  - `pkg/types/order.go` - 订单相关的输入输出类型

#### 完整示例（pkg/types/auth.go）
```go
package types

import (
    "time"
    z "github.com/Oudwins/zog"
    "github.com/liukeshao/echo-template/pkg/errors"
)

// RegisterInput 用户注册输入
type RegisterInput struct {
    Username string `json:"username"`
    Email    string `json:"email"`
    Password string `json:"password"`
}

// Validate 验证注册输入
func (i *RegisterInput) Validate() []*errors.ErrorDetail {
    issuesMap := z.Struct(z.Shape{
        "Username": z.String().Min(3, z.Message("用户名长度不能小于3")).Max(50, z.Message("用户名长度不能大于50")).Required(z.Message("用户名不能为空")),
        "Email":    z.String().Email(z.Message("邮箱格式不正确")).Required(z.Message("邮箱不能为空")),
        "Password": z.String().Min(8, z.Message("密码长度不能小于8")).Required(z.Message("密码不能为空")),
    }).Validate(i)

    return ConvertZogIssues(issuesMap)
}

// AuthOutput 认证输出
type AuthOutput struct {
    User         *UserInfo `json:"user"`
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    ExpiresAt    int64     `json:"expires_at"`
}

// UserInfo 用户信息（可在多个Output中复用）
type UserInfo struct {
    ID          string     `json:"id"`
    Username    string     `json:"username"`
    Email       string     `json:"email"`
    Status      string     `json:"status"`
    LastLoginAt *time.Time `json:"last_login_at,omitempty"`
    CreatedAt   time.Time  `json:"created_at"`
}
```

#### 验证规范
**所有Input类型必须实现Validate()方法**：
- 使用 Zog 验证框架进行类型安全验证
- 返回 `[]*errors.ErrorDetail` 类型的错误详情
- 通过 `ConvertZogIssues` 转换 Zog 验证错误为统一格式

#### Handler层使用规范
```go
// Handler层统一使用types包中的类型，并调用Validate方法
func (h *AuthHandler) Register(c echo.Context) error {
    var in types.RegisterInput
    if err := c.Bind(&in); err != nil {
        return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
    }
    
    // 使用统一的验证方法
    if errorDetails := in.Validate(); len(errorDetails) > 0 {
        return ValidationError("验证失败", errorDetails).JSON(c)
    }
    
    output, err := h.authService.Register(ctx, &in)
    if err != nil {
        return err
    }
    
    return Success(output).JSON(c)
}
```

### 2. 错误处理统一规范

#### ErrorDetail 统一定义
项目中所有错误详情都使用 `pkg/errors` 包中的 `ErrorDetail` 类型：

```go
// pkg/errors/error_detail.go
type ErrorDetail struct {
    Message  string `json:"message,omitempty"`  // 错误消息
    Location string `json:"location,omitempty"` // 错误位置（字段名）
    Value    any    `json:"value,omitempty"`    // 错误值或错误代码
}
```

#### 响应结构统一规范
```go
// pkg/handlers/response.go
type Response struct {
    Code      int                    `json:"code"`                 // 业务状态码
    Message   string                 `json:"message"`              // 响应消息
    Data      interface{}            `json:"data"`                 // 响应数据
    Errors    []*errors.ErrorDetail  `json:"errors,omitempty"`     // 错误详情列表（指针类型）
    Timestamp int64                  `json:"timestamp"`            // 时间戳
    RequestID string                 `json:"request_id,omitempty"` // 请求ID
    Success   bool                   `json:"success"`              // 是否成功
}
```

### 3. API文档维护规范 📋

#### OpenAPI 文档同步要求
**⚠️ 强制要求：任何 API 变更都必须同步更新 `docs/openapi.yaml` 文档**

#### 文档更新时机
以下操作**必须**同步更新 OpenAPI 文档：

1. **新增API接口**
   - 在 `paths` 中添加新的路径和方法
   - 定义对应的 `requestBody` 和 `responses`
   - 在 `components/schemas` 中添加相关的 Input/Output 类型

2. **删除API接口**
   - 从 `paths` 中移除对应的路径和方法
   - 清理不再使用的 `components/schemas` 定义
   - 移除相关的 tags 和 responses

3. **修改API接口**
   - 更新 `paths` 中的请求/响应定义
   - 同步更新 `components/schemas` 中的类型定义
   - 更新相关的参数、状态码、示例等

4. **修改数据结构**
   - 同步更新 `components/schemas` 中对应的类型定义
   - 确保字段名、类型、验证规则与代码一致
   - 更新相关的示例和描述

#### OpenAPI 文档结构规范

##### 基本信息
```yaml
openapi: 3.1.0
info:
  title: Echo Template API Documentation
  description: 基于Echo框架的高性能Web应用API文档
  version: 1.0.0
  contact:
    name: API Support
    email: support@example.com

servers:
  - url: http://localhost:8000
    description: 开发服务器
```

##### 路径定义规范
```yaml
paths:
  /api/v1/users:
    post:
      tags:
        - 用户  # 使用中文标签名
      summary: 创建用户  # 使用中文描述
      description: 创建新的用户账户
      operationId: createUser  # 使用驼峰命名
      requestBody:
        required: true
        description: 用户创建信息
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateUserInput'
      responses:
        '200':
          description: 创建成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UserResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
```

##### Schema 定义规范
```yaml
components:
  schemas:
    CreateUserInput:
      type: object
      title: 创建用户输入  # 使用中文标题
      description: 创建用户所需的信息
      required:
        - username
        - email
      properties:
        username:
          type: string
          description: 用户名，用于登录和显示
          minLength: 3
          maxLength: 50
          pattern: '^[a-zA-Z0-9_-]+$'
          examples:
            - john_doe
            - user123
        email:
          type: string
          format: email
          description: 用户邮箱地址
          examples:
            - user@example.com
      additionalProperties: false  # 防止额外字段
```

#### 文档编写最佳实践

1. **命名约定**
   - Schema 名称与 Go 类型名称保持一致（如 `CreateUserInput`）
   - operationId 使用驼峰命名法
   - 标签和描述使用中文

2. **详细描述**
   - 每个字段都要有清晰的 `description`
   - 提供具体的 `examples`
   - 包含验证规则（如 `minLength`、`maxLength`、`pattern`）

3. **响应定义**
   - 统一使用项目的响应格式
   - 定义常用的错误响应（400、401、404、500）
   - 包含错误示例

4. **标签分组**
   - 按业务模块分组（如：认证、用户、订单）
   - 使用中文标签名便于理解

#### 文档验证和测试
```bash
# 验证 OpenAPI 文档语法
# 可以使用在线工具：https://editor.swagger.io/

# 启动应用后访问文档
# http://localhost:8000/docs
```

#### 文档更新工作流示例

**场景：新增用户列表API**

1. **在Handler中添加新路由**：
```go
func (h *UserHandler) Routes(g *echo.Group) {
    api := g.Group("/api/v1/users")
    api.GET("", h.GetUsers)  // 新增的API
    api.POST("", h.CreateUser)
}
```

2. **在 `pkg/types/user.go` 中定义类型**：
```go
// GetUsersInput 获取用户列表输入
type GetUsersInput struct {
    Page     int    `json:"page" query:"page"`
    PageSize int    `json:"page_size" query:"page_size"`
    Keyword  string `json:"keyword" query:"keyword"`
}

// GetUsersOutput 获取用户列表输出
type GetUsersOutput struct {
    Users      []*UserInfo `json:"users"`
    Total      int64       `json:"total"`
    Page       int         `json:"page"`
    PageSize   int         `json:"page_size"`
    TotalPages int         `json:"total_pages"`
}
```

3. **⚠️ 同步更新 `docs/openapi.yaml`**：
```yaml
# 在 paths 中添加
/api/v1/users:
  get:
    tags:
      - 用户
    summary: 获取用户列表
    description: 分页获取用户列表，支持关键字搜索
    operationId: getUsers
    parameters:
      - name: page
        in: query
        description: 页码，从1开始
        schema:
          type: integer
          minimum: 1
          default: 1
      - name: page_size
        in: query
        description: 每页数量
        schema:
          type: integer
          minimum: 1
          maximum: 100
          default: 20
      - name: keyword
        in: query
        description: 搜索关键字
        schema:
          type: string
    responses:
      '200':
        description: 获取成功
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/GetUsersResponse'

# 在 components/schemas 中添加
GetUsersResponse:
  type: object
  title: 获取用户列表响应
  properties:
    code:
      type: integer
      examples: [200]
    message:
      type: string
      examples: ["操作成功"]
    data:
      $ref: '#/components/schemas/GetUsersOutput'
    timestamp:
      type: integer
    request_id:
      type: string
    success:
      type: boolean
      examples: [true]

GetUsersOutput:
  type: object
  title: 用户列表数据
  properties:
    users:
      type: array
      items:
        $ref: '#/components/schemas/UserInfo'
    total:
      type: integer
      description: 总数量
    page:
      type: integer
      description: 当前页码
    page_size:
      type: integer
      description: 每页数量
    total_pages:
      type: integer
      description: 总页数
```

## 代码规范

### 4. 项目结构规范
```
/
├── cmd/           # 应用程序入口点
├── pkg/           # 核心业务逻辑
│   ├── handlers/  # HTTP 路由处理器
│   ├── services/  # 业务服务层
│   ├── middleware/ # 中间件（统一放置在此目录）
│   ├── context/   # 上下文处理
│   ├── errors/    # 错误处理（包含统一的ErrorDetail）
│   ├── types/     # 数据传输对象（Input/Output定义）
│   ├── log/       # 日志系统
│   ├── utils/     # 工具函数
│   └── tests/     # 测试辅助工具
├── config/        # 配置文件和结构
├── docs/          # 文档（包含 openapi.yaml）
└── ent/           # Ent ORM 生成代码和 schema
```

## 开发工作流

### 新功能开发流程
1. 如需新的输入类型，在pkg/types中定义，实现Validate()方法
2. 如需新服务，先在服务容器中注册
3. 创建 Handler 文件，通过Init()方法注入依赖
4. 定义路由和处理逻辑
5. **⚠️ 同步更新 `docs/openapi.yaml` API文档**
6. 如需新实体，创建 Ent schema
7. 编写测试

### API变更检查清单 ✅
每次API变更时，必须检查以下项目：

- [ ] **代码实现**：Handler、Service、Types 定义完成
- [ ] **文档同步**：`docs/openapi.yaml` 已更新
- [ ] **类型定义**：Input/Output 类型在 `pkg/types/` 中定义
- [ ] **验证方法**：Input 类型实现了 `Validate()` 方法  
- [ ] **响应格式**：使用统一的响应格式
- [ ] **错误处理**：定义了相关的错误响应
- [ ] **测试编写**：添加了对应的测试用例

## 常用命令
```bash
# Ent 相关
make ent-new name=EntityName  # 创建新实体
make ent-gen      # 生成 Ent 代码

# 开发
go run cmd/main.go  # 启动应用
go test ./...       # 运行所有测试
go build ./...      # 编译项目

# API 文档相关
make docs           # 运行所有文档检查
make docs-validate  # 验证 OpenAPI 文档语法
make docs-lint      # 检查文档结构和常见问题
make docs-serve     # 启动本地文档服务器（端口 8081）
make docs-install   # 安装文档工具（swagger CLI）
make docs-check     # 检查文档是否需要更新

# 快速检查命令
grep -r "type.*Input struct" pkg/types/     # 查找所有 Input 类型
grep -r "type.*Output struct" pkg/types/    # 查找所有 Output 类型
grep -r "Routes.*echo.Group" pkg/handlers/  # 查找所有 Handler 路由

# 文档验证
# 访问 http://localhost:8000/docs 查看API文档
# 使用 https://editor.swagger.io/ 验证OpenAPI语法
```

### 5. Handler 开发规范

#### Handler 接口
所有 Handler 必须实现以下接口：

```go
type Handler interface {
    // Routes 允许自注册 HTTP 路由到路由器
    Routes(g *echo.Group)
    
    // Init 提供服务容器进行初始化
    Init(*services.Container) error
}
```

#### Handler 开发模式
```go
// 1. 使用 init 函数注册 Handler
func init() {
    Register(new(UserHandler))
}

// 2. 实现 Init 方法（依赖注入）
func (h *UserHandler) Init(c *services.Container) error {
    h.orm = c.ORM
    h.authService = c.Auth        // 从容器注入服务
    return nil
}

// 3. 实现 Routes 方法（路由定义）
func (h *UserHandler) Routes(g *echo.Group) {
    api := g.Group("/api/v1/users")
    api.GET("", h.GetUsers)
    api.POST("", h.CreateUser)
}
```

### 6. 服务容器规范

#### 服务容器架构
```go
type Container struct {
    ORM    *ent.Client          // 数据库ORM客户端
    Config *config.Config       // 配置管理
    Web    *echo.Echo          // Web框架实例
    
    // 业务服务（通过容器注入）
    Auth   *services.AuthService    // 认证服务
    User   *services.UserService    // 用户服务
}
```

#### 依赖注入最佳实践
- **所有服务都在容器中注册**：避免在Handler中直接创建服务实例
- **Handler通过Init()方法注入依赖**：保持依赖关系明确和可测试
- **服务之间的依赖通过容器管理**：避免循环依赖

### 7. 中间件开发规范

#### 中间件基础模式
```go
// 简单中间件
func MiddlewareName(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 前置处理逻辑
        err := next(c)
        // 后置处理逻辑（可选）
        return err
    }
}
```

#### 认证中间件使用
```go
// 在Handler中使用认证中间件
func (h *UserHandler) Routes(g *echo.Group) {
    // 公开路由
    api := g.Group("/api/v1/users")
    api.GET("/public", h.GetPublicUsers)
    
    // 需要认证的路由
    authMw := middleware.NewAuthMiddleware(h.orm)
    protected := g.Group("/api/v1/users")
    protected.Use(authMw.RequireAuth)
    protected.GET("", h.GetUsers)
}
```

### 8. 响应处理规范

#### 统一响应格式
```go
// 成功响应
return Success(data).JSON(c)

// 验证错误响应 - 使用统一的ErrorDetail类型
errorDetails := []*errors.ErrorDetail{
    {Location: "email", Message: "邮箱格式不正确", Value: "INVALID_EMAIL"},
}
return ValidationError("验证失败", errorDetails).JSON(c)
```

### 9. 日志规范

#### 使用结构化日志
```go
import "log/slog"

// 推荐：使用 Context 版本，自动注入 request_id
slog.InfoContext(ctx, "处理用户请求", "user_id", userID)
slog.ErrorContext(ctx, "业务逻辑错误", "error", err)
```

## 编码最佳实践

### 验证处理
- **所有Input类型必须实现Validate()方法**
- **使用Zog进行类型安全验证**
- **通过ConvertZogIssues统一转换错误格式**
- **验证错误统一使用[]*errors.ErrorDetail格式**

### 错误处理
- 始终检查并处理错误
- 使用包装错误提供上下文
- **统一使用pkg/errors包中的ErrorDetail类型**

### 依赖管理
- **避免在Handler中直接创建服务实例**
- **通过服务容器管理所有依赖关系**
- **服务之间的依赖通过构造函数注入**

## 核心原则

### 类型安全原则
- **所有输入类型都有验证方法**
- **错误详情使用统一的ErrorDetail类型**
- **响应格式标准化，使用指针类型提高效率**
- **验证框架提供编译时类型检查**

### 依赖注入原则
- **单一职责**：每个服务只负责一个领域
- **依赖倒置**：依赖抽象接口而非具体实现
- **容器管理**：所有依赖通过容器统一管理
- **生命周期控制**：容器负责服务的创建和销毁

### API文档一致性原则
- **代码与文档同步**：任何API变更都必须更新OpenAPI文档
- **类型定义一致**：文档中的Schema与Go类型保持一致
- **验证规则一致**：文档中的验证规则与代码中的Validate()方法一致
- **响应格式一致**：文档中的响应格式与实际API响应保持一致

记住：这个项目的目标是快速、简单的开发。保持代码简洁，优先选择简单的解决方案。中间件应该专注单一职责，便于组合和测试。服务通过容器注入，保持松耦合和高可测试性。验证逻辑统一化，确保数据安全和类型安全。**最重要的是：任何API变更都必须同步更新docs/openapi.yaml文档，确保文档与代码的一致性。** 