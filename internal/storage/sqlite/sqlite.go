package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sso/internal/domain/models"
	"sso/internal/storage"
	"time"

	"github.com/mattn/go-sqlite3"
)

type Storage struct {
	db              *sql.DB
	userInsertStmt  *sql.Stmt
	userByEmailStmt *sql.Stmt
	appByIDStmt     *sql.Stmt
}

func New(storagePath string) (storage *Storage, err error) {
	const op = "storage.sqlite.New"

	db, err := sql.Open("sqlite3", storagePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(10 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
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
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, userInsertStmt)

	userByEmailStmt, err := db.Prepare("SELECT id, email, pass_hash FROM users WHERE email = ?")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, userByEmailStmt)

	appByIDStmt, err := db.Prepare("SELECT id, name, secret FROM apps WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	stmts = append(stmts, appByIDStmt)

	storage = &Storage{
		db:              db,
		userInsertStmt:  userInsertStmt,
		userByEmailStmt: userByEmailStmt,
		appByIDStmt:     appByIDStmt,
	}

	return storage, nil
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte) (int64, error) {
	const op = "storage.sqlite.SaveUser"

	res, err := s.userInsertStmt.ExecContext(ctx, email, passHash)
	if err != nil {
		if ctx.Err() != nil {
			return 0, fmt.Errorf("%s: context error: %w", op, ctx.Err())
		}

		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) User(ctx context.Context, email string) (models.User, error) {
	const op = "storage.sqlite.User"

	var user models.User

	err := s.userByEmailStmt.QueryRowContext(ctx, email).Scan(&user.ID, &user.Email, &user.PassHash)
	if err != nil {
		if ctx.Err() != nil {
			return models.User{}, fmt.Errorf("%s: context error: %w", op, ctx.Err())
		}

		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}

		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

func (s *Storage) App(ctx context.Context, appID int32) (models.App, error) {
	const op = "storage.sqlite.App"

	var app models.App

	err := s.appByIDStmt.QueryRowContext(ctx, appID).Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if ctx.Err() != nil {
			return models.App{}, fmt.Errorf("%s: context error: %w", op, ctx.Err())
		}

		if errors.Is(err, sql.ErrNoRows) {
			return models.App{}, fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}

		return models.App{}, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil
}

func (s *Storage) Close() error {
	var errs []error

	if s.userInsertStmt != nil {
		if err := s.userInsertStmt.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close userInsertStmt: %w", err))
		}
	}

	if s.userByEmailStmt != nil {
		if err := s.userByEmailStmt.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close userByEmailStmt: %w", err))
		}
	}

	if s.appByIDStmt != nil {
		if err := s.appByIDStmt.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close appByIDStmt: %w", err))
		}
	}

	if err := s.db.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close db: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing storage: %v", errs)
	}

	return nil
}
