package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/static"
)

// Docs 提供 API 文档 UI（Stoplight Elements）及相关静态资源（纯标准库 net/http）。
type Docs struct {
	staticServer http.Handler
}

// NewDocs 构造 Docs 并初始化静态文件服务器。
func NewDocs(_ do.Injector) (*Docs, error) {
	return &Docs{
		// /static/* 请求去掉 /static 前缀后仅暴露 js/ 与 css/ 目录
		// 例如 /static/js/web-components.min.js → js/web-components.min.js
		staticServer: http.StripPrefix("/static", http.FileServerFS(static.AssetsFS)),
	}, nil
}

// Handler 返回可挂载到 rest.Router 的文档端点处理器。
func (h *Docs) Handler(encodeErr rest.ErrorEncoder) rest.Handler {
	if encodeErr == nil {
		encodeErr = rest.DefaultErrorEncoder
	}
	return rest.NewHandler(func(mux *http.ServeMux) {
		mux.HandleFunc("GET /docs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(static.IndexHTML)
		})
		mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(static.OpenAPIYAML)
		})
		mux.HandleFunc("GET /static/", func(w http.ResponseWriter, r *http.Request) {
			name := strings.TrimPrefix(path.Clean(r.URL.Path), "/static/")
			info, err := fs.Stat(static.AssetsFS, name)
			if err != nil || info.IsDir() {
				encodeErr(w, r, rest.Problem{
					Status: http.StatusNotFound,
					Code:   rest.CodeNotFound,
					Metadata: rest.ResourceInfo{
						ResourceType: "static_asset",
						ResourceName: r.URL.Path,
					},
				})
				return
			}
			h.staticServer.ServeHTTP(w, r)
		})
	})
}
