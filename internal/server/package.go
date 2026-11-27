package server

import (
	"net"
	"strconv"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/httpkit"
)

// Package 注册 HTTP Server 到 DI 容器。
var Package = do.Package(
	do.Lazy(newServer),
)

// newServer 供 DI 容器构造已绑定路由与优雅停机生命周期的 *httpkit.Server。
func newServer(i do.Injector) (*httpkit.Server, error) {
	cfg := do.MustInvoke[*config.Config](i)
	h, err := BuildRouter(i)
	if err != nil {
		return nil, err
	}
	return httpkit.NewServer(httpkit.ServerConfig{
		Addr:            net.JoinHostPort(cfg.HTTP.Hostname, strconv.Itoa(int(cfg.HTTP.Port))),
		ReadTimeout:     cfg.HTTP.ReadTimeout,
		WriteTimeout:    cfg.HTTP.WriteTimeout,
		IdleTimeout:     cfg.HTTP.IdleTimeout,
		GracefulTimeout: cfg.HTTP.GracefulTimeout,
	}, h), nil
}
