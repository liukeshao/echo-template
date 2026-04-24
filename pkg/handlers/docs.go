package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/static"
)

// DocsHandler 提供 API 文档 UI 及相关静态资源。
type DocsHandler struct {
	staticServer http.Handler
}

// NewDocsHandler 构造 DocsHandler 并初始化静态文件服务器。
func NewDocsHandler(_ do.Injector) (*DocsHandler, error) {
	return &DocsHandler{
		// /static/* 请求去掉 /static 前缀后直接对应 embed.FS 中的路径
		// 例如 /static/js/web-components.min.js → js/web-components.min.js
		staticServer: http.StripPrefix("/static", http.FileServerFS(static.FS)),
	}, nil
}

// Routes 注册文档相关路由。
func (h *DocsHandler) Routes(g *echo.Group) {
	g.GET("/docs", h.index)
	g.GET("/openapi.yaml", h.openAPI)
	g.GET("/static/*", h.serveStatic)
}

// index 返回 API 文档页面（Stoplight Elements）。
func (h *DocsHandler) index(c *echo.Context) error {
	content, err := static.FS.ReadFile("index.html")
	if err != nil {
		return err
	}
	return c.HTMLBlob(http.StatusOK, content)
}

// openAPI 返回 OpenAPI YAML 规范文件。
func (h *DocsHandler) openAPI(c *echo.Context) error {
	content, err := static.FS.ReadFile("openapi/openapi.yaml")
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "application/yaml", content)
}

// serveStatic 提供嵌入的 JS / CSS 静态资源。
func (h *DocsHandler) serveStatic(c *echo.Context) error {
	h.staticServer.ServeHTTP(c.Response(), c.Request())
	return nil
}
