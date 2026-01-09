package auth

import (
	"context"
	"errors"
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
	msgInvalidEmail       = "invalid email format"
	msgPasswordTooShort   = "password must be at least 8 characters"
	msgInvalidCredentials = "invalid email or password"
	msgUserExists         = "user already exists"
	msgLoginFailed        = "failed to login"
	msgRegisterFailed     = "failed to register user"
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
		appId int32,
	) (token string, err error)
	RegisterNewUser(
		ctx context.Context,
		email string,
		password string,
	) (userId int64, err error)
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

	if in.GetAppId() == 0 {
		return nil, status.Error(codes.InvalidArgument, msgAppIDRequired)
	}

	token, err := s.auth.Login(ctx, in.Email, in.Password, int32(in.GetAppId()))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, status.Error(codes.InvalidArgument, msgInvalidCredentials)
		}

		return nil, status.Error(codes.Internal, msgLoginFailed)
	}

	return &ssov1.LoginResponse{Token: token}, nil
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
