package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
)

// Server wraps *echo.Echo and integrates it into the do lifecycle.
// Implements do.ShutdownerWithContextAndError so the DI container can
// trigger graceful HTTP drain before closing ORM and DB.
type Server struct {
	echo    *echo.Echo
	cfg     *config.Config
	ctx     context.Context
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

	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		echo:    web,
		cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
		stopped: make(chan struct{}),
	}, nil
}

// Start begins serving HTTP requests. It blocks until the server stops.
// Call Shutdown to initiate graceful drain.
func (s *Server) Start() error {
	defer close(s.stopped)

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

	return sc.Start(s.ctx, s.echo)
}

// Shutdown implements do.ShutdownerWithContextAndError.
// It cancels the server context (triggering echo's graceful drain) and waits
// for Start to return before releasing the DI container lock.
func (s *Server) Shutdown(_ context.Context) error {
	s.cancel()
	<-s.stopped
	return nil
}
