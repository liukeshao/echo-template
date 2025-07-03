package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// MeHandler 用户处理器
type MeHandler struct {
	orm         *ent.Client
	userService *services.UserService
	authService *services.AuthService
}

// init 注册handler
func init() {
	Register(new(MeHandler))
}

// Init 初始化依赖
func (h *MeHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.userService = c.User
	h.authService = c.Auth
	return nil
}

// Routes 注册路由
func (h *MeHandler) Routes(g *echo.Group) {
	// 需要认证的路由组
	authMw := middleware.NewAuthMiddleware(h.orm, h.authService)

	protected := g.Group("/api/v1/me")
	protected.Use(authMw.RequireAuth) // 先验证用户身份

	// 当前用户相关路由（不需要额外权限，只要登录即可）
	protected.GET("", h.Get)
	protected.PUT("", h.Update)
	protected.POST("/change-password", h.ChangePassword)
}

// Get 获取当前用户信息
func (h *MeHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := context.GetUserFromEcho(c)
	if !ok {
		return errors.ErrUnauthorized.Errorf("用户未登录")
	}

	// 获取用户信息
	output, err := h.userService.GetUserByID(ctx, user.ID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// Update 更新当前用户信息
func (h *MeHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := context.GetUserFromEcho(c)
	if !ok {
		return errors.ErrUnauthorized.Errorf("用户未登录")
	}

	var in types.UpdateMeInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新当前用户信息
	out, err := h.userService.UpdateMe(ctx, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// ChangePassword 修改当前用户密码
func (h *MeHandler) ChangePassword(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	user, ok := context.GetUserFromEcho(c)
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
	err := h.userService.ChangePassword(ctx, user.ID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}
