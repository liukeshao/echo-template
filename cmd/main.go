// Package main is the entry point of the application.
//
//	@title						Echo Template API
//	@version					1.0
//	@description				Echo Template RESTful API
//	@host						localhost:8000
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Bearer {token}
//
//go:generate make -C .. swag
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/clients"
	"github.com/liukeshao/echo-template/pkg/handlers"
	"github.com/liukeshao/echo-template/pkg/i18n"
	"github.com/liukeshao/echo-template/pkg/infra"
	"github.com/liukeshao/echo-template/pkg/log"
	"github.com/liukeshao/echo-template/pkg/services"
)

func main() {
	// 1. Config (bootstrap — needed before DI starts)
	cfg, err := config.GetConfig()
	fatal("load config", err)

	// 2. Log
	log.Setup(&cfg)

	// 3. DI container
	i := do.New(infra.Package, i18n.Package, clients.Package, services.Package, handlers.Package)
	do.ProvideValue(i, &cfg)

	// 4. Start HTTP server
	server := do.MustInvoke[*handlers.Server](i)
	slog.Info("API docs", "url", fmt.Sprintf("%s/docs", cfg.App.Host))

	go func() {
		if err := server.Start(); err != nil {
			slog.Error("server stopped", "error", err)
		}
	}()

	// 5. Block until SIGINT/SIGTERM, then shut down all DI services in order:
	//    Server (drain HTTP) → ORM → DB
	sig, report := i.ShutdownOnSignalsWithContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	slog.Info("shutdown complete", "signal", sig)

	if !report.Succeed {
		slog.Error("shutdown errors", "errors", report.Errors)
		os.Exit(1)
	}
}

func fatal(msg string, err error) {
	if err != nil {
		slog.Error(msg, "error", err)
		os.Exit(1)
	}
}
