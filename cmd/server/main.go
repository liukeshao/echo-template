//go:generate make -C ../.. api
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/internal/auth"
	"github.com/liukeshao/echo-template/internal/client"
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/internal/handler"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/internal/server"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/pkg/httpkit"
	"github.com/liukeshao/echo-template/pkg/logging"
)

func main() {
	cfg, err := config.Load()
	fatal("load config", err)

	logLevel := slog.LevelDebug
	if cfg.App.Environment == config.EnvProduction {
		logLevel = slog.LevelInfo
	}
	logging.Setup(logLevel)

	i := do.New(database.Package, i18n.Package, auth.Package, client.Package, service.Package, handler.Package, server.Package)
	do.ProvideValue(i, &cfg)

	srv := do.MustInvoke[*httpkit.Server](i)
	if cfg.App.Environment != config.EnvProduction {
		slog.Info("API docs", "url", fmt.Sprintf("%s/docs", strings.TrimRight(cfg.App.Host, "/")))
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	var startErr error
	select {
	case <-sigCtx.Done():
		slog.Info("shutting down on signal")
	case err := <-errCh:
		if err != nil {
			startErr = err
			slog.Error("server stopped with error", "error", err)
		}
	}

	report := i.ShutdownWithContext(context.Background())
	if !report.Succeed {
		slog.Error("shutdown errors", "errors", report.Errors)
		os.Exit(1)
	}
	if startErr != nil {
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}

func fatal(msg string, err error) {
	if err != nil {
		slog.Error(msg, "error", err)
		os.Exit(1)
	}
}
