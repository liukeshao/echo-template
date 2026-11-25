package rest

import (
	"net/http"
	"sync"
)

// Handler 表示一组由 TypeSpec 生成（或手写）的 REST 端点（Connect-Go 风格）：
//   - 本身直接实现标准 http.Handler，可独立运行于 httptest 或嵌入任意 HTTP 服务；
//   - 同时实现 Mount(*http.ServeMux)，可由 Router 展平挂载到共享 ServeMux，
//     完整保留 Go 1.22+ 精确路由匹配、冲突检测与 404/405 RFC 9457 语义。
type Handler interface {
	http.Handler
	Mount(mux *http.ServeMux)
}

type mountableHandler struct {
	register func(mux *http.ServeMux)

	once sync.Once
	mux  *http.ServeMux
}

// NewHandler 将路由注册函数包装为同时支持独立 ServeHTTP 与 Mount 的 rest.Handler。
func NewHandler(register func(mux *http.ServeMux)) Handler {
	return &mountableHandler{register: register}
}

func (h *mountableHandler) Mount(mux *http.ServeMux) {
	h.register(mux)
}

func (h *mountableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.once.Do(func() {
		h.mux = http.NewServeMux()
		h.register(h.mux)
	})
	h.mux.ServeHTTP(w, r)
}
