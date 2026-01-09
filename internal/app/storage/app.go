package storage

import (
	"log/slog"
	sqlite "sso/internal/storage/sqlite"
)

type App struct {
	Storage *sqlite.Storage
}

func New(storagePath string, log *slog.Logger) (*App, error) {
	storage, err := sqlite.New(storagePath, log)

	return &App{
		Storage: storage,
	}, err
}
