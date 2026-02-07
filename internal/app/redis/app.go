package redis

import (
	"context"
	"log/slog"
	"sso/internal/lib/logger/sl"
	"time"

	"github.com/redis/go-redis/v9"
)

type App struct {
	log    *slog.Logger
	Client *redis.Client
}

func New(ctx context.Context, addr string, password string, log *slog.Logger) (*App, error) {
	const op = "redisapp.New"

	if addr == "" {
		log.With(slog.String("op", op)).Info("redis addr is empty, skipping redis init")
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.With(slog.String("op", op), slog.String("addr", addr)).Info("redis connected")

	return &App{
		log:    log,
		Client: client,
	}, nil
}

func (a *App) Close() {
	const op = "redisapp.Close"
	log := a.log.With(slog.String("op", op))

	if a == nil || a.Client == nil {
		log.Error("redis app is nil or client is nil, skipping redis close")
		return
	}

	if err := a.Client.Close(); err != nil {
		log.Error("failed to close redis", sl.Err(err))
	}
}
