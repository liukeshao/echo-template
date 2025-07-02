package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/services"
)

// OpenAPIHandler OpenAPI文档处理器
type OpenAPIHandler struct{}

// 自动注册
func init() {
	Register(new(OpenAPIHandler))
}

// Init 依赖注入
func (h *OpenAPIHandler) Init(c *services.Container) error {
	return nil
}

// Routes 路由定义
func (h *OpenAPIHandler) Routes(g *echo.Group) {
	// 重定向到静态文档页面
	g.GET("/docs", h.RedirectToDocs)
	g.GET("/", h.RedirectToDocs)
}

// RedirectToDocs 重定向到静态文档页面
func (h *OpenAPIHandler) RedirectToDocs(c echo.Context) error {
	return c.Redirect(http.StatusMovedPermanently, "/static/docs/index.html")
}
