# Echo Template 中间件开发规范

## 概述

中间件是处理HTTP请求的重要组件，负责在请求到达处理器之前或之后执行特定的逻辑。本项目将所有自定义中间件统一放置在`pkg/middleware`目录下。

## 目录结构

```
pkg/middleware/
├── auth.go          # 认证中间件
├── rate_limit.go    # 限流中间件
├── request_id.go    # 请求ID中间件
├── cors.go          # CORS中间件（如需要）
├── security.go      # 安全头中间件（如需要）
└── logging.go       # 访问日志中间件（如需要）
```

## 中间件开发规范

### 1. 基础模式

#### 中间件函数签名
所有中间件必须遵循Echo的中间件签名：
```go
func MiddlewareName(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 前置处理逻辑
        
        // 调用下一个处理器
        err := next(c)
        
        // 后置处理逻辑（可选）
        
        return err
    }
}
```

#### 带配置的中间件模式
```go
type MiddlewareConfig struct {
    Option1 string
    Option2 int
}

type Middleware struct {
    config MiddlewareConfig
}

func NewMiddleware(config MiddlewareConfig) *Middleware {
    return &Middleware{
        config: config,
    }
}

func (m *Middleware) Handler(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 使用 m.config 中的配置
        return next(c)
    }
}
```

### 2. 认证中间件规范

#### 基础认证中间件
```go
// 强制认证中间件
func (m *AuthMiddleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 1. 从Authorization header获取token
        // 2. 验证JWT token
        // 3. 检查token是否存在于数据库中
        // 4. 检查用户状态
        // 5. 将用户信息存储到context中
        return next(c)
    }
}

// 可选认证中间件
func (m *AuthMiddleware) OptionalAuth(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 尝试认证，失败时不返回错误，继续处理请求
        return next(c)
    }
}
```

#### 用户信息获取函数
```go
// 从context中获取当前用户
func GetUserFromContext(ctx context.Context) (*ent.User, bool) {
    user, ok := ctx.Value(UserContextKey).(*ent.User)
    return user, ok
}

// 从Echo context中获取当前用户
func GetUserFromEcho(c echo.Context) (*ent.User, bool) {
    return GetUserFromContext(c.Request().Context())
}

// 强制获取用户（用于必须有用户的场景）
func MustGetUser(ctx context.Context) *ent.User {
    user, ok := GetUserFromContext(ctx)
    if !ok {
        panic("user not found in context")
    }
    return user
}
```

### 3. 限流中间件规范

#### 基于IP的限流
```go
type RateLimiter struct {
    mu          sync.RWMutex
    clients     map[string]*clientInfo
    rate        int           // 每分钟允许的请求数
    windowSize  time.Duration // 时间窗口大小
}

func (rl *RateLimiter) RateLimit(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        clientIP := c.RealIP()
        
        // 检查限流逻辑
        if exceeded := rl.checkRateLimit(clientIP); exceeded {
            // 设置限流响应头
            rl.setRateLimitHeaders(c)
            return errors.New(429, "请求频率过高，请稍后重试")
        }
        
        return next(c)
    }
}
```

#### 基于用户的限流
```go
func (rl *RateLimiter) UserRateLimit(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        user, ok := GetUserFromEcho(c)
        if !ok {
            // 未认证用户使用IP限流
            return rl.RateLimit(next)(c)
        }
        
        // 基于用户ID的限流逻辑
        if exceeded := rl.checkUserRateLimit(user.ID); exceeded {
            return errors.New(429, "请求频率过高，请稍后重试")
        }
        
        return next(c)
    }
}
```

### 4. 中间件注册规范

#### 在router.go中注册全局中间件
```go
func BuildRouter(c *services.Container) error {
    g := c.Web.Group("")

    // 全局中间件（按执行顺序）
    g.Use(
        // 1. 请求ID生成
        echomw.RequestIDWithConfig(echomw.RequestIDConfig{
            RequestIDHandler: middleware.RequestIDHandler,
        }),
        
        // 2. 访问日志
        middleware.AccessLogger(),
        
        // 3. 恢复panic
        echomw.Recover(),
        
        // 4. 限流（可选）
        middleware.NewRateLimiter(100).RateLimit,
        
        // 5. 安全头
        middleware.SecurityHeaders(),
        
        // 6. 压缩
        echomw.Gzip(),
        
        // 7. 超时控制
        echomw.TimeoutWithConfig(echomw.TimeoutConfig{
            Timeout: c.Config.App.Timeout,
        }),
    )

    return nil
}
```

#### 在Handler中使用认证中间件
```go
func (h *UserHandler) Routes(g *echo.Group) {
    // 公开路由
    api := g.Group("/api/v1/users")
    api.GET("/public", h.GetPublicUsers)
    
    // 需要认证的路由
    authMw := middleware.NewAuthMiddleware(h.orm)
    protected := g.Group("/api/v1/users")
    protected.Use(authMw.RequireAuth)
    protected.GET("", h.GetUsers)
    protected.POST("", h.CreateUser)
    
    // 可选认证的路由
    optional := g.Group("/api/v1/users")
    optional.Use(authMw.OptionalAuth)
    optional.GET("/recommendations", h.GetRecommendations)
}
```

### 5. 中间件最佳实践

#### 错误处理
```go
func SomeMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 使用统一的错误处理
        if err := validateSomething(); err != nil {
            return errors.BadRequestError("验证失败").
                With("reason", err.Error())
        }
        
        return next(c)
    }
}
```

