package handlers

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/apperrs"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
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
		return apperrs.ErrBadRequest.Wrap(err)
	}

	if err := in.Validate(); err != nil {
		return err
	}

	// 调用服务层
	out, err := h.auth.Register(ctx, &in)
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
		return apperrs.ErrBadRequest.Wrap(err)
	}

	if err := in.Validate(); err != nil {
		return err
	}

	// 调用服务层
	out, err := h.auth.Login(ctx, &in)
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
		return apperrs.ErrBadRequest.Wrap(err)
	}

	// 验证输入
	if err := req.Validate(); err != nil {
		return err
	}

	// 调用服务层
	out, err := h.auth.RefreshToken(ctx, &req)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Logout 用户登出（撤销令牌）
func (h *AuthHandler) Logout(c echo.Context) error {
	ctx := c.Request().Context()

	// 从认证中间件获取当前用户
	_, ok := appctx.GetUserFromContext(ctx)
	if !ok {
		return apperrs.ErrUnauthorized.Errorf("用户未登录")
	}

	// 从Authorization header获取token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return apperrs.ErrUnauthorized.Errorf("用户未登录")
	}

	// 检查Bearer格式
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return apperrs.ErrUnauthorized.Errorf("用户未登录")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return apperrs.ErrUnauthorized.Errorf("用户未登录")
	}

	// 撤销token
	err := h.auth.Logout(ctx, token)
	if err != nil {
		return err
	}

	return Success(c, nil)
}
