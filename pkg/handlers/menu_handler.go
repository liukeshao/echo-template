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
}

// init 注册handler
func init() {
	Register(new(MenuHandler))
}

// Init 初始化依赖
func (h *MenuHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.menuService = c.Menu
	return nil
}

// Routes 注册路由
func (h *MenuHandler) Routes(g *echo.Group) {
	// 需要认证的路由组
	authMw := middleware.NewAuthMiddleware(h.orm)
	protected := g.Group("/api/v1/menus")
	protected.Use(authMw.RequireAuth)

	// 菜单管理路由（需要认证）
	protected.POST("", h.CreateMenu)           // 创建菜单
	protected.GET("", h.ListMenus)             // 获取菜单列表
	protected.GET("/tree", h.GetMenuTree)      // 获取菜单树
	protected.GET("/stats", h.GetMenuStats)    // 获取菜单统计
	protected.GET("/:id", h.GetMenuByID)       // 根据ID获取菜单
	protected.PUT("/:id", h.UpdateMenu)        // 更新菜单
	protected.DELETE("/:id", h.DeleteMenu)     // 删除菜单
	protected.PUT("/order", h.UpdateMenuOrder) // 更新菜单排序
}

// CreateMenu 创建菜单
func (h *MenuHandler) CreateMenu(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.CreateMenuInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 创建菜单
	output, err := h.menuService.CreateMenu(ctx, &input)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// ListMenus 获取菜单列表
func (h *MenuHandler) ListMenus(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.ListMenusInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 获取菜单列表
	output, err := h.menuService.ListMenus(ctx, &input)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// GetMenuTree 获取菜单树
func (h *MenuHandler) GetMenuTree(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.ListMenusInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 强制设置为树形模式
	input.TreeMode = true

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 获取菜单树
	output, err := h.menuService.ListMenus(ctx, &input)
	if err != nil {
		return err
	}

	// 转换为菜单树输出格式
	treeOutput := &types.MenuTreeOutput{
		Menus: output.Menus,
		Total: output.Total,
	}

	return Success(treeOutput).JSON(c)
}

// GetMenuByID 根据ID获取菜单
func (h *MenuHandler) GetMenuByID(c echo.Context) error {
	ctx := c.Request().Context()

	menuID := c.Param("id")
	if menuID == "" {
		return errors.BadRequestError("菜单ID不能为空")
	}

	// 获取菜单
	output, err := h.menuService.GetMenuByID(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// UpdateMenu 更新菜单
func (h *MenuHandler) UpdateMenu(c echo.Context) error {
	ctx := c.Request().Context()

	menuID := c.Param("id")
	if menuID == "" {
		return errors.BadRequestError("菜单ID不能为空")
	}

	var input types.UpdateMenuInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 更新菜单
	output, err := h.menuService.UpdateMenu(ctx, menuID, &input)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}

// DeleteMenu 删除菜单
func (h *MenuHandler) DeleteMenu(c echo.Context) error {
	ctx := c.Request().Context()

	menuID := c.Param("id")
	if menuID == "" {
		return errors.BadRequestError("菜单ID不能为空")
	}

	// 删除菜单
	err := h.menuService.DeleteMenu(ctx, menuID)
	if err != nil {
		return err
	}

	return Success(map[string]string{"message": "菜单删除成功"}).JSON(c)
}

// UpdateMenuOrder 更新菜单排序
func (h *MenuHandler) UpdateMenuOrder(c echo.Context) error {
	ctx := c.Request().Context()

	var input types.UpdateMenuOrderInput
	if err := c.Bind(&input); err != nil {
		return errors.BadRequestError("请求参数格式错误").With("error", err.Error())
	}

	// 验证输入
	if errorDetails := input.Validate(); len(errorDetails) > 0 {
		return ValidationError("验证失败", errorDetails).JSON(c)
	}

	// 更新菜单排序
	err := h.menuService.UpdateMenuOrder(ctx, &input)
	if err != nil {
		return err
	}

	return Success(map[string]string{"message": "菜单排序更新成功"}).JSON(c)
}

// GetMenuStats 获取菜单统计
func (h *MenuHandler) GetMenuStats(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取菜单统计
	output, err := h.menuService.GetMenuStats(ctx)
	if err != nil {
		return err
	}

	return Success(output).JSON(c)
}
