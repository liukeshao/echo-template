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

// RoleHandler 角色处理器
type RoleHandler struct {
	orm               *ent.Client
	roleService       *services.RoleService
	permissionService *services.PermissionService
}

// init 注册处理器
func init() {
	Register(new(RoleHandler))
}

// Init 初始化处理器
func (h *RoleHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.roleService = c.Role
	h.permissionService = c.Permission
	return nil
}

// Routes 注册路由
func (h *RoleHandler) Routes(g *echo.Group) {
	// 需要认证的路由组
	authMw := middleware.NewAuthMiddleware(h.orm)
	permMw := middleware.NewPermissionMiddleware(h.orm, h.roleService)

	roles := g.Group("/api/v1/roles")
	roles.Use(authMw.RequireAuth) // 先验证用户身份

	// 角色基本操作（需要对应权限）
	roles.POST("", h.CreateRole, permMw.RequirePermission("role.create"))
	roles.GET("", h.ListRoles, permMw.RequirePermission("role.list"))
	roles.GET("/:id", h.GetRole, permMw.RequirePermission("role.view"))
	roles.PUT("/:id", h.UpdateRole, permMw.RequirePermission("role.update"))
	roles.DELETE("/:id", h.DeleteRole, permMw.RequirePermission("role.delete"))

	// 用户角色管理（需要用户管理权限）
	roles.POST("/assign", h.AssignRoles, permMw.RequirePermission("user.role.assign"))
	roles.POST("/revoke", h.RevokeRoles, permMw.RequirePermission("user.role.revoke"))
	roles.GET("/users/:user_id", h.GetUserRoles, permMw.RequirePermission("user.role.view"))

	// 角色权限管理（需要权限管理权限）
	roles.POST("/:id/permissions", h.AssignPermissions, permMw.RequirePermission("role.permission.assign"))
	roles.GET("/:id/permissions", h.GetRolePermissions, permMw.RequirePermission("role.permission.view"))

	// 角色菜单管理（需要菜单管理权限）
	roles.POST("/:id/menus", h.AssignRoleMenus, permMw.RequirePermission("role.menu.assign"))
	roles.GET("/:id/menus", h.GetRoleMenus, permMw.RequirePermission("role.menu.view"))
	roles.GET("/:id/menus/permissions", h.GetRoleMenuPermissions, permMw.RequirePermission("role.menu.view"))

	// 用户菜单查询（需要菜单查看权限）
	roles.GET("/users/menus", h.GetUserMenus, permMw.RequirePermission("menu.view"))
}

// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.CreateRoleInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	role, err := h.roleService.CreateRole(ctx, &input)
	if err != nil {
		return err
	}

	return Success(role).JSON(c)
}

// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	var input types.UpdateRoleInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	role, err := h.roleService.UpdateRole(ctx, id, &input)
	if err != nil {
		return err
	}

	return Success(role).JSON(c)
}

// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	err := h.roleService.DeleteRole(ctx, id)
	if err != nil {
		return err
	}

	return SuccessWithMessage("删除成功").JSON(c)
}

// GetRole 获取角色详情
func (h *RoleHandler) GetRole(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	role, err := h.roleService.GetRole(ctx, id)
	if err != nil {
		return err
	}

	return Success(role).JSON(c)
}

// ListRoles 获取角色列表
func (h *RoleHandler) ListRoles(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.ListRolesInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	result, err := h.roleService.ListRoles(ctx, &input)
	if err != nil {
		return err
	}

	return Success(result).JSON(c)
}

// AssignRoles 分配角色给用户
func (h *RoleHandler) AssignRoles(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.AssignRoleInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 获取当前用户ID作为授权者
	granterID := ""
	if user, ok := appContext.GetUserFromEcho(c); ok && user != nil {
		granterID = user.ID
	}

	err := h.roleService.AssignRoles(ctx, &input, granterID)
	if err != nil {
		return err
	}

	return SuccessWithMessage("角色分配成功").JSON(c)
}

// RevokeRoles 撤销用户角色
func (h *RoleHandler) RevokeRoles(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.RevokeRoleInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	err := h.roleService.RevokeRoles(ctx, &input)
	if err != nil {
		return err
	}

	return SuccessWithMessage("角色撤销成功").JSON(c)
}

// GetUserRoles 获取用户角色列表
func (h *RoleHandler) GetUserRoles(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Param("user_id")

	if userID == "" {
		return errors.BadRequestError("用户ID不能为空")
	}

	roles, err := h.roleService.GetUserRoles(ctx, userID)
	if err != nil {
		return err
	}

	return Success(roles).JSON(c)
}

// AssignPermissions 为角色分配权限
func (h *RoleHandler) AssignPermissions(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	var input types.AssignPermissionInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 设置角色ID
	input.RoleID = roleID

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	err := h.permissionService.AssignPermissions(ctx, &input)
	if err != nil {
		return err
	}

	return SuccessWithMessage("权限分配成功").JSON(c)
}

// GetRolePermissions 获取角色权限列表
func (h *RoleHandler) GetRolePermissions(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.BadRequestError("角色ID不能为空")
	}

	permissions, err := h.permissionService.GetRolePermissions(ctx, roleID)
	if err != nil {
		return err
	}

	return Success(permissions).JSON(c)
}

// SuccessWithMessage 成功响应（只有消息）
func SuccessWithMessage(message string) *ResponseBuilder {
	return NewResponse().
		WithCode(errors.OK).
		WithMessage(message).
		WithData(nil)
}

// AssignRoleMenus 为角色分配菜单
func (h *RoleHandler) AssignRoleMenus(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	var input types.AssignRoleMenusInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 设置角色ID
	input.RoleID = roleID

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	err := h.roleService.AssignRoleMenus(ctx, &input)
	if err != nil {
		return err
	}

	return SuccessWithMessage("角色菜单分配成功").JSON(c)
}

// GetRoleMenus 获取角色菜单列表
func (h *RoleHandler) GetRoleMenus(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.BadRequestError("角色ID不能为空")
	}

	result, err := h.roleService.GetRoleMenus(ctx, roleID)
	if err != nil {
		return err
	}

	return Success(result).JSON(c)
}

// GetRoleMenuPermissions 获取角色菜单权限详情
func (h *RoleHandler) GetRoleMenuPermissions(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.BadRequestError("角色ID不能为空")
	}

	result, err := h.roleService.GetRoleMenuPermissions(ctx, roleID)
	if err != nil {
		return err
	}

	return Success(result).JSON(c)
}

// GetUserMenus 获取用户可访问的菜单
func (h *RoleHandler) GetUserMenus(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.GetUserMenusInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	result, err := h.roleService.GetUserMenus(ctx, &input)
	if err != nil {
		return err
	}

	return Success(result).JSON(c)
}
