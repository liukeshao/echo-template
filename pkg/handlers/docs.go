package handlers

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/static"
)

// DocsHandler 提供 API 文档 UI（Stoplight Elements）及相关静态资源（纯标准库 net/http）。
type DocsHandler struct {
	staticServer http.Handler
}

// NewDocsHandler 构造 DocsHandler 并初始化静态文件服务器。
func NewDocsHandler(_ do.Injector) (*DocsHandler, error) {
	return &DocsHandler{
		// /static/* 请求去掉 /static 前缀后仅暴露 js/ 与 css/ 目录
		// 例如 /static/js/web-components.min.js → js/web-components.min.js
		staticServer: http.StripPrefix("/static", http.FileServerFS(static.AssetsFS)),
	}, nil
}

// Mount 注册文档相关路由。
func (h *DocsHandler) Mount(mux *http.ServeMux, rt *httpruntime.Runtime) {
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
			rt.WriteProblem(w, r, httpruntime.Problem{
				Status: http.StatusNotFound,
				Code:   "NOT_FOUND",
				Detail: http.StatusText(http.StatusNotFound),
			})
			return
		}
		h.staticServer.ServeHTTP(w, r)
	})
}
