package app

import (
	"context"
	"log/slog"
	grpcapp "sso/internal/app/grpc"
	"sso/internal/app/redis"
	storageapp "sso/internal/app/storage"
	"sso/internal/config"
	"sso/internal/services/auth"
	"time"
)

type App struct {
	gRPCServer *grpcapp.App
	storageApp *storageapp.App
	redisApp   *redis.App
}

func New(
	log *slog.Logger,
	grpcPort int32,
	storagePath string,
	tokenTTL time.Duration,
	redisCfg config.RedisConfig,
) *App {
	storageApp, err := storageapp.New(storagePath, log)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	redisApp, err := redis.New(ctx, redisCfg.Addr, redisCfg.Password, log)
	if err != nil {
		panic(err)
	}

	var loginRateLimitBackend grpcapp.RateLimitBackend
	if redisApp != nil {
		loginRateLimitBackend = grpcapp.NewRedisRateLimitBackend(
			redisApp.Client(),
			redisCfg.RateLimits.LoginLimit,
			redisCfg.RateLimits.LoginWindow)
	}

	authService := auth.New(
		log,
		storageApp.Storage,
		storageApp.Storage,
		storageApp.Storage,
		storageApp.Storage,
		storageApp.Storage,
		storageApp.Storage,
		tokenTTL)

	grpcApp := grpcapp.New(
		log,
		authService,
		grpcPort,
		loginRateLimitBackend)

	return &App{
		gRPCServer: grpcApp,
		storageApp: storageApp,
		redisApp:   redisApp,
	}
}

func (a *App) MustRun() {
	a.gRPCServer.MustRun()
}

func (a *App) Stop() {
	a.gRPCServer.Stop()
	a.storageApp.Storage.Close()
	if a.redisApp != nil {
		a.redisApp.Close()
	}
}
