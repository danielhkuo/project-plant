package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/danielkuo/project-plant/services/dashboard/internal/api"
	"github.com/danielkuo/project-plant/services/dashboard/internal/config"
	"github.com/danielkuo/project-plant/services/dashboard/internal/store"
	"github.com/danielkuo/project-plant/services/dashboard/internal/ws"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	// Connect to PostgreSQL
	pool, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer redisClient.Close()

	// Create stores
	redisStore := store.NewRedisDeviceReader(redisClient)
	pgStore := store.NewPostgresStore(pool)

	// Create REST handler
	handler := api.NewHandler(redisStore, pgStore, pgStore, pgStore, logger)

	// Create and start WebSocket hub
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := ws.NewHub()
	go hub.Run(ctx)

	// Start Redis Pub/Sub subscriber
	subscriber := ws.NewRedisSubscriber(redisClient, hub, logger)
	go func() {
		if err := subscriber.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("redis subscriber error", zap.Error(err))
		}
	}()

	// Create WebSocket handler and wire up router
	wsHandler := ws.NewWSHandler(hub, logger)
	router := api.NewRouter(handler, wsHandler, logger)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh

		logger.Info("shutting down dashboard server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		cancel() // Stop hub and subscriber
		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("starting dashboard server", zap.String("addr", cfg.Addr))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatal("server error", zap.Error(err))
	}
}
