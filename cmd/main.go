package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	httpadapter "github.com/rajabinekoo/sigryx/internal/adapter/in/http"
	"github.com/rajabinekoo/sigryx/internal/adapter/out/blockchain/ethereum"
	postgresadapter "github.com/rajabinekoo/sigryx/internal/adapter/out/persistence/postgres"
	"github.com/rajabinekoo/sigryx/internal/config"
	"github.com/rajabinekoo/sigryx/internal/core/service"
	pkgent "github.com/rajabinekoo/sigryx/internal/ent"
	"github.com/rajabinekoo/sigryx/pkg/entpg"
	pkghttp "github.com/rajabinekoo/sigryx/pkg/httpserver"
	"github.com/rajabinekoo/sigryx/pkg/logger"
	pkgpostgres "github.com/rajabinekoo/sigryx/pkg/postgres"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
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
			slog.Any("err", err),
		)
		os.Exit(1)
	}
}

func run() error {
	if err := securemem.Initialize(); err != nil {
		return fmt.Errorf("initialize secure memory: %w", err)
	}

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

	// ---- outbound persistence adapters ----
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

	unsealKeySlotRepository := postgresadapter.NewUnsealKeySlotRepository(entClient)
	keyRootRepository := postgresadapter.NewKeyRootRepository(entClient)
	walletRepository := postgresadapter.NewWalletRepository(pool)

	// ---- runtime secret ownership ----
	secrets := secretstore.New()
	defer secrets.Clear()

	// ---- core services ----
	sealService := service.NewSealService(
		unsealKeySlotRepository,
		secrets,
		cfg.MaxUnsealSize,
	)

	keyRootService := service.NewKeyRootService(
		keyRootRepository,
		secrets,
	)

	walletService := service.NewWalletService(
		walletRepository,
		keyRootRepository,
		secrets,
		ethereum.New(),
	)

	// ---- inbound HTTP adapter ----
	httpHandler := httpadapter.New(httpadapter.Deps{
		Seal:     sealService,
		KeyRoots: keyRootService,
		Wallets:  walletService,
	})

	httpSrv := pkghttp.New(pkghttp.Config{
		Addr:            cfg.HTTPAddr,
		ReadTimeout:     cfg.HTTPReadTimeout,
		WriteTimeout:    cfg.HTTPWriteTimeout,
		IdleTimeout:     cfg.HTTPIdleTimeout,
		ShutdownTimeout: cfg.HTTPShutdownTimeout,
	}, log, httpHandler)

	// ---- start services ----
	g, gctx := errgroup.WithContext(ctx)

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
