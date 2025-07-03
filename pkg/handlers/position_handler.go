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
	// 需要认证的路由组
	authMw := middleware.NewAuthMiddleware(h.orm, h.authService)

	// 岗位管理路由
	admin := g.Group("/api/v1/admin/positions")
	admin.Use(authMw.RequireAuth) // 先验证用户身份
	// TODO: 添加管理员权限检查中间件
	// admin.Use(authMw.RequireRole("admin"))

	// 岗位CRUD相关路由
	admin.POST("", h.CreatePosition)
	admin.GET("", h.ListPositions)
	admin.GET("/stats", h.GetPositionStats)
	admin.GET("/:id", h.GetPosition)
	admin.PUT("/:id", h.UpdatePosition)
	admin.DELETE("/:id", h.DeletePosition)

	// 岗位维护相关路由
	admin.POST("/sort", h.SortPositions)
	admin.GET("/:id/check-deletable", h.CheckPositionDeletable)
}

// CreatePosition 创建岗位
func (h *PositionHandler) CreatePosition(c echo.Context) error {
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
	out, err := h.positionService.CreatePosition(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// ListPositions 获取岗位列表
func (h *PositionHandler) ListPositions(c echo.Context) error {
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
	out, err := h.positionService.ListPositions(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// GetPosition 获取岗位详情
func (h *PositionHandler) GetPosition(c echo.Context) error {
	ctx := c.Request().Context()
	positionID := c.Param("id")

	if positionID == "" {
		return errors.ErrBadRequest.Errorf("岗位ID不能为空")
	}

	// 获取岗位信息
	output, err := h.positionService.GetPositionByID(ctx, positionID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// UpdatePosition 更新岗位
func (h *PositionHandler) UpdatePosition(c echo.Context) error {
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
	out, err := h.positionService.UpdatePosition(ctx, positionID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// DeletePosition 删除岗位
func (h *PositionHandler) DeletePosition(c echo.Context) error {
	ctx := c.Request().Context()
	positionID := c.Param("id")

	if positionID == "" {
		return errors.ErrBadRequest.Errorf("岗位ID不能为空")
	}

	// 删除岗位
	err := h.positionService.DeletePosition(ctx, positionID)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// SortPositions 批量更新岗位排序
func (h *PositionHandler) SortPositions(c echo.Context) error {
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
	err := h.positionService.SortPositions(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// CheckPositionDeletable 检查岗位是否可删除
func (h *PositionHandler) CheckPositionDeletable(c echo.Context) error {
	ctx := c.Request().Context()
	positionID := c.Param("id")

	if positionID == "" {
		return errors.ErrBadRequest.Errorf("岗位ID不能为空")
	}

	// 检查是否可删除
	output, err := h.positionService.CheckPositionDeletable(ctx, positionID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// GetPositionStats 获取岗位统计信息
func (h *PositionHandler) GetPositionStats(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取统计信息
	output, err := h.positionService.GetPositionStats(ctx)
	if err != nil {
		return err
	}

	return Success(c, output)
}
