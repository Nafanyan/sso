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
	ErrAppNotFound        = errors.New("App not found")
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

	// Генерация хэша от пароля
	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", sl.Err(err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	// Сохранение User в БД
	id, err := a.userSaver.SaveUser(ctx, email, passHash)
	if err != nil {
		log.Error("failed to save user", sl.Err(err))

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user registered is successfully")

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

	// Получение User
	user, err := getUser(ctx, a.userProvider, email, log, op)
	if err != nil {
		return "", err
	}

	// Проверка валидности пароля по хэшу
	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		log.Error("invalid credentials", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	// Получение App
	app, err := getApp(ctx, a.appProvider, appCode, log, op)
	if err != nil {
		return "", err
	}

	// Получение UserApp, если нет - создаём новый с доступом. При гонке несколько запросов
	// могут получить ErrUserAppNotFound; первый создаёт запись, остальные получают ErrUserAppExists — это ок.
	_, err = getUserApp(ctx, a.userAppProvider, user.ID, app.ID, log, op)
	if err != nil && errors.Is(err, storage.ErrUserAppNotFound) {
		err = saveUserApp(ctx, a.userAppSaver, user.ID, app.ID, true, log, op)
		if err != nil {
			if errors.Is(err, storage.ErrUserAppExists) {
				// Запись уже создана другим запросом — продолжаем, выдаём токен
				err = nil
			} else {
				return "", err
			}
		}
	}

	if err != nil {
		log.Error("failed to get user app", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	// Генерация токена
	token, err = jwt.NewToken(user, app, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate token", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged is successfully")

	return token, nil
}

func (a *Auth) Logout(ctx context.Context, email string, appCode string) (isSuccess bool, err error) {
	const op = "Auth.Logout"
	log := a.log.With(
		slog.String("op", op),
		slog.String("email", email),
		slog.String("app_code", appCode),
	)
	log.Info("attempting to logout user")

	// Получение User
	user, err := getUser(ctx, a.userProvider, email, log, op)
	if err != nil {
		return false, err
	}

	// Получение App
	app, err := getApp(ctx, a.appProvider, appCode, log, op)
	if err != nil {
		return false, err
	}

	// Получение UserApp
	userApp, err := getUserApp(ctx, a.userAppProvider, user.ID, app.ID, log, op)
	if err != nil {
		log.Error("failed to get user app", sl.Err(err))
		return false, fmt.Errorf("%s: %w", op, err)
	}

	// Запрет доступа User к App
	err = a.userAppUpdater.UpdateUserApp(ctx, userApp.UserID, userApp.AppID, false)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (a *Auth) ValidateToken(ctx context.Context, token string, appCode string) (email string, err error) {
	const op = "Auth.ValidateToken"
	log := a.log.With(
		slog.String("op", op),
	)
	log.Info("validating token")

	// Получение App
	app, err := getApp(ctx, a.appProvider, appCode, log, op)
	if err != nil {
		return "", err
	}

	// Валидация токена
	email, err = jwt.ValidateToken(token, app.Secret)
	if err != nil {
		log.Error("failed to validate token", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	// Получение User
	user, err := getUser(ctx, a.userProvider, email, log, op)
	if err != nil {
		return "", err
	}

	// Проверка доступа User к App
	err = isAccessAllowed(ctx, a.userAppProvider, user.ID, app.ID, log, op)
	if err != nil {
		return "", err
	}
	log.Info("token validated is successfully")

	return email, nil
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
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Warn("app not found", sl.Err(err))
			return models.App{}, fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}
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
