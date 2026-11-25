package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/liukeshao/echo-template/config"
)

// Server 封装标准库 *http.Server 并实现优雅停机，供 DI 容器在关闭 ORM/DB 前排空在途请求。
type Server struct {
	srv *http.Server
	cfg *config.Config

	mu      sync.Mutex
	started bool
	closed  bool
	stopped chan struct{}
}

// NewServer 使用给定配置与根 http.Handler 构造 Server。
func NewServer(cfg *config.Config, handler http.Handler) *Server {
	return &Server{
		cfg: cfg,
		srv: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", cfg.HTTP.Hostname, cfg.HTTP.Port),
			Handler:      handler,
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
			IdleTimeout:  cfg.HTTP.IdleTimeout,
		},
		stopped: make(chan struct{}),
	}
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
	if s.cfg.HTTP.GracefulTimeout > 0 {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(ctx, s.cfg.HTTP.GracefulTimeout)
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
