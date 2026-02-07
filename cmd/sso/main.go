package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sso/internal/app"
	"sso/internal/config"
	"sso/internal/lib/logger"
	"sso/internal/lib/logger/sl"
	"syscall"
	"time"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	ssoApplication := app.New(
		log,
		cfg.GRPC.Port,
		cfg.StoragePath,
		cfg.TokenTTL,
		cfg.Redis.Addr,
		cfg.Redis.Password)

	go func() {
		ssoApplication.MustRun()
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	// Waiting for SIGINT (pkill -2) or SIGTERM
	<-stop

	const op = "main.shutdown"
	shutdownLog := log.With(slog.String("op", op))

	shutdownLog.Info("shutting down gracefully...")

	// Создаем контекст с таймаутом для graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Запускаем graceful shutdown в отдельной горутине
	done := make(chan error, 1)
	go func() {
		ssoApplication.Stop()
		done <- nil
	}()

	// Ждем завершения или таймаута
	select {
	case <-ctx.Done():
		shutdownLog.Error("shutdown timeout exceeded, forcing exit")
		return
	case err := <-done:
		if err != nil {
			shutdownLog.Error("error during shutdown", sl.Err(err))
			return
		}
		shutdownLog.Info("gracefully stopped")
	}
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(logger.NewPrettyHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case envDev:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	case envProd:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	default:
		log = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	return log
}
