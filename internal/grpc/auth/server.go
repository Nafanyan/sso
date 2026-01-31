package auth

import (
	"context"
	"errors"
	"sso/internal/lib/jwt"
	"sso/internal/services/auth"
	"sso/internal/storage"

	ssov1 "github.com/Nafanyan/sso-proto/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	msgEmailRequired      = "email is required"
	msgPasswordRequired   = "password is required"
	msgAppIDRequired      = "app_id is required"
	msgAppCodeRequired    = "app_code is required"
	msgInvalidEmail       = "invalid email format"
	msgPasswordTooShort   = "password must be at least 8 characters"
	msgInvalidCredentials = "invalid email or password"
	msgUserExists         = "user already exists"
	msgLoginFailed        = "failed to login"
	msgRegisterFailed     = "failed to register user"
	msgTokenRequired      = "Token is required"
	msgTokenExpired       = "Token is expired"
	msgTokenInvalid       = "Token is invalid"
	msgUserAppNotEnabled  = "Access denied"
)

type serverAPI struct {
	ssov1.UnimplementedAuthServer
	auth Auth
}

type Auth interface {
	Login(
		ctx context.Context,
		email string,
		password string,
		appCode string,
	) (token string, err error)
	Logout(
		ctx context.Context,
		email string,
		appCode string,
	) (isSuccess bool, err error)
	RegisterNewUser(
		ctx context.Context,
		email string,
		password string,
	) (userID int64, err error)
	ValidateToken(
		ctx context.Context,
		token string,
		appCode string,
	) (email string, err error)
}

func Register(gRPCServer *grpc.Server, auth Auth) {
	ssov1.RegisterAuthServer(gRPCServer, &serverAPI{
		auth: auth,
	})
}

func (s *serverAPI) Login(ctx context.Context, in *ssov1.LoginRequest) (*ssov1.LoginResponse, error) {
	if in.Email == "" {
		return nil, status.Error(codes.InvalidArgument, msgEmailRequired)
	}

	if in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, msgPasswordRequired)
	}

	if in.GetAppCode() == "" {
		return nil, status.Error(codes.InvalidArgument, msgAppCodeRequired)
	}

	token, err := s.auth.Login(ctx, in.Email, in.Password, in.GetAppCode())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, status.Error(codes.InvalidArgument, msgInvalidCredentials)
		}

		if errors.Is(err, auth.ErrUserAppNotEnabled) {
			return nil, status.Error(codes.Unauthenticated, msgUserAppNotEnabled)
		}

		return nil, status.Error(codes.Internal, msgLoginFailed)
	}

	return &ssov1.LoginResponse{Token: token}, nil
}

func (s *serverAPI) Logout(ctx context.Context, in *ssov1.LogoutRequest) (*ssov1.LogoutResponse, error) {
	if in.Email == "" {
		return &ssov1.LogoutResponse{
			Success: false,
		}, status.Error(codes.InvalidArgument, msgEmailRequired)
	}

	if in.AppCode == "" {
		return &ssov1.LogoutResponse{
			Success: false,
		}, status.Error(codes.InvalidArgument, msgAppCodeRequired)
	}

	isSuccess, err := s.auth.Logout(ctx, in.Email, in.AppCode)
	if err != nil {
		return nil, status.Error(codes.Internal, msgRegisterFailed)
	}

	return &ssov1.LogoutResponse{Success: isSuccess}, nil
}

func (s *serverAPI) Register(ctx context.Context, in *ssov1.RegisterRequest) (*ssov1.RegisterResponse, error) {
	if in.Email == "" {
		return nil, status.Error(codes.InvalidArgument, msgEmailRequired)
	}

	if in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, msgPasswordRequired)
	}

	if len(in.Email) > 254 || len(in.Email) < 3 {
		return nil, status.Error(codes.InvalidArgument, msgInvalidEmail)
	}

	if len(in.Password) < 8 {
		return nil, status.Error(codes.InvalidArgument, msgPasswordTooShort)
	}

	uid, err := s.auth.RegisterNewUser(ctx, in.GetEmail(), in.GetPassword())
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			return nil, status.Error(codes.AlreadyExists, msgUserExists)
		}

		return nil, status.Error(codes.Internal, msgRegisterFailed)
	}

	return &ssov1.RegisterResponse{UserId: uid}, nil
}

func (s *serverAPI) Validate(ctx context.Context, in *ssov1.ValidateTokenRequest) (*ssov1.ValidateTokenResponse, error) {
	if in.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, msgTokenRequired)
	}

	if in.GetAppCode() == "" {
		return nil, status.Error(codes.InvalidArgument, msgAppCodeRequired)
	}

	email, err := s.auth.ValidateToken(ctx, in.GetToken(), in.GetAppCode())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, status.Error(codes.Unauthenticated, msgTokenExpired)
		}

		if errors.Is(err, auth.ErrUserAppNotEnabled) {
			return nil, status.Error(codes.Unauthenticated, msgUserAppNotEnabled)
		}

		return nil, status.Error(codes.Unauthenticated, msgTokenInvalid)

	}

	return &ssov1.ValidateTokenResponse{Email: email}, nil
}
