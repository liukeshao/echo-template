package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// PositionHandler 岗位管理处理器
type PositionHandler struct {
	orm             *ent.Client
	positionService *services.PositionService
	authService     *services.AuthService
}

// init 注册handler
func init() {
	Register(new(PositionHandler))
}

// Init 初始化依赖
func (h *PositionHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.positionService = c.Position
	h.authService = c.Auth
	return nil
}

// Routes 注册路由
func (h *PositionHandler) Routes(g *echo.Group) {
	// 岗位管理路由
	admin := g.Group("/api/v1/admin/positions")
	admin.Use(middleware.RequireAuth(h.authService)) // 先验证用户身份
	// TODO: 添加管理员权限检查中间件
	// admin.Use(authMw.RequireRole("admin"))

	// 岗位CRUD相关路由
	admin.POST("", h.Create)
	admin.GET("", h.List)
	admin.GET("/stats", h.Stats)
	admin.GET("/:id", h.Get)
	admin.PUT("/:id", h.Update)
	admin.DELETE("/:id", h.Delete)

	// 岗位维护相关路由
	admin.POST("/sort", h.Sort)
	admin.GET("/:id/check-deletable", h.CheckDeletable)
}

// Create 创建岗位
func (h *PositionHandler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.CreatePositionInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 创建岗位
	out, err := h.positionService.Create(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// List 获取岗位列表
func (h *PositionHandler) List(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.ListPositionsInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 获取岗位列表
	out, err := h.positionService.List(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Get 获取岗位详情
func (h *PositionHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	positionID := c.Param("id")

	if positionID == "" {
		return errors.ErrBadRequest.Errorf("岗位ID不能为空")
	}

	// 获取岗位信息
	output, err := h.positionService.GetByID(ctx, positionID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// Update 更新岗位
func (h *PositionHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()
	positionID := c.Param("id")

	if positionID == "" {
		return errors.ErrBadRequest.Errorf("岗位ID不能为空")
	}

	var in types.UpdatePositionInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新岗位
	out, err := h.positionService.Update(ctx, positionID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// Delete 删除岗位
func (h *PositionHandler) Delete(c echo.Context) error {
	ctx := c.Request().Context()
	positionID := c.Param("id")

	if positionID == "" {
		return errors.ErrBadRequest.Errorf("岗位ID不能为空")
	}

	// 删除岗位
	err := h.positionService.Delete(ctx, positionID)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// Sort 批量更新岗位排序
func (h *PositionHandler) Sort(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.SortPositionInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新排序
	err := h.positionService.Sort(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// CheckDeletable 检查岗位是否可删除
func (h *PositionHandler) CheckDeletable(c echo.Context) error {
	ctx := c.Request().Context()
	positionID := c.Param("id")

	if positionID == "" {
		return errors.ErrBadRequest.Errorf("岗位ID不能为空")
	}

	// 检查是否可删除
	output, err := h.positionService.CheckDeletable(ctx, positionID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// Stats 获取岗位统计信息
func (h *PositionHandler) Stats(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取统计信息
	output, err := h.positionService.Stats(ctx)
	if err != nil {
		return err
	}

	return Success(c, output)
}
