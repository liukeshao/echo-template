package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// UserHandler 用户管理处理器（管理员版本）
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
	admin := g.Group("/api/v1/admin/users")
	admin.Use(middleware.RequireAuth(h.authService)) // 先验证用户身份
	// TODO: 添加管理员权限检查中间件
	// admin.Use(authMw.RequireRole("admin"))

	// 用户管理相关路由
	admin.POST("", h.Create)
	admin.GET("", h.List)
	admin.GET("/stats", h.Stats)
	admin.GET("/:id", h.Get)
	admin.PUT("/:id", h.Update)
	admin.DELETE("/:id", h.Delete)
	admin.POST("/:id/reset-password", h.ResetPassword)
	admin.PUT("/:id/status", h.SetStatus)
	admin.POST("/batch/status", h.BatchUpdateStatus)
	admin.POST("/batch/delete", h.BatchDelete)
}

// Create 创建用户
func (h *UserHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.CreateUserInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 创建用户
	out, err := h.userService.Create(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// List 获取用户列表
func (h *UserHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.ListUsersInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 获取用户列表
	out, err := h.userService.List(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Get 获取用户详情
func (h *UserHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Param("id")

	if userID == "" {
		return errors.ErrBadRequest.Errorf("用户ID不能为空")
	}

	// 获取用户信息
	output, err := h.userService.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// Update 更新用户
func (h *UserHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Param("id")

	if userID == "" {
		return errors.ErrBadRequest.Errorf("用户ID不能为空")
	}

	var in types.UpdateUserInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新用户
	out, err := h.userService.Update(ctx, userID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Delete 删除用户
func (h *UserHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Param("id")

	if userID == "" {
		return errors.ErrBadRequest.Errorf("用户ID不能为空")
	}

	// 删除用户
	err := h.userService.Delete(ctx, userID)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// ResetPassword 重置用户密码
func (h *UserHandler) ResetPassword(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Param("id")

	if userID == "" {
		return errors.ErrBadRequest.Errorf("用户ID不能为空")
	}

	var in types.ResetPasswordInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 重置密码
	err := h.userService.ResetPassword(ctx, userID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// SetStatus 设置用户状态
func (h *UserHandler) SetStatus(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Param("id")

	if userID == "" {
		return errors.ErrBadRequest.Errorf("用户ID不能为空")
	}

	var in types.SetUserStatusInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 设置用户状态
	err := h.userService.SetUserStatus(ctx, userID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// BatchUpdateStatus 批量更新用户状态
func (h *UserHandler) BatchUpdateStatus(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.BatchUpdateStatusInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 批量更新状态
	err := h.userService.BatchUpdateStatus(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// BatchDelete 批量删除用户
func (h *UserHandler) BatchDelete(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.BatchOperationInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 批量删除用户
	err := h.userService.BatchDeleteUsers(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// Stats 获取用户统计信息
func (h *UserHandler) Stats(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取用户统计
	stats, err := h.userService.Stats(ctx)
	if err != nil {
		return err
	}

	return Success(c, stats)
}
