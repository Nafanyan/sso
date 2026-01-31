package suite

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	ssov1 "github.com/Nafanyan/sso-proto/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientCfg — только то, что нужно клиенту: куда стучаться и таймауты.
type ClientCfg struct {
	Port     int32
	Timeout  time.Duration
	TokenTTL time.Duration
}

type Suite struct {
	*testing.T
	Cfg        ClientCfg
	AuthClient ssov1.AuthClient
}

const (
	grpcHost        = "localhost"
	defaultPort     = 8080
	defaultTimeout  = 60 * time.Second
	defaultTokenTTL = time.Hour
)

func New(t *testing.T) (context.Context, *Suite) {
	t.Helper()
	t.Parallel()

	cfg := clientCfg()

	ctx, cancelCtx := context.WithTimeout(context.Background(), cfg.Timeout)
	t.Cleanup(func() {
		t.Helper()
		cancelCtx()
	})

	cc, err := grpc.DialContext(context.Background(),
		net.JoinHostPort(grpcHost, strconv.Itoa(int(cfg.Port))),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc server connection failed: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })

	return ctx, &Suite{
		T:          t,
		Cfg:        cfg,
		AuthClient: ssov1.NewAuthClient(cc),
	}
}

func clientCfg() ClientCfg {
	cfg := ClientCfg{
		Port:     defaultPort,
		Timeout:  defaultTimeout,
		TokenTTL: defaultTokenTTL,
	}

	return cfg
}
