package redis

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const pingTimeout = 3 * time.Second

// App — обёртка над Redis-клиентом с логгером и graceful close.
type App struct {
	log    *slog.Logger
	client *redis.Client
}

// New создаёт подключение к Redis. При пустом addr возвращает (nil, nil) — Redis отключён.
func New(ctx context.Context, addr, password string, log *slog.Logger) (*App, error) {
	const op = "redis.New"
	redisLog := log.With(slog.String("op", op), slog.String("addr", addr))

	if addr == "" {
		redisLog.Info("redis addr is empty, skipping")
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	redisLog.Info("redis connected")

	return &App{
		log:    log,
		client: client,
	}, nil
}

// Client возвращает клиент Redis для использования в других пакетах. Если Redis отключён — nil.
func (a *App) Client() *redis.Client {
	if a == nil {
		return nil
	}
	return a.client
}

// Close закрывает соединение с Redis. Безопасен для вызова при nil.
func (a *App) Close() {
	if a == nil || a.client == nil {
		return
	}
	if err := a.client.Close(); err != nil {
		a.log.With(slog.String("op", "redis.Close")).Error("failed to close redis", slog.Any("err", err))
	}
}
