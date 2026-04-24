package handlers

import (
	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// MeHandler 用户处理器
type MeHandler struct {
	me     *services.MeService
	authMW middleware.AuthMiddleware
}

// NewMeHandler 构造 MeHandler 并注入依赖。
func NewMeHandler(i do.Injector) (*MeHandler, error) {
	return &MeHandler{
		me:     do.MustInvoke[*services.MeService](i),
		authMW: do.MustInvoke[middleware.AuthMiddleware](i),
	}, nil
}

// Routes 注册路由
func (h *MeHandler) Routes(g *echo.Group) {
	protected := g.Group("/api/v1/me")
	protected.Use(echo.MiddlewareFunc(h.authMW))

	protected.GET("", h.Get)
	protected.PUT("/username", h.UpdateUsername)
	protected.PUT("/email", h.UpdateEmail)
	protected.POST("/change-password", h.ChangePassword)
}

// Get 获取当前用户信息
//
//	@Summary	获取当前用户信息
//	@Tags		me
//	@Security	BearerAuth
//	@Produce	json
//	@Success	200	{object}	types.UserInfo
//	@Failure	401	{object}	ErrorResponse
//	@Router		/api/v1/me [get]
func (h *MeHandler) Get(c *echo.Context) error {
	user := appctx.MustGetUserFromContext(c.Request().Context())
	return c.JSON(200, types.UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Status:      string(user.Status),
		LastLoginAt: user.LastLoginAt,
		CreatedAt:   user.CreatedAt,
	})
}

// UpdateUsername 更新当前用户用户名
//
//	@Summary	更新用户名
//	@Tags		me
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.UpdateUsernameRequest	true	"新用户名"
//	@Success	200		{object}	map[string]interface{}
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Router		/api/v1/me/username [put]
func (h *MeHandler) UpdateUsername(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)

	var req types.UpdateUsernameRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	if err := h.me.UpdateUsername(ctx, user.ID, &req); err != nil {
		return err
	}

	return c.JSON(200, map[string]interface{}{})
}

// UpdateEmail 更新当前用户邮箱
//
//	@Summary	更新邮箱
//	@Tags		me
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.UpdateEmailRequest	true	"新邮箱"
//	@Success	200		{object}	map[string]interface{}
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Router		/api/v1/me/email [put]
func (h *MeHandler) UpdateEmail(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)

	var req types.UpdateEmailRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	if err := h.me.UpdateEmail(ctx, user.ID, &req); err != nil {
		return err
	}

	return c.JSON(200, map[string]interface{}{})
}

// ChangePassword 修改当前用户密码
//
//	@Summary	修改密码
//	@Tags		me
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		types.ChangePasswordRequest	true	"密码信息"
//	@Success	200		{object}	map[string]interface{}
//	@Failure	400		{object}	ErrorResponse
//	@Failure	401		{object}	ErrorResponse
//	@Router		/api/v1/me/change-password [post]
func (h *MeHandler) ChangePassword(c *echo.Context) error {
	ctx := c.Request().Context()
	user := appctx.MustGetUserFromContext(ctx)

	var req types.ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	if err := h.me.ChangePassword(ctx, user.ID, &req); err != nil {
		return err
	}

	return c.JSON(200, map[string]interface{}{})
}
