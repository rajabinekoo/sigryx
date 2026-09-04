package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	httpadapter "github.com/rajabinekoo/sigryx/internal/adapter/in/http"
	alertadapter "github.com/rajabinekoo/sigryx/internal/adapter/out/alert"
	"github.com/rajabinekoo/sigryx/internal/adapter/out/blockchain/ethereum"
	postgresadapter "github.com/rajabinekoo/sigryx/internal/adapter/out/persistence/postgres"
	"github.com/rajabinekoo/sigryx/internal/config"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
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
	accessRepository := postgresadapter.NewAccessRepository(pool)
	auditRepository := postgresadapter.NewAuditRepository(pool)
	signingRecordRepository := postgresadapter.NewSigningRecordRepository(pool)

	// ---- runtime secret ownership ----
	secrets := secretstore.New()
	defer secrets.Clear()

	// ---- core services ----
	authService, err := service.NewAuthService(accessRepository, service.AuthConfig{
		SetupToken: cfg.SetupToken,
		JWTSecret:  []byte(cfg.AuthJWTSecret),
		Issuer:     cfg.AuthIssuer,
		Audience:   cfg.AuthAudience,
		AccessTTL:  cfg.AuthAccessTTL,
		RefreshTTL: cfg.AuthRefreshTTL,
	})
	if err != nil {
		return fmt.Errorf("initialize auth service: %w", err)
	}

	accessService := service.NewAccessService(accessRepository)
	auditService := service.NewAuditService(auditRepository)
	auditRetentionService, err := service.NewAuditRetentionService(
		auditRepository,
		service.AuditRetentionConfig{
			NormalRetentionDays:   cfg.AuditNormalRetentionDays,
			CriticalRetentionDays: cfg.AuditCriticalRetentionDays,
			CleanupInterval:       cfg.AuditCleanupInterval,
			BatchSize:             cfg.AuditCleanupBatchSize,
		},
	)
	if err != nil {
		return fmt.Errorf("initialize audit retention service: %w", err)
	}

	sealService := service.NewSealService(
		unsealKeySlotRepository,
		secrets,
		cfg.MaxUnsealSize,
	)

	keyRootService := service.NewKeyRootService(
		keyRootRepository,
		secrets,
	)

	ethereumAdapter := ethereum.New()

	walletService := service.NewWalletService(
		walletRepository,
		keyRootRepository,
		secrets,
		ethereumAdapter,
	)

	alertSink, err := alertadapter.NewWebhook(cfg.AlertWebhookURL, cfg.AlertWebhookTimeout)
	if err != nil {
		return fmt.Errorf("initialize alert webhook: %w", err)
	}

	signingService := service.NewSigningService(
		walletRepository,
		keyRootRepository,
		secrets,
		ethereumAdapter,
		service.IntegrityDependencies{
			Records: signingRecordRepository,
			Audit:   auditRepository,
			Alerts:  alertSink,
		},
	)

	recoveryService := service.NewRecoveryService(
		keyRootRepository,
		secrets,
	)

	// ---- inbound HTTP adapter ----
	httpHandler := httpadapter.New(httpadapter.Deps{
		Auth:              authService,
		Access:            accessService,
		Seal:              sealService,
		KeyRoots:          keyRootService,
		Wallets:           walletService,
		Signing:           signingService,
		Recovery:          recoveryService,
		Audit:             auditService,
		TrustedProxyCIDRs: splitCSV(cfg.TrustedProxyCIDRs),
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

	if auditRetentionService.Enabled() {
		g.Go(func() error {
			log.Info(
				"starting audit retention worker",
				slog.Int("normal_retention_days", cfg.AuditNormalRetentionDays),
				slog.Int("critical_retention_days", cfg.AuditCriticalRetentionDays),
				slog.Duration("cleanup_interval", auditRetentionService.Interval()),
				slog.Int("batch_size", cfg.AuditCleanupBatchSize),
			)

			runCleanup := func() {
				result, cleanupErr := auditRetentionService.Cleanup(gctx)
				if cleanupErr != nil {
					if gctx.Err() == nil {
						log.Error("audit retention cleanup failed", slog.Any("err", cleanupErr))
					}
					return
				}
				if result.TotalDeleted() > 0 {
					log.Info(
						"audit retention cleanup completed",
						slog.Int("normal_deleted", result.NormalDeleted),
						slog.Int("critical_deleted", result.CriticalDeleted),
					)
					if auditErr := auditService.Record(context.WithoutCancel(gctx), domain.AuditEvent{
						ActorType:      "SYSTEM",
						Action:         "audit.retention_cleanup",
						Outcome:        domain.AuditOutcomeSuccess,
						RetentionClass: domain.AuditRetentionCritical,
						Details: map[string]any{
							"normal_deleted":          result.NormalDeleted,
							"critical_deleted":        result.CriticalDeleted,
							"normal_retention_days":   cfg.AuditNormalRetentionDays,
							"critical_retention_days": cfg.AuditCriticalRetentionDays,
						},
					}); auditErr != nil {
						log.Error("append audit retention event", slog.Any("err", auditErr))
					}
				}
			}

			runCleanup()
			ticker := time.NewTicker(auditRetentionService.Interval())
			defer ticker.Stop()
			for {
				select {
				case <-gctx.Done():
					return nil
				case <-ticker.C:
					runCleanup()
				}
			}
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	log.Info("all services stopped gracefully")
	return nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
