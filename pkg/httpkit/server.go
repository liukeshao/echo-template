package httpkit

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// ServerConfig 配置 httpkit.Server 的底层 *http.Server 参数与优雅停机超时。
type ServerConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	GracefulTimeout time.Duration
}

// Server 封装标准库 *http.Server 并实现竞态安全的优雅停机（含启动前 Shutdown 不死锁），
// 隐式满足 samber/do 的 Shutdown(context.Context) error 契约，零三方依赖。
type Server struct {
	srv             *http.Server
	gracefulTimeout time.Duration

	mu      sync.Mutex
	started bool
	closed  bool
	stopped chan struct{}
}

// NewServer 使用给定配置与根 http.Handler 构造 Server。
func NewServer(cfg ServerConfig, handler http.Handler) *Server {
	return &Server{
		gracefulTimeout: cfg.GracefulTimeout,
		srv: &http.Server{
			Addr:         cfg.Addr,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
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

// Shutdown 优雅排空在途 HTTP 请求并等待 Start 返回。
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	started := s.started
	s.mu.Unlock()

	if !started {
		return nil
	}

	shutdownCtx := ctx
	if s.gracefulTimeout > 0 {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(ctx, s.gracefulTimeout)
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
