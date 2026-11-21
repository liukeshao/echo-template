package static

import "embed"

// AssetsFS 仅包含对外暴露的静态前端资源（JS / CSS），用于 /static/* 路由，
// 避免将 index.html 或 openapi.yaml 暴露在 /static/ 路径下。
//
//go:embed js css
var AssetsFS embed.FS

// IndexHTML 为 Stoplight Elements 文档入口页面内容。
//
//go:embed index.html
var IndexHTML []byte

// OpenAPIYAML 为由 `make swag` 生成的 OpenAPI 规范内容。
//
//go:embed openapi/openapi.yaml
var OpenAPIYAML []byte
