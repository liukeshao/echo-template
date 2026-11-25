package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
)

// Server 封装标准库 *http.Server 并实现优雅停机，供 DI 容器在关闭 ORM/DB 前排空在途请求。
type Server struct {
	srv *http.Server
	cfg config.HTTPConfig

	mu      sync.Mutex
	started bool
	closed  bool
	stopped chan struct{}
}

// NewServer 使用给定 HTTP 配置与根 http.Handler 构造 Server。
func NewServer(cfg config.HTTPConfig, handler http.Handler) *Server {
	return &Server{
		cfg: cfg,
		srv: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port),
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		stopped: make(chan struct{}),
	}
}

// New 构建路由并返回已接入 DI 生命周期（优雅停机）的 *Server。
func New(i do.Injector) (*Server, error) {
	cfg := do.MustInvoke[*config.Config](i)
	h, err := BuildRouter(i)
	if err != nil {
		return nil, err
	}
	return NewServer(cfg.HTTP, h), nil
}

// Start 阻塞式启动 HTTP 服务，直到 Shutdown 触发优雅停机或监听出错。
func (s *Server) Start() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	defer close(s.stopped)

	err := s.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown 实现 do.ShutdownerWithContextAndError：优雅排空在途 HTTP 请求并等待 Start 返回。
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	started := s.started
	s.mu.Unlock()

	if !started {
		return nil
	}

	shutdownCtx := ctx
	if s.cfg.GracefulTimeout > 0 {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(ctx, s.cfg.GracefulTimeout)
		defer cancel()
	}

	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
