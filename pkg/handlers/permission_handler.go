package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// PermissionHandler 权限处理器
type PermissionHandler struct {
	orm               *ent.Client
	permissionService *services.PermissionService
}

// init 注册处理器
func init() {
	Register(new(PermissionHandler))
}

// Init 初始化处理器
func (h *PermissionHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.permissionService = c.Permission
	return nil
}

// Routes 注册路由
func (h *PermissionHandler) Routes(g *echo.Group) {
	permissions := g.Group("/api/v1/permissions")

	// 权限基本操作
	permissions.POST("", h.CreatePermission)
	permissions.GET("", h.ListPermissions)
	permissions.GET("/:id", h.GetPermission)
	permissions.PUT("/:id", h.UpdatePermission)
	permissions.DELETE("/:id", h.DeletePermission)

	// 权限分组
	permissions.GET("/groups", h.GetPermissionGroups)
}

// CreatePermission 创建权限
func (h *PermissionHandler) CreatePermission(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.CreatePermissionInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	permission, err := h.permissionService.CreatePermission(ctx, &input)
	if err != nil {
		return err
	}

	return Success(permission).JSON(c)
}

// UpdatePermission 更新权限
func (h *PermissionHandler) UpdatePermission(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	var input types.UpdatePermissionInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	permission, err := h.permissionService.UpdatePermission(ctx, id, &input)
	if err != nil {
		return err
	}

	return Success(permission).JSON(c)
}

// DeletePermission 删除权限
func (h *PermissionHandler) DeletePermission(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	err := h.permissionService.DeletePermission(ctx, id)
	if err != nil {
		return err
	}

	return SuccessWithMessage("删除成功").JSON(c)
}

// GetPermission 获取权限详情
func (h *PermissionHandler) GetPermission(c echo.Context) error {
	ctx := c.Request().Context()
	id := c.Param("id")

	permission, err := h.permissionService.GetPermission(ctx, id)
	if err != nil {
		return err
	}

	return Success(permission).JSON(c)
}

// ListPermissions 获取权限列表
func (h *PermissionHandler) ListPermissions(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.ListPermissionsInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	result, err := h.permissionService.ListPermissions(ctx, &input)
	if err != nil {
		return err
	}

	return Success(result).JSON(c)
}

// GetPermissionGroups 获取权限分组
func (h *PermissionHandler) GetPermissionGroups(c echo.Context) error {
	ctx := c.Request().Context()

	groups, err := h.permissionService.ListPermissionsByResource(ctx)
	if err != nil {
		return err
	}

	return Success(groups).JSON(c)
}
