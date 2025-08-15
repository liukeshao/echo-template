package handlers

import (
	"context"

	"github.com/labstack/echo/v4"
	appcontext "github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/samber/oops"
)

// MeHandler 用户处理器
type MeHandler struct {
	me   *services.MeService
	auth *services.AuthService
}

// init 注册handler
func init() {
	Register(new(MeHandler))
}

// Init 初始化依赖
func (h *MeHandler) Init(c *services.Container) error {
	h.me = c.Me
	h.auth = c.Auth
	return nil
}

// Routes 注册路由
func (h *MeHandler) Routes(g *echo.Group) {
	// 需要认证的路由组
	protected := g.Group("/api/v1/me")
	protected.Use(middleware.RequireAuth(h.auth)) // 先验证用户身份

	// 当前用户相关路由（不需要额外权限，只要登录即可）
	protected.GET("", h.Get)
	protected.PUT("/username", h.UpdateUsername)
	protected.PUT("/email", h.UpdateEmail)
	protected.POST("/change-password", h.ChangePassword)
}

// Get 获取当前用户信息
func (h *MeHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := appcontext.GetUserFromContext(ctx)
	if !ok {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "GET /api/v1/me")
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("用户未登录"), "用户身份验证失败")
	}

	// 将用户信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("user_id", user.ID).
			With("endpoint", "GET /api/v1/me"))

	// 获取用户信息
	output, err := h.me.GetByID(ctxWithBuilder, user.ID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// UpdateUsername 更新当前用户用户名
func (h *MeHandler) UpdateUsername(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := appcontext.GetUserFromContext(ctx)
	if !ok {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "PUT /api/v1/me/username")
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("用户未登录"), "用户身份验证失败")
	}

	var in types.UpdateUsernameInput
	if err := c.Bind(&in); err != nil {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("user_id", user.ID).
			With("endpoint", "PUT /api/v1/me/username")
		return errors.ErrBadRequest.
			Wrapf(errorBuilder.Wrapf(err, "JSON解析失败"), "请求参数格式错误")
	}

	// 验证输入
	if err := in.Validate(); err != nil {
		return err
	}

	// 将用户信息和请求信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("user_id", user.ID).
			With("new_username", in.Username).
			With("endpoint", "PUT /api/v1/me/username"))

	// 更新当前用户用户名
	out, err := h.me.UpdateUsername(ctxWithBuilder, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// UpdateEmail 更新当前用户邮箱
func (h *MeHandler) UpdateEmail(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := appcontext.GetUserFromContext(ctx)
	if !ok {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "PUT /api/v1/me/email")
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("用户未登录"), "用户身份验证失败")
	}

	var in types.UpdateEmailInput
	if err := c.Bind(&in); err != nil {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("user_id", user.ID).
			With("endpoint", "PUT /api/v1/me/email")
		return errors.ErrBadRequest.
			Wrapf(errorBuilder.Wrapf(err, "JSON解析失败"), "请求参数格式错误")
	}

	// 验证输入
	if err := in.Validate(); err != nil {
		return err
	}

	// 将用户信息和请求信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("user_id", user.ID).
			With("new_email", in.Email).
			With("endpoint", "PUT /api/v1/me/email"))

	// 更新当前用户邮箱
	out, err := h.me.UpdateEmail(ctxWithBuilder, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// ChangePassword 修改当前用户密码
func (h *MeHandler) ChangePassword(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := appcontext.GetUserFromContext(ctx)
	if !ok {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("endpoint", "POST /api/v1/me/change-password")
		return errors.ErrUnauthorized.
			Wrapf(errorBuilder.Errorf("用户未登录"), "用户身份验证失败")
	}

	var in types.ChangePasswordInput
	if err := c.Bind(&in); err != nil {
		// 创建带有请求上下文的错误构建器
		errorBuilder := h.createErrorBuilder(ctx, c).
			With("user_id", user.ID).
			With("endpoint", "POST /api/v1/me/change-password")
		return errors.ErrBadRequest.
			Wrapf(errorBuilder.Wrapf(err, "JSON解析失败"), "请求参数格式错误")
	}

	// 验证输入
	if err := in.Validate(); err != nil {
		return err
	}

	// 将用户信息和请求信息添加到上下文，传递给下游服务
	ctxWithBuilder := oops.WithBuilder(ctx,
		h.createErrorBuilder(ctx, c).
			With("user_id", user.ID).
			With("endpoint", "POST /api/v1/me/change-password"))

	// 修改当前用户密码
	err := h.me.ChangePassword(ctxWithBuilder, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// createErrorBuilder 创建带有请求上下文信息的错误构建器 - 遵循 oops 最佳实践
func (h *MeHandler) createErrorBuilder(ctx context.Context, c echo.Context) oops.OopsErrorBuilder {
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
