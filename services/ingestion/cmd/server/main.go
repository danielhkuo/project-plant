package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/ingestion/internal/api"
	"github.com/danielkuo/project-plant/services/ingestion/internal/auth"
	"github.com/danielkuo/project-plant/services/ingestion/internal/config"
	"github.com/danielkuo/project-plant/services/ingestion/internal/kafka"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	producer := kafka.NewKafkaProducer(cfg.Kafka)
	defer producer.Close()

	router := api.NewRouter(producer, logger)

	// Wrap with auth middleware
	authedRouter := auth.Middleware(cfg.Auth)(router)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      authedRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh

		logger.Info("shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		srv.Shutdown(ctx)
	}()

	logger.Info("starting ingestion server", zap.String("addr", cfg.Addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatal("server error", zap.Error(err))
	}
}
