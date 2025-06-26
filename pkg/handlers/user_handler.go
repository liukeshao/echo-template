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
	roleService *services.RoleService
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
	h.roleService = c.Role
	h.authService = c.Auth
	return nil
}

// Routes 注册路由
func (h *UserHandler) Routes(g *echo.Group) {
	// 需要认证的路由组
	authMw := middleware.NewAuthMiddleware(h.orm, h.authService)
	permMw := middleware.NewPermissionMiddleware(h.orm, h.roleService)

	protected := g.Group("/api/v1/users")
	protected.Use(authMw.RequireAuth) // 先验证用户身份

	// 用户管理路由（需要对应权限）
	protected.POST("", h.CreateUser, permMw.RequirePermission("user.create"))                           // 需要创建用户权限
	protected.GET("", h.ListUsers, permMw.RequirePermission("user.list"))                               // 需要查看用户列表权限
	protected.GET("/stats", h.GetUserStats, permMw.RequirePermission("user.view"))                      // 需要查看用户权限
	protected.GET("/:id", h.GetUserByID, permMw.RequirePermission("user.view"))                         // 需要查看用户权限
	protected.PUT("/:id", h.UpdateUser, permMw.RequirePermission("user.update"))                        // 需要更新用户权限
	protected.DELETE("/:id", h.DeleteUser, permMw.RequirePermission("user.delete"))                     // 需要删除用户权限
	protected.POST("/:id/change-password", h.ChangePassword, permMw.RequirePermission("user.password")) // 需要修改密码权限

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
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
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

// ListUsers 获取用户列表
func (h *UserHandler) ListUsers(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.ListUsersInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 获取用户列表
	output, err := h.userService.ListUsers(ctx, &input)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// GetUserByID 根据ID获取用户
func (h *UserHandler) GetUserByID(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Param("id")
	if userID == "" {
		return errors.BadRequestError("用户ID不能为空")
	}

	// 获取用户
	output, err := h.userService.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// UpdateUser 更新用户
func (h *UserHandler) UpdateUser(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Param("id")
	if userID == "" {
		return errors.BadRequestError("用户ID不能为空")
	}

	var input types.UpdateUserInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 更新用户
	output, err := h.userService.UpdateUser(ctx, userID, &input)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// DeleteUser 删除用户
func (h *UserHandler) DeleteUser(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Param("id")
	if userID == "" {
		return errors.BadRequestError("用户ID不能为空")
	}

	// 删除用户
	err := h.userService.DeleteUser(ctx, userID)
	if err != nil {
		return err
	}

	return Success(map[string]string{"message": "用户删除成功"}).JSON(c)
}

// ChangePassword 修改用户密码
func (h *UserHandler) ChangePassword(c echo.Context) error {
	ctx := c.Request().Context()

	userID := c.Param("id")
	if userID == "" {
		return errors.BadRequestError("用户ID不能为空")
	}

	var input types.ChangePasswordInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 修改密码
	err := h.userService.ChangePassword(ctx, userID, &input)
	if err != nil {
		return err
	}

	return Success(map[string]string{"message": "密码修改成功"}).JSON(c)
}

// GetUserStats 获取用户统计
func (h *UserHandler) GetUserStats(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取统计信息
	output, err := h.userService.GetUserStats(ctx)
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
		return errors.UnauthorizedError("用户未登录")
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
		return errors.UnauthorizedError("用户未登录")
	}

	var input types.UpdateUserInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
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
		return errors.UnauthorizedError("用户未登录")
	}

	var input types.ChangePasswordInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
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
