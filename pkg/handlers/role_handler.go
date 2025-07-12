package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// RoleHandler 角色管理处理器
type RoleHandler struct {
	orm         *ent.Client
	roleService *services.RoleService
	authService *services.AuthService
}

// init 注册handler
func init() {
	Register(new(RoleHandler))
}

// Init 初始化依赖
func (h *RoleHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.roleService = c.Role
	h.authService = c.Auth
	return nil
}

// Routes 注册路由
func (h *RoleHandler) Routes(g *echo.Group) {
	// 角色管理路由
	admin := g.Group("/api/v1/admin/roles")
	admin.Use(middleware.RequireAuth(h.authService)) // 先验证用户身份
	// TODO: 添加管理员权限检查中间件
	// admin.Use(authMw.RequireRole("admin"))

	// 角色CRUD相关路由
	admin.POST("", h.CreateRole)
	admin.GET("", h.ListRoles)
	admin.GET("/stats", h.GetRoleStats)
	admin.GET("/:id", h.GetRole)
	admin.PUT("/:id", h.UpdateRole)
	admin.DELETE("/:id", h.DeleteRole)

	// 角色维护相关路由
	admin.GET("/:id/check-deletable", h.CheckRoleDeletable)
	admin.POST("/batch/delete", h.BatchDeleteRoles)

	// 角色权限分配相关路由
	admin.PUT("/:id/menus", h.AssignRoleMenus)
	admin.GET("/:id/menus", h.GetRoleMenus)
	admin.PUT("/:id/users", h.AssignRoleUsers)
	admin.GET("/:id/users", h.GetRoleUsers)
}

// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.CreateRoleInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 创建角色
	out, err := h.roleService.Create(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// ListRoles 获取角色列表
func (h *RoleHandler) ListRoles(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.ListRolesInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 获取角色列表
	out, err := h.roleService.List(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// GetRole 获取角色详情
func (h *RoleHandler) GetRole(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	// 获取角色信息
	output, err := h.roleService.Get(ctx, roleID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	var in types.UpdateRoleInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新角色
	out, err := h.roleService.Update(ctx, roleID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	// 删除角色
	err := h.roleService.Delete(ctx, roleID)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// CheckRoleDeletable 检查角色是否可删除
func (h *RoleHandler) CheckRoleDeletable(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	// 检查角色是否可删除
	output, err := h.roleService.CheckDeletable(ctx, roleID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// BatchDeleteRoles 批量删除角色
func (h *RoleHandler) BatchDeleteRoles(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.BatchDeleteRolesInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 批量删除角色
	for _, roleID := range in.RoleIds {
		if err := h.roleService.Delete(ctx, roleID); err != nil {
			return err
		}
	}

	return Success(c, nil)
}

// GetRoleStats 获取角色统计
func (h *RoleHandler) GetRoleStats(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取角色统计
	stats, err := h.roleService.GetStats(ctx)
	if err != nil {
		return err
	}

	return Success(c, stats)
}

// AssignRoleMenus 分配角色菜单权限
func (h *RoleHandler) AssignRoleMenus(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	var in types.AssignRoleMenusInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 分配菜单权限
	err := h.roleService.AssignMenus(ctx, roleID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// GetRoleMenus 获取角色菜单权限
func (h *RoleHandler) GetRoleMenus(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	// 获取角色菜单权限
	output, err := h.roleService.GetRoleMenus(ctx, roleID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// AssignRoleUsers 分配角色用户
func (h *RoleHandler) AssignRoleUsers(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	var in types.AssignRoleUsersInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 分配用户
	err := h.roleService.AssignUsers(ctx, roleID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// GetRoleUsers 获取角色用户列表
func (h *RoleHandler) GetRoleUsers(c echo.Context) error {
	ctx := c.Request().Context()
	roleID := c.Param("id")

	if roleID == "" {
		return errors.ErrBadRequest.Errorf("角色ID不能为空")
	}

	var in types.PageInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 获取角色用户列表
	output, err := h.roleService.GetRoleUsers(ctx, roleID, &in)
	if err != nil {
		return err
	}

	return Success(c, output)
}
