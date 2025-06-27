package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	appContext "github.com/liukeshao/echo-template/pkg/context"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// UserHandler 用户处理器
type UserHandler struct {
	orm         *ent.Client
	userService *services.UserService
	authService *services.AuthService
}

// init 注册handler
func init() {
	Register(new(UserHandler))
}

// Init 初始化依赖
func (h *UserHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.userService = c.User
	h.authService = c.Auth
	return nil
}

// Routes 注册路由
func (h *UserHandler) Routes(g *echo.Group) {
	// 需要认证的路由组
	authMw := middleware.NewAuthMiddleware(h.orm, h.authService)

	protected := g.Group("/api/v1/users")
	protected.Use(authMw.RequireAuth) // 先验证用户身份

	// 当前用户相关路由（不需要额外权限，只要登录即可）
	protected.GET("/me", h.GetCurrentUser)                             // 获取当前用户信息
	protected.PUT("/me", h.UpdateCurrentUser)                          // 更新当前用户信息
	protected.POST("/me/change-password", h.ChangeCurrentUserPassword) // 修改当前用户密码
}

// getUserID 从Echo context中获取当前用户ID
func getUserID(c echo.Context) string {
	user, ok := appContext.GetUserFromEcho(c)
	if !ok || user == nil {
		return ""
	}
	return user.ID
}

// CreateUser 创建用户
func (h *UserHandler) CreateUser(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.CreateUserInput
	if err := c.Bind(&input); err != nil {
		return errors.ErrBadRequest("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 创建用户
	output, err := h.userService.CreateUser(ctx, &input)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// GetCurrentUser 获取当前用户信息
func (h *UserHandler) GetCurrentUser(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	userID := getUserID(c)
	if userID == "" {
		return errors.ErrUnauthorized("用户未登录")
	}

	// 获取用户信息
	output, err := h.userService.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// UpdateCurrentUser 更新当前用户信息
func (h *UserHandler) UpdateCurrentUser(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	userID := getUserID(c)
	if userID == "" {
		return errors.ErrUnauthorized("用户未登录")
	}

	var input types.UpdateUserInput
	if err := c.Bind(&input); err != nil {
		return errors.ErrBadRequest("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 普通用户不能修改状态
	input.Status = nil

	// 更新当前用户信息
	output, err := h.userService.UpdateUser(ctx, userID, &input)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// ChangeCurrentUserPassword 修改当前用户密码
func (h *UserHandler) ChangeCurrentUserPassword(c echo.Context) error {
	ctx := c.Request().Context()

	// 从上下文获取当前用户ID
	userID := getUserID(c)
	if userID == "" {
		return errors.ErrUnauthorized("用户未登录")
	}

	var input types.ChangePasswordInput
	if err := c.Bind(&input); err != nil {
		return errors.ErrBadRequest("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 修改当前用户密码
	err := h.userService.ChangePassword(ctx, userID, &input)
	if err != nil {
		return err
	}

	return Success(map[string]string{"message": "密码修改成功"}).JSON(c)
}
