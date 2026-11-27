package httpkit

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

type ServerConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	GracefulTimeout time.Duration
}

// Server 封装 *http.Server 并提供竞态安全的优雅停机。
type Server struct {
	srv             *http.Server
	gracefulTimeout time.Duration

	mu      sync.Mutex
	started bool
	closed  bool
	stopped chan struct{}
}

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
