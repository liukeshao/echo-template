package static

import "embed"

// FS 包含编译时嵌入的所有静态资源（HTML、JS、CSS）。
//
//go:embed index.html js css openapi
var FS embed.FS
