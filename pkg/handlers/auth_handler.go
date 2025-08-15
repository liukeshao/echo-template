package handlers

import (
	"context"
	"strings"

	"github.com/labstack/echo/v4"
	appcontext "github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/samber/oops"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	auth *services.AuthService
}

// 自动注册
func init() {
	Register(new(AuthHandler))
}

// Init 依赖注入
func (h *AuthHandler) Init(c *services.Container) error {
	h.auth = c.Auth
	return nil
}

// Routes 路由定义
func (h *AuthHandler) Routes(g *echo.Group) {
	auth := g.Group("/api/v1/auth")

	// 公开路由（无需认证）
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/refresh", h.RefreshToken)

	// 需要认证的路由
	protected := g.Group("/api/v1/auth")
	protected.Use(middleware.RequireAuth(h.auth))
	protected.POST("/logout", h.Logout)
}

// Register 用户注册
func (h *AuthHandler) Register(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.RegisterInput
	if err := c.Bind(&in); err != nil {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/register").
			With("username", in.Username).
			With("email", in.Email)
		return errors.ErrBadRequest.
			Wrapf(errorBuilder.Wrapf(err, "JSON解析失败"), "请求参数格式错误")
	}

	if err := in.Validate(); err != nil {
		return err
	}

	// 将请求信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/register").
			With("username", in.Username).
			With("email", in.Email))

	// 调用服务层
	out, err := h.auth.Register(ctxWithBuilder, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Login 用户登录
func (h *AuthHandler) Login(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.LoginInput
	if err := c.Bind(&in); err != nil {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/login").
			With("email", in.Email)
		return errors.ErrBadRequest.
			Wrapf(errorBuilder.Wrapf(err, "JSON解析失败"), "请求参数格式错误")
	}

	if err := in.Validate(); err != nil {
		return err
	}

	// 将请求信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/login").
			With("email", in.Email))

	// 调用服务层
	out, err := h.auth.Login(ctxWithBuilder, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// RefreshToken 刷新访问令牌
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	ctx := c.Request().Context()

	var req types.RefreshTokenInput
	if err := c.Bind(&req); err != nil {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/refresh")
		return errors.ErrBadRequest.
			Wrapf(errorBuilder.Wrapf(err, "JSON解析失败"), "请求参数格式错误")
	}

	// 验证输入
	if err := req.Validate(); err != nil {
		return err
	}

	// 将请求信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/refresh"))

	// 调用服务层 - 修复参数传递
	out, err := h.auth.RefreshToken(ctxWithBuilder, &req)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Logout 用户登出（撤销令牌）
func (h *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()

	// 从认证中间件获取当前用户
	_, ok := appcontext.GetUserFromContext(ctx)
	if !ok {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/logout")
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("用户未登录"), "用户身份验证失败")
	}

	// 从Authorization header获取token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/logout")
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("缺少Authorization头"), "授权头验证失败")
	}

	// 检查Bearer格式
	if !strings.HasPrefix(authHeader, "Bearer ") {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/logout").
			With("auth_header_prefix", authHeader[:min(len(authHeader), 10)])
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("无效的Authorization格式"), "授权格式验证失败")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/logout")
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("令牌不能为空"), "令牌验证失败")
	}

	// 将请求信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/auth/logout"))

	// 撤销token
	err := h.auth.Logout(ctxWithBuilder, token)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// createErrorBuilder 创建带有请求上下文信息的错误构建器 - 遵循 oops 最佳实践
func (h *AuthHandler) createErrorBuilder(ctx context.Context, c echo.Context) oops.OopsErrorBuilder {
	errorBuilder := oops.FromContext(ctx).
		In("handler").
		With("path", c.Request().URL.Path).
		With("method", c.Request().Method).
		With("user_agent", c.Request().UserAgent()).
		With("remote_addr", c.RealIP())

	// 添加请求ID（如果存在）
	if requestID, ok := appcontext.GetRequestIDFromContext(ctx); ok {
		errorBuilder = errorBuilder.Trace(requestID)
	}

	return errorBuilder
}
