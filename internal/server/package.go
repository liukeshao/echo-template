package server

import (
	"net"
	"strconv"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/httpkit"
)

var Package = do.Package(
	do.Lazy(newServer),
)

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
