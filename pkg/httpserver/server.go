// Package httpserver wraps http.Server with consistent timeouts and graceful
// shutdown. The router is supplied by the caller (http.Handler) so this package
// stays framework-agnostic — see [[project-open-decisions]] in memory: the
// router/framework for HTTP-facing services has not been selected yet.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Config struct {
	Addr              string        `env:"HTTP_ADDR,required"`
	ReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"5s"`
	WriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"15s"`
	IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	ShutdownTimeout   time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

type Server struct {
	cfg    Config
	logger *slog.Logger
	srv    *http.Server
}

// New builds a Server bound to cfg.Addr with handler as the root mux.
func New(cfg Config, logger *slog.Logger, handler http.Handler) *Server {
	return &Server{
		cfg:    cfg,
		logger: logger,
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

// Serve blocks until ctx is cancelled, then drives a graceful shutdown.
func (s *Server) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", slog.String("addr", s.cfg.Addr))
		err := s.srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("http server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("httpserver: shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("httpserver: serve: %w", err)
		}
		return nil
	}
}
