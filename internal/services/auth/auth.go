package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sso/internal/domain/models"
	"sso/internal/lib/jwt"
	"sso/internal/lib/logger/sl"
	"sso/internal/storage"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAppNotEnabled  = errors.New("user not have access")
	ErrInvalidToken       = errors.New("invalide token")
)

type UserSaver interface {
	SaveUser(ctx context.Context, email string, passHash []byte) (int64, error)
}

type UserProvider interface {
	User(ctx context.Context, email string) (models.User, error)
}

type AppProvider interface {
	App(ctx context.Context, appCode string) (models.App, error)
}

type UserAppProvider interface {
	UserApp(ctx context.Context, userID int64, appID int32) (models.UserApp, error)
}

type UserAppSaver interface {
	SaveUserApp(ctx context.Context, userID int64, appID int32, isEnabled bool) (int64, error)
}

type UserAppUpdater interface {
	UpdateUserApp(ctx context.Context, userID int64, appID int32, isEnabled bool) error
}

type Auth struct {
	log             *slog.Logger
	userSaver       UserSaver
	userProvider    UserProvider
	appProvider     AppProvider
	userAppProvider UserAppProvider
	userAppSaver    UserAppSaver
	userAppUpdater  UserAppUpdater
	tokenTTL        time.Duration
}

func New(
	log *slog.Logger,
	userSaver UserSaver,
	userProvider UserProvider,
	appProvider AppProvider,
	userAppProvider UserAppProvider,
	userAppSaver UserAppSaver,
	userAppUpdater UserAppUpdater,
	ttl time.Duration,
) *Auth {
	return &Auth{
		log:             log,
		userSaver:       userSaver,
		userProvider:    userProvider,
		appProvider:     appProvider,
		userAppProvider: userAppProvider,
		userAppSaver:    userAppSaver,
		userAppUpdater:  userAppUpdater,
		tokenTTL:        ttl,
	}
}

func (a *Auth) RegisterNewUser(ctx context.Context, email string, password string) (userID int64, err error) {
	const op = "Auth.RegisterNewUser"

	log := a.log.With(
		slog.String("op", op),
		slog.String("email", email),
	)
	log.Info("registering user")

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", sl.Err(err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.userSaver.SaveUser(ctx, email, passHash)
	if err != nil {
		log.Error("failed to save user", sl.Err(err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (a *Auth) Login(ctx context.Context, email string, password string, appCode string) (token string, err error) {
	const op = "Auth.Login"

	log := a.log.With(
		slog.String("op", op),
		slog.String("email", email),
		slog.String("app_code", appCode),
	)

	log.Info("attempting to login user")

	user, err := getUser(ctx, a.userProvider, email, log, op)
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		log.Error("invalid credentials", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	app, err := getApp(ctx, a.appProvider, appCode, log, op)
	if err != nil {
		return "", err
	}

	err = isAccessAllowed(ctx, a.userAppProvider, user.ID, app.ID, log, op)
	if err != nil {
		return "", err
	}

	log.Info("user logged in successfully")

	token, err = jwt.NewToken(user, app, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate token", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return token, nil
}

func (a *Auth) ValidateToken(ctx context.Context, token string, appCode string) (email string, err error) {
	const op = "Auth.ValidateToken"
	log := a.log.With(
		slog.String("op", op),
	)
	log.Info("validating token")

	app, err := getApp(ctx, a.appProvider, appCode, log, op)
	if err != nil {
		return "", err
	}

	email, err = jwt.ValidateToken(token, app.Secret)
	if err != nil {
		log.Error("failed to validate token", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	user, err := getUser(ctx, a.userProvider, email, log, op)
	if err != nil {
		return "", err
	}

	err = isAccessAllowed(ctx, a.userAppProvider, user.ID, app.ID, log, op)
	if err != nil {
		return "", err
	}

	err = isAccessAllowed(ctx, a.userAppProvider, user.ID, app.ID, log, op)
	if err != nil {
		return "", err
	}

	return email, nil
}

func (a *Auth) AccessControl(ctx context.Context, email string, appCode string, isEnabled bool) (string, error) {
	const op = "Auth.AccessControl"

	log := a.log.With(
		slog.String("op", op),
		slog.String("email", email),
		slog.String("appCode", appCode),
		slog.Bool("is_enabled", isEnabled),
	)
	log.Info("start access control")

	user, err := getUser(ctx, a.userProvider, email, log, op)
	if err != nil {
		return "", err
	}

	app, err := getApp(ctx, a.appProvider, appCode, log, op)
	if err != nil {
		return "", err
	}

	userApp, err := getUserApp(ctx, a.userAppProvider, user.ID, app.ID, log, op)
	if err != nil && errors.Is(err, storage.ErrUserAppNotFound) {
		err = saveUserApp(ctx, a.userAppSaver, user.ID, app.ID, isEnabled, log, op)
		if err != nil {
			return "", err
		} else {
			return app.Code, nil
		}
	}

	if err != nil {
		log.Error("failed to get user app", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	err = a.userAppUpdater.UpdateUserApp(ctx, userApp.UserID, userApp.AppID, isEnabled)
	if err != nil {
		return "", err
	}

	return app.Code, nil
}

func getUser(
	ctx context.Context,
	userProvider UserProvider,
	email string,
	log *slog.Logger,
	op string,
) (models.User, error) {
	user, err := userProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Warn("user not found", sl.Err(err))
			return models.User{}, fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}

		log.Error("failed to get user", sl.Err(err))
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}

func getApp(
	ctx context.Context,
	appProvider AppProvider,
	appCode string,
	log *slog.Logger,
	op string,
) (models.App, error) {
	app, err := appProvider.App(ctx, appCode)
	if err != nil {
		log.Error("failed to get app", sl.Err(err))
		return models.App{}, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil
}

func getUserApp(
	ctx context.Context,
	userAppProvider UserAppProvider,
	userID int64,
	appID int32,
	log *slog.Logger,
	op string,
) (models.UserApp, error) {
	userApp, err := userAppProvider.UserApp(ctx, userID, appID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Error("user app not found")
			return models.UserApp{}, fmt.Errorf("%s: %w", op, ErrUserAppNotEnabled)
		}

		log.Error("failed to get user app", sl.Err(err))
		return models.UserApp{}, fmt.Errorf("%s: %w", op, err)
	}

	return userApp, nil
}

func saveUserApp(
	ctx context.Context,
	userAppSaver UserAppSaver,
	userID int64,
	appID int32,
	isEnabled bool,
	log *slog.Logger,
	op string,
) error {
	_, err := userAppSaver.SaveUserApp(ctx, userID, appID, isEnabled)
	if err != nil {
		log.Error("failed to save user app", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func isAccessAllowed(
	ctx context.Context,
	userAppProvider UserAppProvider,
	userID int64,
	appID int32,
	log *slog.Logger,
	op string,
) error {
	userApp, err := getUserApp(ctx, userAppProvider, userID, appID, log, op)
	if err != nil {
		return err
	}

	if !userApp.IsEnabled {
		log.Error("user app is not enabled")
		return fmt.Errorf("%s: %w", op, ErrUserAppNotEnabled)
	}

	return nil
}
