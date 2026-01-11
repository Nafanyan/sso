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
	db                          *sql.DB
	userInsertStmt              *sql.Stmt
	userByEmailStmt             *sql.Stmt
	appByCodeStmt               *sql.Stmt
	userAppByUserIdAndAppIdStmt *sql.Stmt
	userAppInsertStmt           *sql.Stmt
	userAppUpdateStmt           *sql.Stmt
	log                         *slog.Logger
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

	appByCodeStmt, err := db.Prepare("SELECT id, name, secret FROM apps WHERE code = ?")
	if err != nil {
		opLog.Error("failed to prepare app by code statement", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, appByCodeStmt)

	userAppByUserIdAndAppIdStmt, err := db.Prepare(`
		SELECT user_id, app_id, is_enabled 
		FROM user_app 
		WHERE user_id = ? AND app_id = ?`)
	if err != nil {
		opLog.Error("failed to prepare userApp by user id and app id statement", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, userAppByUserIdAndAppIdStmt)

	userAppInsertStmt, err := db.Prepare(`
		INSERT INTO user_app (user_id, app_id, is_enabled) VALUES (?, ?, ?)
	`)
	if err != nil {
		opLog.Error("failed to prepare userApp insert statement", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, userAppInsertStmt)

	userAppUpdateStmt, err := db.Prepare(`
		UPDATE user_app SET is_enabled = ? WHERE user_id = ? AND app_id = ?
	`)
	if err != nil {
		opLog.Error("failed to prepare userApp update statement", sl.Err(err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, userAppUpdateStmt)

	storage = &Storage{
		db:                          db,
		userInsertStmt:              userInsertStmt,
		userByEmailStmt:             userByEmailStmt,
		appByCodeStmt:               appByCodeStmt,
		userAppByUserIdAndAppIdStmt: userAppByUserIdAndAppIdStmt,
		userAppInsertStmt:           userAppInsertStmt,
		userAppUpdateStmt:           userAppUpdateStmt,
		log:                         log,
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

func (s *Storage) App(ctx context.Context, appCode string) (models.App, error) {
	const op = "storage.sqlite.App"

	log := s.log.With(
		slog.String("op", op),
		slog.String("app_code", appCode),
	)

	var app models.App

	err := s.appByCodeStmt.QueryRowContext(ctx, appCode).Scan(&app.ID, &app.Code, &app.Secret)
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

func (s *Storage) UserApp(ctx context.Context, userID int64, appID int32) (models.UserApp, error) {
	const op = "storage.sqlite.UserApp"

	log := s.log.With(
		slog.String("op", op),
		slog.Int64("user_id", userID),
		slog.Int("app_id", int(appID)),
	)

	var userApp models.UserApp

	err := s.userAppByUserIdAndAppIdStmt.QueryRowContext(ctx, userID, appID).
		Scan(&userApp.UserID, &userApp.AppID, &userApp.IsEnabled)
	if err != nil {
		if ctx.Err() != nil {
			err := fmt.Errorf("%s: context error: %w", op, ctx.Err())
			log.Error("failed to get userApp: context error", sl.Err(err))
			return models.UserApp{}, err
		}

		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("userApp not found")
			return models.UserApp{}, fmt.Errorf("%s: %w", op, storage.ErrUserAppNotFound)
		}

		log.Error("failed to get userApp", sl.Err(err))
		return models.UserApp{}, fmt.Errorf("%s: %w", op, err)
	}

	return userApp, nil
}

func (s *Storage) SaveUserApp(
	ctx context.Context,
	userID int64,
	appID int32,
	isEnabled bool,
) (int64, error) {
	const op = "storage.sqlite.SaveUserApp"

	log := s.log.With(
		slog.String("op", op),
		slog.Int64("user_id", userID),
		slog.Int("app_id", int(appID)),
	)

	res, err := s.userInsertStmt.Exec(ctx, userID, appID, isEnabled)
	if err != nil {
		if ctx.Err() != nil {
			err := fmt.Errorf("%s: context error: %w", op, ctx.Err())
			log.Error("failed to save userApp: context error", sl.Err(err))
			return 0, err
		}

		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			log.Warn("failed to save userApp: userApp already exists")
			return 0, fmt.Errorf("%s: %w", op, storage.ErrUserAppExists)
		}

		log.Error("failed to save userApp", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		log.Error("failed to get last insert id", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) UpdateUserApp(ctx context.Context, userID int64, appID int32, isEnabled bool) error {
	const op = "storage.sqlite.UpdateUserApp"

	log := s.log.With(
		slog.String("op", op),
		slog.Int64("user_id", userID),
		slog.Int("app_id", int(appID)),
		slog.Bool("is_enabled", isEnabled),
	)

	res, err := s.userAppUpdateStmt.ExecContext(ctx, isEnabled, userID, appID)
	if err != nil {
		if ctx.Err() != nil {
			err := fmt.Errorf("%s: context error: %w", op, ctx.Err())
			log.Error("failed to update userApp: context error", sl.Err(err))
			return err
		}

		log.Error("failed to update userApp", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Error("failed to get rows affected", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	if rowsAffected == 0 {
		log.Warn("userApp not found for update")
		return fmt.Errorf("%s: %w", op, storage.ErrUserAppNotFound)
	}

	log.Info("userApp updated successfully")
	return nil
}

func (s *Storage) Close() error {
	const op = "storage.sqlite.Close"

	if s == nil {
		return nil
	}

	log := s.log.With(slog.String("op", op))
	var errs []error

	if s.userAppUpdateStmt != nil {
		if err := s.userAppUpdateStmt.Close(); err != nil {
			log.Error("failed to close userApp update statement", sl.Err(err))
			errs = append(errs, fmt.Errorf("close userAppUpdateStmt: %w", err))
		}
		s.userAppUpdateStmt = nil
	}

	if s.userAppInsertStmt != nil {
		if err := s.userAppInsertStmt.Close(); err != nil {
			log.Error("failed to close userApp insert statement", sl.Err(err))
			errs = append(errs, fmt.Errorf("close userAppInsertStmt: %w", err))
		}
		s.userAppInsertStmt = nil
	}

	if s.userAppByUserIdAndAppIdStmt != nil {
		if err := s.userAppByUserIdAndAppIdStmt.Close(); err != nil {
			log.Error("failed to close app by id statement", sl.Err(err))
			errs = append(errs, fmt.Errorf("close userAppByUserIdAndAppIdStmt: %w", err))
		}
		s.userAppByUserIdAndAppIdStmt = nil
	}

	if s.appByCodeStmt != nil {
		if err := s.appByCodeStmt.Close(); err != nil {
			log.Error("failed to close app by id statement", sl.Err(err))
			errs = append(errs, fmt.Errorf("close appByCodeStmt: %w", err))
		}
		s.appByCodeStmt = nil
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
