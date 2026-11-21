package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
)

// Server wraps *echo.Echo and integrates it into the do lifecycle.
// Implements do.ShutdownerWithContextAndError so the DI container can
// trigger graceful HTTP drain before closing ORM and DB.
type Server struct {
	echo *echo.Echo
	cfg  *config.Config

	mu      sync.Mutex
	started bool
	closed  bool
	cancel  context.CancelFunc // cancels the StartConfig context → triggers graceful shutdown
	stopped chan struct{}      // closed when Start returns
}

// NewServer constructs a configured echo server and wires all routes.
func NewServer(i do.Injector) (*Server, error) {
	cfg := do.MustInvoke[*config.Config](i)

	web := echo.New()
	if err := BuildRouter(i, web); err != nil {
		return nil, err
	}

	return &Server{
		echo:    web,
		cfg:     cfg,
		stopped: make(chan struct{}),
	}, nil
}

// Start begins serving HTTP requests. It blocks until the server stops.
// Call Shutdown to initiate graceful drain.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.started = true
	s.mu.Unlock()

	defer func() {
		cancel()
		close(s.stopped)
	}()

	sc := echo.StartConfig{
		Address:         fmt.Sprintf("%s:%d", s.cfg.HTTP.Hostname, s.cfg.HTTP.Port),
		GracefulTimeout: s.cfg.HTTP.GracefulTimeout,
		HideBanner:      true,
		BeforeServeFunc: func(srv *http.Server) error {
			srv.ReadTimeout = s.cfg.HTTP.ReadTimeout
			srv.WriteTimeout = s.cfg.HTTP.WriteTimeout
			srv.IdleTimeout = s.cfg.HTTP.IdleTimeout
			return nil
		},
	}

	return sc.Start(ctx, s.echo)
}

// Shutdown implements do.ShutdownerWithContextAndError.
// It cancels the server context (triggering echo's graceful drain) and waits
// for Start to return before releasing the DI container lock.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	started := s.started
	cancel := s.cancel
	s.mu.Unlock()

	if !started {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
