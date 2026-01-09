package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sso/internal/domain/models"
	"sso/internal/lib/logger/sl"
	"sso/internal/storage"
	"time"

	"github.com/mattn/go-sqlite3"
)

type Storage struct {
	db              *sql.DB
	userInsertStmt  *sql.Stmt
	userByEmailStmt *sql.Stmt
	appByIDStmt     *sql.Stmt
	log             *slog.Logger
}

func New(storagePath string, log *slog.Logger) (storage *Storage, err error) {
	const op = "storage.sqlite.New"
	opLog := log.With(slog.String("op", op))

	db, err := sql.Open("sqlite3", storagePath)
	if err != nil {
		opLog.Error("failed to open database", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(10 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		opLog.Error("failed to ping database", sl.Err(err))
		return nil, fmt.Errorf("%s: ping failed: %w", op, err)
	}

	var stmts []*sql.Stmt
	defer func() {
		if err != nil {
			for _, stmt := range stmts {
				if stmt != nil {
					stmt.Close()
				}
			}
			db.Close()
		}
	}()

	userInsertStmt, err := db.Prepare("INSERT INTO users(email, pass_hash) VALUES(?, ?)")
	if err != nil {
		opLog.Error("failed to prepare user insert statement", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, userInsertStmt)

	userByEmailStmt, err := db.Prepare("SELECT id, email, pass_hash FROM users WHERE email = ?")
	if err != nil {
		opLog.Error("failed to prepare user by email statement", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, userByEmailStmt)

	appByIDStmt, err := db.Prepare("SELECT id, name, secret FROM apps WHERE id = ?")
	if err != nil {
		opLog.Error("failed to prepare app by id statement", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, appByIDStmt)

	storage = &Storage{
		db:              db,
		userInsertStmt:  userInsertStmt,
		userByEmailStmt: userByEmailStmt,
		appByIDStmt:     appByIDStmt,
		log:             log,
	}

	return storage, nil
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte) (int64, error) {
	const op = "storage.sqlite.SaveUser"

	log := s.log.With(
		slog.String("op", op),
		slog.String("email", email),
	)

	res, err := s.userInsertStmt.ExecContext(ctx, email, passHash)
	if err != nil {
		if ctx.Err() != nil {
			err := fmt.Errorf("%s: context error: %w", op, ctx.Err())
			log.Error("failed to save user: context error", sl.Err(err))
			return 0, err
		}

		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			log.Warn("failed to save user: user already exists")
			return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}

		log.Error("failed to save user", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		log.Error("failed to get last insert id", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) User(ctx context.Context, email string) (models.User, error) {
	const op = "storage.sqlite.User"

	log := s.log.With(
		slog.String("op", op),
		slog.String("email", email),
	)

	var user models.User

	err := s.userByEmailStmt.QueryRowContext(ctx, email).Scan(&user.ID, &user.Email, &user.PassHash)
	if err != nil {
		if ctx.Err() != nil {
			err := fmt.Errorf("%s: context error: %w", op, ctx.Err())
			log.Error("failed to get user: context error", sl.Err(err))
			return models.User{}, err
		}

		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("user not found")
			return models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		log.Error("failed to get user", sl.Err(err))
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

func (s *Storage) App(ctx context.Context, appID int32) (models.App, error) {
	const op = "storage.sqlite.App"

	log := s.log.With(
		slog.String("op", op),
		slog.Int("app_id", int(appID)),
	)

	var app models.App

	err := s.appByIDStmt.QueryRowContext(ctx, appID).Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if ctx.Err() != nil {
			err := fmt.Errorf("%s: context error: %w", op, ctx.Err())
			log.Error("failed to get app: context error", sl.Err(err))
			return models.App{}, err
		}

		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("app not found")
			return models.App{}, fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		log.Error("failed to get app", sl.Err(err))
		return models.App{}, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil
}

func (s *Storage) Close() error {
	const op = "storage.sqlite.Close"

	if s == nil {
		return nil
	}

	log := s.log.With(slog.String("op", op))
	var errs []error

	if s.appByIDStmt != nil {
		if err := s.appByIDStmt.Close(); err != nil {
			log.Error("failed to close app by id statement", sl.Err(err))
			errs = append(errs, fmt.Errorf("close appByIDStmt: %w", err))
		}
		s.appByIDStmt = nil
	}

	if s.userByEmailStmt != nil {
		if err := s.userByEmailStmt.Close(); err != nil {
			log.Error("failed to close user by email statement", sl.Err(err))
			errs = append(errs, fmt.Errorf("close userByEmailStmt: %w", err))
		}
		s.userByEmailStmt = nil
	}

	if s.userInsertStmt != nil {
		if err := s.userInsertStmt.Close(); err != nil {
			log.Error("failed to close user insert statement", sl.Err(err))
			errs = append(errs, fmt.Errorf("close userInsertStmt: %w", err))
		}
		s.userInsertStmt = nil
	}

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Error("failed to close database", sl.Err(err))
			errs = append(errs, fmt.Errorf("close db: %w", err))
		}
		s.db = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing storage: %v", errs)
	}

	log.Info("storage closed successfully")
	return nil
}
