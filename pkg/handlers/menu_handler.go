package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// MenuHandler 菜单管理处理器
type MenuHandler struct {
	orm         *ent.Client
	menuService *services.MenuService
	authService *services.AuthService
}

// init 注册handler
func init() {
	Register(new(MenuHandler))
}

// Init 初始化依赖
func (h *MenuHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.menuService = c.Menu
	h.authService = c.Auth
	return nil
}

// Routes 注册路由
func (h *MenuHandler) Routes(g *echo.Group) {
	admin := g.Group("/api/v1/admin/menus")
	admin.Use(middleware.RequireAuth(h.authService)) // 先验证用户身份
	// TODO: 添加管理员权限检查中间件
	// admin.Use(authMw.RequireRole("admin"))

	// 菜单管理相关路由
	admin.POST("", h.CreateMenu)
	admin.GET("", h.ListMenus)
	admin.GET("/tree", h.GetMenuTree)
	admin.GET("/:id", h.GetMenu)
	admin.PUT("/:id", h.UpdateMenu)
	admin.DELETE("/:id", h.DeleteMenu)
	admin.GET("/:id/check-deletable", h.CheckMenuDeletable)
	admin.POST("/sort", h.SortMenus)
	admin.POST("/:id/move", h.MoveMenu)
}

// CreateMenu 创建菜单
func (h *MenuHandler) CreateMenu(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.CreateMenuInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 创建菜单
	out, err := h.menuService.CreateMenu(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// ListMenus 获取菜单列表
func (h *MenuHandler) ListMenus(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.ListMenusInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 获取菜单列表
	out, err := h.menuService.ListMenus(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// GetMenuTree 获取菜单树
func (h *MenuHandler) GetMenuTree(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.ListMenusInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 获取菜单树
	out, err := h.menuService.GetMenuTree(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// GetMenu 获取菜单详情
func (h *MenuHandler) GetMenu(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	// 获取菜单信息
	output, err := h.menuService.GetMenuByID(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// UpdateMenu 更新菜单
func (h *MenuHandler) UpdateMenu(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	var in types.UpdateMenuInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新菜单
	out, err := h.menuService.UpdateMenu(ctx, menuID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// DeleteMenu 删除菜单
func (h *MenuHandler) DeleteMenu(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	// 删除菜单
	err := h.menuService.DeleteMenu(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// CheckMenuDeletable 检查菜单是否可删除
func (h *MenuHandler) CheckMenuDeletable(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	// 检查菜单是否可删除
	output, err := h.menuService.CheckMenuDeletable(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// SortMenus 菜单排序
func (h *MenuHandler) SortMenus(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.SortMenuInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 执行排序
	err := h.menuService.SortMenus(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// MoveMenu 移动菜单
func (h *MenuHandler) MoveMenu(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	var in types.MoveMenuInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 移动菜单
	err := h.menuService.MoveMenu(ctx, menuID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}
