package handlers

import (
	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"
)

// Handler handles one or more HTTP routes.
type Handler interface {
	Routes(g *echo.Group)
}

// RouteRegistry 收集通过 provideHandler 注册进当前 DI 容器的业务 Handler，
// 供 BuildRouter 一次性挂载，避免在 router.go 中逐个手动 do.MustInvoke。
type RouteRegistry struct {
	mounts []func(i do.Injector, g *echo.Group)
}

// MountAll 解析并挂载所有已登记的 Handler 路由。
func (r *RouteRegistry) MountAll(i do.Injector, g *echo.Group) {
	for _, mount := range r.mounts {
		mount(i, g)
	}
}

// provideHandler 将 Handler 的构造函数以 Lazy 方式注册到 DI 容器，
// 同时登记到当前容器的 RouteRegistry（编译期约束实现 Handler 接口）。
func provideHandler[T Handler](provider do.Provider[T]) func(do.Injector) {
	return func(i do.Injector) {
		do.Lazy(provider)(i)
		reg := do.MustInvoke[*RouteRegistry](i)
		reg.mounts = append(reg.mounts, func(inj do.Injector, g *echo.Group) {
			do.MustInvoke[T](inj).Routes(g)
		})
	}
}
