package grpc

import (
	"context"
	"log/slog"
	"time"

	ssov1 "github.com/Nafanyan/sso-proto/gen/go/sso"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	grpcMethodAuthLogin = "/auth.Auth/Login"
	redisKeyLoginPrefix = "rate:login:email:"
)

// LoginRateLimitBackend — хранилище счётчика попыток логина (например Redis).
type LoginRateLimitBackend interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// RedisLoginRateLimitBackend реализует LoginRateLimitBackend через Redis.
type RedisLoginRateLimitBackend struct {
	client *redis.Client
}

// NewRedisLoginRateLimitBackend возвращает backend для rate limiter. При client == nil возвращает nil.
func NewRedisLoginRateLimitBackend(client *redis.Client) LoginRateLimitBackend {
	if client == nil {
		return nil
	}
	return &RedisLoginRateLimitBackend{client: client}
}

func (r *RedisLoginRateLimitBackend) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *RedisLoginRateLimitBackend) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

// LoginRateLimiter — интерцептор, ограничивающий число попыток логина по email.
type LoginRateLimiter struct {
	log    *slog.Logger
	store  LoginRateLimitBackend
	limit  int64
	window time.Duration
}

// NewLoginRateLimiter создаёт интерцептор. При store == nil лимит не применяется.
func NewLoginRateLimiter(log *slog.Logger, store LoginRateLimitBackend, limit int64, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		log:    log.With(slog.String("component", "login_rate_limiter")),
		store:  store,
		limit:  limit,
		window: window,
	}
}

func (l *LoginRateLimiter) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != grpcMethodAuthLogin || l.store == nil {
			return handler(ctx, req)
		}

		loginReq, ok := req.(*ssov1.LoginRequest)
		if !ok {
			return handler(ctx, req)
		}
		email := loginReq.GetEmail()
		if email == "" {
			return handler(ctx, req)
		}

		key := redisKeyLoginPrefix + email

		attempts, err := l.store.Incr(ctx, key)
		if err != nil {
			l.log.Error("rate limit incr failed", slog.String("email", email), slog.Any("err", err))
			return handler(ctx, req)
		}

		if attempts == 1 {
			_ = l.store.Expire(ctx, key, l.window)
		}

		if attempts > l.limit {
			l.log.Warn("too many login attempts", slog.String("email", email), slog.Int64("attempts", attempts))
			return nil, status.Error(codes.ResourceExhausted, "too many login attempts, try again later")
		}

		return handler(ctx, req)
	}
}
