package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	httpadapter "github.com/rajabinekoo/sigryx/internal/adapter/in/http"
	"github.com/rajabinekoo/sigryx/internal/config"
	pkgent "github.com/rajabinekoo/sigryx/internal/ent"
	"github.com/rajabinekoo/sigryx/internal/secretstore"
	"github.com/rajabinekoo/sigryx/internal/securemem"
	"github.com/rajabinekoo/sigryx/pkg/entpg"
	pkghttp "github.com/rajabinekoo/sigryx/pkg/httpserver"
	"github.com/rajabinekoo/sigryx/pkg/logger"
	pkgpostgres "github.com/rajabinekoo/sigryx/pkg/postgres"
	"golang.org/x/sync/errgroup"
)

func main() {
	if err := run(); err != nil {
		slog.New(
			slog.NewJSONHandler(
				os.Stderr,
				nil,
			),
		).Error(
			"server exited with error",
			slog.Any(
				"err",
				err,
			),
		)
		os.Exit(1)
	}
}

func run() error {
	if err := securemem.Initialize(); err != nil {
		return fmt.Errorf(
			"initialize secure memory: %w",
			err,
		)
	}

	secretStore, err := secretstore.New(3)
	if err != nil {
		return err
	}
	defer secretStore.Clear()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(logger.Config{
		ServiceName: cfg.ServiceName,
		Level:       cfg.LogLevel,
		Format:      cfg.LogFormat,
	})

	log.Info(
		"sigryx is starting",
		slog.String("http_addr", cfg.HTTPAddr),
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	// ---- outbound persistance adapters ----
	pool, err := pkgpostgres.New(ctx, pkgpostgres.Config{
		DSN:             cfg.PostgresDSN,
		MaxConns:        cfg.PostgresMaxConns,
		MinConns:        cfg.PostgresMinConns,
		MaxConnLifetime: cfg.PostgresMaxConnLifetime,
		MaxConnIdleTime: cfg.PostgresMaxConnIdleTime,
		ConnectTimeout:  cfg.PostgresConnectTimeout,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	entDriver, sqlDB := entpg.OpenDriver(pool)
	entClient := pkgent.NewClient(pkgent.Driver(entDriver))
	defer entClient.Close()
	defer entDriver.Close()
	defer sqlDB.Close()

	// ---- inbound HTTP adapter ----
	httpHandler := httpadapter.New(httpadapter.Deps{})
	httpSrv := pkghttp.New(pkghttp.Config{
		Addr:            cfg.HTTPAddr,
		ReadTimeout:     cfg.HTTPReadTimeout,
		WriteTimeout:    cfg.HTTPWriteTimeout,
		IdleTimeout:     cfg.HTTPIdleTimeout,
		ShutdownTimeout: cfg.HTTPShutdownTimeout,
	}, log, httpHandler)

	// ---- start services and jobs ----
	g, gctx := errgroup.WithContext(ctx)

	// Start HTTP server
	g.Go(func() error {
		log.Info("starting HTTP server", slog.String("addr", cfg.HTTPAddr))
		if err := httpSrv.Serve(gctx); err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	log.Info("all services stopped gracefully")

	return nil
}
