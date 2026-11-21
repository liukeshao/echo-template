package handlers

import (
	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"
)

// Handler 定义业务路由注册接口。
// g 为 API 版本前缀组（如 /api/v1），auth 为认证中间件，按需挂在受保护的分组或单条路由上。
type Handler interface {
	Routes(g *echo.Group, auth echo.MiddlewareFunc)
}

// routeRegistry 收集通过 provideHandler 注册进当前 DI 容器的业务 Handler，
// 供 BuildRouter 一次性挂载，避免在 router.go 中逐个手动 do.MustInvoke。
type routeRegistry struct {
	mounts []func(i do.Injector, g *echo.Group, auth echo.MiddlewareFunc)
}

func getOrCreateRouteRegistry(i do.Injector) *routeRegistry {
	if reg, err := do.Invoke[*routeRegistry](i); err == nil {
		return reg
	}
	reg := &routeRegistry{}
	do.ProvideValue(i, reg)
	return reg
}

func (r *routeRegistry) mountAll(i do.Injector, g *echo.Group, auth echo.MiddlewareFunc) {
	for _, mount := range r.mounts {
		mount(i, g, auth)
	}
}

// provideHandler 将 Handler 的构造函数以 Lazy 方式注册到 DI 容器，
// 同时登记到当前容器的 routeRegistry（编译期约束实现 Handler 接口，无需关心注册顺序）。
func provideHandler[T Handler](provider do.Provider[T]) func(do.Injector) {
	return func(i do.Injector) {
		do.Lazy(provider)(i)
		reg := getOrCreateRouteRegistry(i)
		reg.mounts = append(reg.mounts, func(inj do.Injector, g *echo.Group, auth echo.MiddlewareFunc) {
			do.MustInvoke[T](inj).Routes(g, auth)
		})
	}
}
