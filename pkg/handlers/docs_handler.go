package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/errors"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
)

func init() {
	Register(new(DocsHandler))
}

// DocsHandler 文档处理器
type DocsHandler struct {
	orm         *ent.Client
	docsService *services.DocsService
	config      *services.Container
}

// Init 初始化文档处理器
func (h *DocsHandler) Init(c *services.Container) error {
	h.orm = c.ORM
	h.docsService = c.Docs
	h.config = c
	return nil
}

// Routes 注册文档相关路由
func (h *DocsHandler) Routes(g *echo.Group) {
	// 只有在文档启用时才注册路由
	if !h.docsService.IsDocsEnabled() {
		return
	}

	// 创建文档中间件
	docsMw := middleware.NewDocsMiddleware(middleware.DocsConfig{
		AppConfig: h.config.Config,
	})

	// 文档路由组，应用中间件
	docs := g.Group("")
	docs.Use(docsMw.CheckAccess)
	docs.Use(docsMw.SecurityHeaders)

	// API规范路由
	docs.GET("/openapi.json", h.GetOpenAPISpec)

	// 文档页面路由
	docs.GET("/docs", h.GetDocsPage)
	docs.GET("/docs/*", h.GetDocsPage) // 处理子路径
}

// GetOpenAPISpec 获取OpenAPI规范
func (h *DocsHandler) GetOpenAPISpec(c echo.Context) error {
	ctx := c.Request().Context()

	slog.InfoContext(ctx, "生成OpenAPI规范")

	spec, err := h.docsService.GenerateOpenAPISpec(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "生成OpenAPI规范失败", "error", err)
		return errors.InternalError("生成API规范失败")
	}

	return c.JSON(http.StatusOK, spec)
}

// GetDocsPage 获取文档页面
func (h *DocsHandler) GetDocsPage(c echo.Context) error {
	ctx := c.Request().Context()

	slog.InfoContext(ctx, "提供API文档页面")

	// 获取当前环境信息
	env := h.config.Config.App.Environment
	envDisplay := "开发环境"
	switch env {
	case "prod":
		envDisplay = "生产环境"
	case "staging":
		envDisplay = "预发布环境"
	case "qa":
		envDisplay = "测试环境"
	case "test":
		envDisplay = "测试环境"
	default:
		envDisplay = "开发环境"
	}

	// 使用 Stoplight Elements 的 Web Components 版本
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>Echo Template API 文档</title>
    
    <!-- Stoplight Elements CSS -->
    <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements/styles.min.css">
    
    <style>
        body {
            margin: 0;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 1rem 2rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        
        .header h1 {
            margin: 0;
            font-size: 1.5rem;
            font-weight: 600;
        }
        
        .header p {
            margin: 0.5rem 0 0 0;
            opacity: 0.9;
            font-size: 0.9rem;
        }
        
        .docs-container {
            height: calc(100vh - 100px);
            overflow: hidden;
        }
        
        /* 环境标识 */
        .env-badge {
            position: fixed;
            top: 10px;
            right: 10px;
            background: rgba(255, 255, 255, 0.2);
            color: white;
            padding: 0.25rem 0.75rem;
            border-radius: 20px;
            font-size: 0.8rem;
            font-weight: 500;
            backdrop-filter: blur(10px);
            z-index: 1000;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Echo Template API 文档</h1>
        <p>基于 Echo 框架的高性能 Web 应用 API 接口文档</p>
    </div>
    
    <div class="env-badge">%s</div>
    
    <div class="docs-container">
        <elements-api
            apiDescriptionUrl="/openapi.json"
            router="hash"
            layout="sidebar"
            hideInternal="false"
            hideTryIt="false"
            tryItCredentialsPolicy="include"
        ></elements-api>
    </div>

    <!-- Stoplight Elements JS -->
    <script src="https://unpkg.com/@stoplight/elements/web-components.min.js"></script>
    
    <script>
        // 添加一些自定义交互
        document.addEventListener('DOMContentLoaded', function() {
            console.log('Echo Template API 文档加载完成');
            console.log('当前环境: %s');
            
            // 可以在这里添加自定义的 JavaScript 逻辑
            // 例如：统计、用户行为追踪等
        });
    </script>
</body>
</html>`, envDisplay, env)

	return c.HTML(http.StatusOK, html)
}
