package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/pkg/logging"
	"github.com/danielkuo/project-plant/services/ingestion/internal/api"
	"github.com/danielkuo/project-plant/services/ingestion/internal/auth"
	"github.com/danielkuo/project-plant/services/ingestion/internal/config"
	"github.com/danielkuo/project-plant/services/ingestion/internal/kafka"
	"github.com/danielkuo/project-plant/services/ingestion/internal/metrics"
)

func main() {
	logger := logging.MustNew("ingestion")
	defer logger.Sync()

	cfg := config.Load()

	producer := kafka.NewKafkaProducer(cfg.Kafka)
	defer producer.Close()

	m := metrics.New()

	// Auth protects the /api/v1/* routes only; /health and /metrics stay public.
	router := api.NewRouter(producer, auth.Middleware(cfg.Auth), m, logger)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Cancel on SIGTERM/SIGINT, then drain in-flight requests via Shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("starting ingestion server", zap.String("addr", cfg.Addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatal("server error", zap.Error(err))
	}
}
