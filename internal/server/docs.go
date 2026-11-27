package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/httpkit"
	"github.com/liukeshao/echo-template/static"
)

// DocsHandler 返回可挂载到 httpkit.Router 的 API 文档与静态资源处理器（Stoplight Elements）。
func DocsHandler(encodeErr httperr.Encoder) httpkit.Handler {
	if encodeErr == nil {
		encodeErr = httperr.DefaultEncoder
	}
	// /static/* 请求去掉 /static 前缀后仅暴露 js/ 与 css/ 目录
	// 例如 /static/js/web-components.min.js → js/web-components.min.js
	staticServer := http.StripPrefix("/static", http.FileServerFS(static.AssetsFS))

	return httpkit.NewHandler(func(mux *http.ServeMux) {
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
				encodeErr(w, r, httperr.Problem{
					Status: http.StatusNotFound,
					Metadata: httperr.ResourceInfo{
						ResourceType: "static_asset",
						ResourceName: r.URL.Path,
					},
				})
				return
			}
			staticServer.ServeHTTP(w, r)
		})
	})
}
