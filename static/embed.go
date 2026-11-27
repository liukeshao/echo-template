package static

import "embed"

// AssetsFS 仅暴露 js/css，隔离 index.html 与 openapi.yaml。
//
//go:embed js css
var AssetsFS embed.FS

//go:embed index.html openapi/openapi.yaml
var DocsFS embed.FS
