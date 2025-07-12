package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
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
	user, ok := context.GetUserFromContext(ctx)
	if !ok {
		return errors.ErrUnauthorized.Errorf("用户未登录")
	}

	// 获取用户信息
	output, err := h.me.GetUserByID(ctx, user.ID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// UpdateUsername 更新当前用户用户名
func (h *MeHandler) UpdateUsername(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := context.GetUserFromContext(ctx)
	if !ok {
		return errors.ErrUnauthorized.Errorf("用户未登录")
	}

	var in types.UpdateUsernameInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新当前用户用户名
	out, err := h.me.UpdateUsername(ctx, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// UpdateEmail 更新当前用户邮箱
func (h *MeHandler) UpdateEmail(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := context.GetUserFromContext(ctx)
	if !ok {
		return errors.ErrUnauthorized.Errorf("用户未登录")
	}

	var in types.UpdateEmailInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新当前用户邮箱
	out, err := h.me.UpdateEmail(ctx, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// ChangePassword 修改当前用户密码
func (h *MeHandler) ChangePassword(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := context.GetUserFromContext(ctx)
	if !ok {
		return errors.ErrUnauthorized.Errorf("用户未登录")
	}

	var in types.ChangePasswordInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 修改当前用户密码
	err := h.me.ChangePassword(ctx, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}
