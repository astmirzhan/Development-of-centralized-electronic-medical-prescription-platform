package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xjam-byte/erx-platform/backend/internal/audit"
	"github.com/xjam-byte/erx-platform/backend/internal/auth"
	"github.com/xjam-byte/erx-platform/backend/internal/cds"
	"github.com/xjam-byte/erx-platform/backend/internal/config"
	"github.com/xjam-byte/erx-platform/backend/internal/database"
	"github.com/xjam-byte/erx-platform/backend/internal/httpserver"
	"github.com/xjam-byte/erx-platform/backend/internal/pki"
	"github.com/xjam-byte/erx-platform/backend/internal/prescriptions"
	"github.com/xjam-byte/erx-platform/backend/internal/qr"
)

type poolPinger struct {
	pool *pgxpool.Pool
}

func (p poolPinger) Ping(ctx context.Context) error {
	return database.HealthPing(ctx, p.pool)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(rootCtx, cfg.DatabaseURL())
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	auditLogger := audit.NewPGLogger()

	var pkiVerifier pki.Verifier
	if cfg.NCATestMode {
		mockVerifier, err := pki.NewMockVerifier(cfg.AppEnv, cfg.NCATestMode, cfg.JWTSigningKey)
		if err != nil {
			logger.Error("failed to initialize mock PKI verifier", "error", err)
			os.Exit(1)
		}
		pkiVerifier = mockVerifier
	} else {
		pkiVerifier = pki.NewNCANodeVerifier(cfg.NCANodeURL, nil)
	}

	cdsRepo := cds.NewPGRepository(pool)
	cdsService := cds.NewService(cdsRepo, logger)
	cdsHandler := cds.NewHandlerWithVerifier(cdsService, cdsRepo, logger)

	rxRepo := prescriptions.NewPGRepository(pool)
	sigRepo := pki.NewPGRepository()
	rxService := prescriptions.NewService(rxRepo, rxRepo, sigRepo, pkiVerifier, auditLogger, logger)
	rxHandler := prescriptions.NewHandler(rxService, logger)

	qrRepo := qr.NewPGRepository(pool)
	qrService := qr.NewService(qrRepo, cfg.QRTokenRotation, cfg.QRTokenAcceptWindow)
	qrHandler := qr.NewHandler(qrService, logger)

	handler := httpserver.NewRouter(httpserver.RouterDeps{
		Logger:             logger,
		DB:                 poolPinger{pool: pool},
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		AuthMiddleware:     auth.JWTMiddleware(cfg.JWTSigningKey),
		RegisterRoutes: func(r chi.Router) {
			cdsHandler.RegisterRoutes(r)
			rxHandler.RegisterRoutes(r)
			qrHandler.RegisterRoutes(r)
		},
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api server starting", "env", cfg.AppEnv, "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("http server error", "error", err)
		os.Exit(1)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("api server stopped cleanly")
}
