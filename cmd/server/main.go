//go:generate make -C ../.. api
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/samber/do/v2"

	"github.com/liukeshao/go-rest-template/config"
	"github.com/liukeshao/go-rest-template/internal/auth"
	"github.com/liukeshao/go-rest-template/internal/client"
	"github.com/liukeshao/go-rest-template/internal/database"
	"github.com/liukeshao/go-rest-template/internal/handler"
	"github.com/liukeshao/go-rest-template/internal/i18n"
	"github.com/liukeshao/go-rest-template/internal/server"
	"github.com/liukeshao/go-rest-template/internal/service"
	"github.com/liukeshao/go-rest-template/pkg/httpkit"
	"github.com/liukeshao/go-rest-template/pkg/logging"
)

func main() {
	configPath := flag.String("config", "", "path to TOML config file (optional)")
	flag.Parse()

	var (
		cfg config.Config
		err error
	)
	if strings.TrimSpace(*configPath) != "" {
		cfg, err = config.LoadFile(*configPath)
	} else {
		cfg, err = config.Load()
	}
	fatal("load config", err)

	logLevel := slog.LevelDebug
	logFormat := logging.FormatJSON
	if cfg.App.Environment == config.EnvProduction {
		logLevel = slog.LevelInfo
	}
	if cfg.App.Environment == config.EnvLocal {
		logFormat = logging.FormatText
	}
	logging.Setup(logLevel, logFormat)

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

	shutdownTimeout := max(cfg.HTTP.GracefulTimeout+5*time.Second, 10*time.Second)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	report := i.ShutdownWithContext(shutdownCtx)
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
