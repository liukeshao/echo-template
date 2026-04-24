package handlers

import (
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	auth *services.AuthService
}

// NewAuthHandler 构造 AuthHandler 并注入依赖。
func NewAuthHandler(i do.Injector) (*AuthHandler, error) {
	return &AuthHandler{
		auth: do.MustInvoke[*services.AuthService](i),
	}, nil
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
//
//	@Summary	用户注册
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.RegisterRequest	true	"注册信息"
//	@Success	200		{object}	types.AuthResponse
//	@Failure	400		{object}	ErrorResponse
//	@Router		/api/v1/auth/register [post]
func (h *AuthHandler) Register(c *echo.Context) error {
	ctx := c.Request().Context()

	var req types.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	resp, err := h.auth.Register(ctx, &req)
	if err != nil {
		return err
	}

	return c.JSON(200, resp)
}

// Login 用户登录
//
//	@Summary	用户登录
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.LoginRequest	true	"登录信息"
//	@Success	200		{object}	types.AuthResponse
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Router		/api/v1/auth/login [post]
func (h *AuthHandler) Login(c *echo.Context) error {
	ctx := c.Request().Context()

	var req types.LoginRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	resp, err := h.auth.Login(ctx, &req)
	if err != nil {
		return err
	}

	return c.JSON(200, resp)
}

// RefreshToken 刷新访问令牌
//
//	@Summary	刷新访问令牌
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.RefreshTokenRequest	true	"刷新令牌"
//	@Success	200		{object}	types.AuthResponse
//	@Failure	401		{object}	ErrorResponse
//	@Router		/api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *echo.Context) error {
	ctx := c.Request().Context()

	var req types.RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	resp, err := h.auth.RefreshToken(ctx, &req)
	if err != nil {
		return err
	}

	return c.JSON(200, resp)
}

// Logout 用户登出（撤销令牌）
//
//	@Summary	用户登出
//	@Tags		auth
//	@Security	BearerAuth
//	@Produce	json
//	@Success	200	{object}	map[string]string
//	@Failure	401	{object}	ErrorResponse
//	@Router		/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)
	// RequireAuth 已完成验证，此处仅做字符串截取，不重复密码学运算。
	// token 不存入 context，防止被日志框架意外序列化后泄露。
	token := strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer ")

	if err := h.auth.Logout(ctx, user.ID, token); err != nil {
		return err
	}

	return c.JSON(200, map[string]string{})
}
