package app

import (
	"log/slog"
	grpcapp "sso/internal/app/grpc"
	storageapp "sso/internal/app/storage"
	"sso/internal/services/auth"
	"time"
)

type App struct {
	gRPCServer *grpcapp.App
	storageApp *storageapp.App
}

func New(
	log *slog.Logger,
	grpcPort int32,
	storagePath string,
	tokenTTL time.Duration,
) *App {
	storageApp, err := storageapp.New(storagePath, log)
	if err != nil {
		panic(err)
	}

	authService := auth.New(log, storageApp.Storage, storageApp.Storage, storageApp.Storage, tokenTTL)
	grpcApp := grpcapp.New(log, authService, grpcPort)

	return &App{
		gRPCServer: grpcApp,
		storageApp: storageApp,
	}
}

func (a *App) MustRun() {
	a.gRPCServer.MustRun()
}

func (a *App) Stop() {
	a.gRPCServer.Stop()
	if err := a.storageApp.Storage.Close(); err != nil {
		// Логируем ошибку закрытия storage, но не паникуем
		// так как приложение уже завершается
	}
}
