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
	admin.POST("", h.Create)
	admin.GET("", h.List)
	admin.GET("/tree", h.Tree)
	admin.GET("/:id", h.Get)
	admin.PUT("/:id", h.Update)
	admin.DELETE("/:id", h.Delete)
	admin.GET("/:id/check-deletable", h.CheckDeletable)
	admin.POST("/sort", h.Sort)
	admin.POST("/:id/move", h.Move)
}

// Create 创建菜单
func (h *MenuHandler) Create(c echo.Context) error {
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
	out, err := h.menuService.Create(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// List 获取菜单列表
func (h *MenuHandler) List(c echo.Context) error {
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
	out, err := h.menuService.List(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Tree 获取菜单树
func (h *MenuHandler) Tree(c echo.Context) error {
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
	out, err := h.menuService.Tree(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Get 获取菜单详情
func (h *MenuHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	// 获取菜单信息
	output, err := h.menuService.GetByID(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// Update 更新菜单
func (h *MenuHandler) Update(c echo.Context) error {
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
	out, err := h.menuService.Update(ctx, menuID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Delete 删除菜单
func (h *MenuHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	// 删除菜单
	err := h.menuService.Delete(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// CheckDeletable 检查菜单是否可删除
func (h *MenuHandler) CheckDeletable(c echo.Context) error {
	ctx := c.Request().Context()
	menuID := c.Param("id")

	if menuID == "" {
		return errors.ErrBadRequest.Errorf("菜单ID不能为空")
	}

	// 检查菜单是否可删除
	output, err := h.menuService.CheckDeletable(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// Sort 菜单排序
func (h *MenuHandler) Sort(c echo.Context) error {
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
	err := h.menuService.Sort(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// Move 移动菜单
func (h *MenuHandler) Move(c echo.Context) error {
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
	err := h.menuService.Move(ctx, menuID, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}
