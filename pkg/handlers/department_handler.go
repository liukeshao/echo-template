package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// DepartmentHandler 部门管理处理器
type DepartmentHandler struct {
	orm               *ent.Client
	departmentService *services.DepartmentService
	authService       *services.AuthService
}

// init 注册handler
func init() {
	Register(new(DepartmentHandler))
}

// Init 初始化依赖
func (h *DepartmentHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.departmentService = c.Department
	h.authService = c.Auth
	return nil
}

// Routes 注册路由
func (h *DepartmentHandler) Routes(g *echo.Group) {
	// 需要认证的路由组
	authMw := middleware.NewAuthMiddleware(h.orm, h.authService)

	// 部门管理路由
	admin := g.Group("/api/v1/admin/departments")
	admin.Use(authMw.RequireAuth) // 先验证用户身份
	// TODO: 添加管理员权限检查中间件
	// admin.Use(authMw.RequireRole("admin"))

	// 部门CRUD相关路由
	admin.POST("", h.CreateDepartment)
	admin.GET("", h.ListDepartments)
	admin.GET("/tree", h.GetDepartmentTree)
	admin.GET("/stats", h.GetDepartmentStats)
	admin.GET("/:id", h.GetDepartment)
	admin.PUT("/:id", h.UpdateDepartment)
	admin.DELETE("/:id", h.DeleteDepartment)

	// 部门结构维护相关路由
	admin.PUT("/:id/move", h.MoveDepartment)
	admin.POST("/sort", h.SortDepartments)
	admin.GET("/:id/check-deletable", h.CheckDepartmentDeletable)
}

// CreateDepartment 创建部门
func (h *DepartmentHandler) CreateDepartment(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.CreateDepartmentInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 创建部门
	out, err := h.departmentService.CreateDepartment(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// ListDepartments 获取部门列表
func (h *DepartmentHandler) ListDepartments(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.ListDepartmentsInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 获取部门列表
	out, err := h.departmentService.ListDepartments(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// GetDepartmentTree 获取部门树形结构
func (h *DepartmentHandler) GetDepartmentTree(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取部门树形结构
	out, err := h.departmentService.GetDepartmentTree(ctx)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// GetDepartment 获取部门详情
func (h *DepartmentHandler) GetDepartment(c echo.Context) error {
	ctx := c.Request().Context()
	departmentID := c.Param("id")

	if departmentID == "" {
		return errors.ErrBadRequest.Errorf("部门ID不能为空")
	}

	// 获取部门信息
	output, err := h.departmentService.GetDepartmentByID(ctx, departmentID)
	if err != nil {
		return err
	}

	return Success(c, output)
}

// UpdateDepartment 更新部门
func (h *DepartmentHandler) UpdateDepartment(c echo.Context) error {
	ctx := c.Request().Context()
	departmentID := c.Param("id")

	if departmentID == "" {
		return errors.ErrBadRequest.Errorf("部门ID不能为空")
	}

	var in types.UpdateDepartmentInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 更新部门
	out, err := h.departmentService.UpdateDepartment(ctx, departmentID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// DeleteDepartment 删除部门
func (h *DepartmentHandler) DeleteDepartment(c echo.Context) error {
	ctx := c.Request().Context()
	departmentID := c.Param("id")

	if departmentID == "" {
		return errors.ErrBadRequest.Errorf("部门ID不能为空")
	}

	// 删除部门
	err := h.departmentService.DeleteDepartment(ctx, departmentID)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// MoveDepartment 移动部门（调整父节点）
func (h *DepartmentHandler) MoveDepartment(c echo.Context) error {
	ctx := c.Request().Context()
	departmentID := c.Param("id")

	if departmentID == "" {
		return errors.ErrBadRequest.Errorf("部门ID不能为空")
	}

	var in types.MoveDepartmentInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 移动部门
	out, err := h.departmentService.MoveDepartment(ctx, departmentID, &in)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// SortDepartments 部门排序
func (h *DepartmentHandler) SortDepartments(c echo.Context) error {
	ctx := c.Request().Context()

	var in types.SortDepartmentInput
	if err := c.Bind(&in); err != nil {
		return errors.ErrBadRequest.Wrapf(err, "请求参数格式错误")
	}

	// 验证输入
	if errorDetails := in.Validate(); len(errorDetails) > 0 {
		return ValidationError(c, errorDetails)
	}

	// 执行排序
	err := h.departmentService.SortDepartments(ctx, &in)
	if err != nil {
		return err
	}

	return Success(c, nil)
}

// CheckDepartmentDeletable 检查部门是否可删除
func (h *DepartmentHandler) CheckDepartmentDeletable(c echo.Context) error {
	ctx := c.Request().Context()
	departmentID := c.Param("id")

	if departmentID == "" {
		return errors.ErrBadRequest.Errorf("部门ID不能为空")
	}

	// 检查部门是否可删除
	out, err := h.departmentService.CheckDepartmentDeletable(ctx, departmentID)
	if err != nil {
		return err
	}

	return Success(c, out)
}

// GetDepartmentStats 获取部门统计信息
func (h *DepartmentHandler) GetDepartmentStats(c echo.Context) error {
	ctx := c.Request().Context()

	// 获取部门统计信息
	out, err := h.departmentService.GetDepartmentStats(ctx)
	if err != nil {
		return err
	}

	return Success(c, out)
}
