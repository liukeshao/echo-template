package handlers

import (
	"net/http"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

// BuildRouter 构建标准库 *http.ServeMux，挂载所有已注册的业务 Handler、文档路由与全局中间件。
func BuildRouter(i do.Injector) (http.Handler, error) {
	cfg := do.MustInvoke[*config.Config](i)
	rt := do.MustInvoke[*httpruntime.Runtime](i)
	authSvc := do.MustInvoke[*services.AuthService](i)

	mux := http.NewServeMux()
	auth := middleware.RequireAuth(authSvc, rt)

	getOrCreateRouteRegistry(i).mountAll(i, mux, rt, auth)

	// 生产环境不暴露 API 文档与静态资源。
	if cfg.App.Environment != config.EnvProduction {
		do.MustInvoke[*DocsHandler](i).Mount(mux, rt)
	}

	return rt.WrapMux(mux, defaultMaxBodyBytes), nil
}
