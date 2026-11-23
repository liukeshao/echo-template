package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/appctx"
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
func (h *AuthHandler) Routes(g *echo.Group, auth echo.MiddlewareFunc) {
	r := g.Group("/auth")

	// 公开路由（无需认证）
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.RefreshToken)

	// 需要认证的路由
	r.POST("/logout", h.Logout, auth)
}

// Register 用户注册
//
//	@Summary	用户注册
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.RegisterRequest	true	"注册信息"
//	@Success	200		{object}	types.TokenPair
//	@Failure	400		{object}	types.Problem
//	@Failure	409		{object}	types.Problem	"邮箱或用户名已被占用"
//	@Router		/api/v1/auth/register [post]
func (h *AuthHandler) Register(c *echo.Context) error {
	req, err := bindAndValidate[types.RegisterRequest](c)
	if err != nil {
		return err
	}

	tokens, err := h.auth.Register(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, tokens)
}

// Login 用户登录
//
//	@Summary	用户登录
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.LoginRequest	true	"登录信息"
//	@Success	200		{object}	types.TokenPair
//	@Failure	400		{object}	types.Problem
//	@Failure	401		{object}	types.Problem
//	@Failure	403		{object}	types.Problem	"账户已停用"
//	@Router		/api/v1/auth/login [post]
func (h *AuthHandler) Login(c *echo.Context) error {
	req, err := bindAndValidate[types.LoginRequest](c)
	if err != nil {
		return err
	}

	tokens, err := h.auth.Login(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, tokens)
}

// RefreshToken 刷新访问令牌
//
//	@Summary	刷新访问令牌
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.RefreshTokenRequest	true	"刷新令牌"
//	@Success	200		{object}	types.TokenPair
//	@Failure	400		{object}	types.Problem
//	@Failure	401		{object}	types.Problem
//	@Failure	403		{object}	types.Problem	"账户已停用"
//	@Router		/api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *echo.Context) error {
	req, err := bindAndValidate[types.RefreshTokenRequest](c)
	if err != nil {
		return err
	}

	tokens, err := h.auth.RefreshToken(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, tokens)
}

// Logout 用户登出（撤销该用户全部活跃令牌）
//
//	@Summary	用户登出
//	@Tags		auth
//	@Security	BearerAuth
//	@Produce	json
//	@Success	204
//	@Failure	401	{object}	types.Problem
//	@Router		/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)

	if err := h.auth.Logout(ctx, user.ID); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
