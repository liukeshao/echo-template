package handlers

import (
	"net/http"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/httpruntime"
)

// Handler 定义业务路由挂载接口。
// 每个业务 Handler 实现对应的 TypeSpec 生成接口（api.XxxHandler），并在 Mount 中调用 api.MountXxxHandler。
type Handler interface {
	Mount(mux *http.ServeMux, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler)
}

// routeRegistry 收集通过 provideHandler 注册进当前 DI 容器的业务 Handler，
// 供 BuildRouter 一次性挂载，避免在 router.go 中逐个手动 do.MustInvoke。
type routeRegistry struct {
	mounts []func(i do.Injector, mux *http.ServeMux, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler)
}

func getOrCreateRouteRegistry(i do.Injector) *routeRegistry {
	if reg, err := do.Invoke[*routeRegistry](i); err == nil {
		return reg
	}
	reg := &routeRegistry{}
	do.ProvideValue(i, reg)
	return reg
}

func (r *routeRegistry) mountAll(i do.Injector, mux *http.ServeMux, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler) {
	for _, mount := range r.mounts {
		mount(i, mux, rt, auth)
	}
}

// provideHandler 将 Handler 的构造函数以 Lazy 方式注册到 DI 容器，
// 同时登记到当前容器的 routeRegistry（编译期约束实现 Handler 接口，无需关心注册顺序）。
func provideHandler[T Handler](provider do.Provider[T]) func(do.Injector) {
	return func(i do.Injector) {
		do.Lazy(provider)(i)
		reg := getOrCreateRouteRegistry(i)
		reg.mounts = append(reg.mounts, func(inj do.Injector, mux *http.ServeMux, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler) {
			do.MustInvoke[T](inj).Mount(mux, rt, auth)
		})
	}
}

// mapSlice 将源切片逐元素映射为目标切片，保证返回值永不为 nil（空切片 JSON 序列化为 []）。
func mapSlice[In, Out any](in []In, fn func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}
