package static

import "embed"

// AssetsFS 仅包含对外暴露的静态前端资源（JS / CSS），用于 /static/* 路由，
// 避免将 index.html 或 openapi.yaml 暴露在 /static/ 路径下。
//
//go:embed js css
var AssetsFS embed.FS

// DocsFS 包含 API 文档入口页与由 `make api` 生成的 OpenAPI 3.1 规范文件。
//
//go:embed index.html openapi/openapi.yaml
var DocsFS embed.FS
