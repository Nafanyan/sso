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

// RateLimitBackend — интерфейс счётчика попыток (например Redis).
type RateLimitBackend interface {
	GetMaxLimit() int64
	GetWindow() time.Duration
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// RedisRateLimitBackend реализует RateLimitBackend через Redis.
type RedisRateLimitBackend struct {
	client *redis.Client
	limit  int64
	window time.Duration
}

func NewRedisRateLimitBackend(client *redis.Client, limit int64, window time.Duration) RateLimitBackend {
	if client == nil {
		return nil
	}
	return &RedisRateLimitBackend{
		client: client,
		limit:  limit,
		window: window,
	}
}

func (r *RedisRateLimitBackend) GetMaxLimit() int64 {
	return r.limit
}

func (r *RedisRateLimitBackend) GetWindow() time.Duration {
	return r.window
}

func (r *RedisRateLimitBackend) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *RedisRateLimitBackend) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

// LoginRateLimiter — интерцептор, ограничивающий число попыток логина по email.
type LoginRateLimiter struct {
	log              *slog.Logger
	rateLimitBackend RateLimitBackend
}

func NewLoginRateLimiter(log *slog.Logger, rateLimitBackend RateLimitBackend) *LoginRateLimiter {
	return &LoginRateLimiter{
		log:              log.With(slog.String("component", "login_rate_limiter")),
		rateLimitBackend: rateLimitBackend,
	}
}

func (l *LoginRateLimiter) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != grpcMethodAuthLogin || l.rateLimitBackend == nil {
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

		attempts, err := l.rateLimitBackend.Incr(ctx, key)
		if err != nil {
			l.log.Error("rate limit incr failed", slog.String("email", email), slog.Any("err", err))
			return handler(ctx, req)
		}

		if attempts == 1 {
			_ = l.rateLimitBackend.Expire(ctx, key, l.rateLimitBackend.GetWindow())
		}

		if attempts > l.rateLimitBackend.GetMaxLimit() {
			l.log.Warn("too many login attempts", slog.String("email", email), slog.Int64("attempts", attempts))
			return nil, status.Error(codes.ResourceExhausted, "too many login attempts, try again later")
		}

		return handler(ctx, req)
	}
}
