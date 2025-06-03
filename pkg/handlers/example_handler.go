package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/services"
)

// ExampleHandler 示例处理器，演示如何使用Response和错误处理
type ExampleHandler struct {
	// 这里可以注入需要的服务
}

// Init 初始化handler
func (h *ExampleHandler) Init(c *services.Container) error {
	// 在这里可以初始化依赖的服务
	return nil
}

func init() {
	Register(new(ExampleHandler))
}

// Routes 注册路由
func (h *ExampleHandler) Routes(g *echo.Group) {
	// 示例路由组
	api := g.Group("/api/v1")

	// 成功响应示例
	api.GET("/success", h.handleSuccess)

	// 业务错误响应示例
	api.GET("/business-error", h.handleBusinessError)

	// 验证错误响应示例
	api.POST("/validation-error", h.handleValidationError)

	// HTTP错误响应示例
	api.GET("/http-error", h.handleHTTPError)

	// 未知错误响应示例
	api.GET("/unknown-error", h.handleUnknownError)
}

// handleSuccess 处理成功响应
func (h *ExampleHandler) handleSuccess(c echo.Context) error {
	// 模拟数据
	data := map[string]interface{}{
		"id":    1,
		"name":  "测试用户",
		"email": "test@example.com",
	}

	// 使用Response构建器返回成功响应（request_id会自动从context获取）
	return Success(data).JSON(c)
}

// handleBusinessError 处理业务错误
func (h *ExampleHandler) handleBusinessError(c echo.Context) error {
	// 创建业务错误
	err := errors.NotFoundError("用户不存在").
		With("user_id", "123", "attempted_at", "2024-01-01T12:00:00Z")

	// 返回错误，由错误处理器统一处理
	return err
}

// handleValidationError 处理验证错误
func (h *ExampleHandler) handleValidationError(c echo.Context) error {
	// 模拟验证错误
	validationErrors := []ErrorDetail{
		{
			Field:   "email",
			Message: "邮箱格式不正确",
			Code:    "INVALID_EMAIL",
		},
		{
			Field:   "password",
			Message: "密码长度不能少于8位",
			Code:    "PASSWORD_TOO_SHORT",
		},
	}

	// 使用ValidationError构建器返回验证错误（request_id会自动从context获取）
	return ValidationError("输入数据验证失败", validationErrors).JSON(c)
}

// handleHTTPError 处理HTTP错误
func (h *ExampleHandler) handleHTTPError(c echo.Context) error {
	// 返回Echo HTTP错误，由错误处理器统一处理
	return echo.NewHTTPError(400, "请求参数无效")
}

// handleUnknownError 处理未知错误
func (h *ExampleHandler) handleUnknownError(c echo.Context) error {
	// 模拟一个未知错误
	panic("这是一个未知错误")
}