#### 日志记录
```go
func SomeMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        ctx := c.Request().Context()
        
        // 记录中间件执行
        slog.DebugContext(ctx, "中间件开始执行", "middleware", "SomeMiddleware")
        
        start := time.Now()
        err := next(c)
        duration := time.Since(start)
        
        // 记录执行结果
        slog.DebugContext(ctx, "中间件执行完成", 
            "middleware", "SomeMiddleware",
            "duration_ms", duration.Milliseconds(),
            "error", err,
        )
        
        return err
    }
}
```

#### 性能考虑
```go
func ExpensiveMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 避免在中间件中执行耗时操作
        // 如需要，使用异步处理
        go func() {
            // 异步处理逻辑
        }()
        
        return next(c)
    }
}
```

#### Context值传递
```go
type contextKey string

const SomeDataKey contextKey = "some_data"

func DataMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        // 获取数据
        data := getSomeData()
        
        // 存储到context中
        ctx := context.WithValue(c.Request().Context(), SomeDataKey, data)
        c.SetRequest(c.Request().WithContext(ctx))
        
        return next(c)
    }
}

// 辅助函数获取数据
func GetDataFromContext(ctx context.Context) (interface{}, bool) {
    data, ok := ctx.Value(SomeDataKey).(interface{})
    return data, ok
}
```

### 6. 常用中间件模板

#### CORS中间件
```go
func CORS() echo.MiddlewareFunc {
    return echomw.CORSWithConfig(echomw.CORSConfig{
        AllowOrigins: []string{"*"},
        AllowMethods: []string{
            http.MethodGet,
            http.MethodPost,
            http.MethodPut,
            http.MethodDelete,
            http.MethodOptions,
        },
        AllowHeaders: []string{
            "Origin",
            "Content-Type",
            "Accept",
            "Authorization",
            "X-Requested-With",
        },
        ExposeHeaders: []string{
            "X-RateLimit-Limit",
            "X-RateLimit-Remaining",
            "X-RateLimit-Reset",
        },
    })
}
```

#### 安全头中间件
```go
func SecurityHeaders() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // 安全头
            c.Response().Header().Set("X-Content-Type-Options", "nosniff")
            c.Response().Header().Set("X-Frame-Options", "DENY")
            c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
            c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
            
            return next(c)
        }
    }
}
```

#### 访问日志中间件
```go
func AccessLogger() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            ctx := c.Request().Context()
            start := time.Now()
            
            err := next(c)
            
            duration := time.Since(start)
            
            slog.InfoContext(ctx, "HTTP请求完成",
                "method", c.Request().Method,
                "uri", c.Request().RequestURI,
                "status", c.Response().Status,
                "duration_ms", duration.Milliseconds(),
                "user_agent", c.Request().UserAgent(),
                "remote_addr", c.RealIP(),
                "bytes_out", c.Response().Size,
            )
            
            return err
        }
    }
}
```

### 7. 测试规范

#### 中间件单元测试
```go
func TestAuthMiddleware(t *testing.T) {
    // 创建测试容器
    c := services.NewContainer()
    defer c.Shutdown()
    
    // 创建中间件
    authMw := middleware.NewAuthMiddleware(c.ORM)
    
    // 测试用例
    tests := []struct {
        name           string
        authHeader     string
        expectedStatus int
        setupFunc      func() string // 返回有效token
    }{
        {
            name:           "无认证头",
            authHeader:     "",
            expectedStatus: 401,
        },
        {
            name:           "无效格式",
            authHeader:     "Invalid token",
            expectedStatus: 401,
        },
        {
            name:           "有效token",
            setupFunc:      func() string { /* 创建有效token */ },
            expectedStatus: 200,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 创建测试请求
            req := httptest.NewRequest("GET", "/test", nil)
            if tt.authHeader != "" {
                req.Header.Set("Authorization", tt.authHeader)
            }
            if tt.setupFunc != nil {
                token := tt.setupFunc()
                req.Header.Set("Authorization", "Bearer "+token)
            }
            
            rec := httptest.NewRecorder()
            c := echo.New().NewContext(req, rec)
            
            // 创建测试handler
            handler := func(c echo.Context) error {
                return c.String(200, "OK")
            }
            
            // 执行中间件
            err := authMw.RequireAuth(handler)(c)
            
            // 验证结果
            assert.Equal(t, tt.expectedStatus, rec.Code)
        })
    }
}
```

## 注意事项

### 1. 中间件执行顺序很重要
- 请求ID生成应该最先执行
- 认证中间件应该在需要用户信息的中间件之前
- 错误恢复中间件应该较早执行
- 压缩和缓存中间件应该较晚执行

### 2. 性能考虑
- 避免在中间件中执行重型操作
- 使用缓存减少重复计算
- 异步处理非关键任务

### 3. 错误处理
- 中间件错误应该使用统一的错误类型
- 记录详细的错误日志
- 不要泄露敏感信息

### 4. 可配置性
- 中间件应该支持配置
- 提供合理的默认值
- 支持环境相关的配置

### 5. 测试友好
- 中间件应该易于测试
- 提供测试辅助函数
- 支持mock依赖

## 总结

良好的中间件设计能够：
1. 提高代码复用性
2. 分离关注点
3. 增强系统安全性
4. 提供统一的横切功能
5. 简化Handler逻辑

遵循这些规范能够确保中间件的一致性、可维护性和高性能。 