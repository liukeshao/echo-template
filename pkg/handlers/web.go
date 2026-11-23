package handlers

import (
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
)

// Server 是 httpruntime.Server 的别名，保留原有 cmd/main.go 注入类型。
type Server = httpruntime.Server

// NewServer 构建路由并返回已接入 DI 生命周期（优雅停机）的 HTTP Server。
func NewServer(i do.Injector) (*Server, error) {
	cfg := do.MustInvoke[*config.Config](i)
	handler, err := BuildRouter(i)
	if err != nil {
		return nil, err
	}
	return httpruntime.NewServer(cfg, handler), nil
}
