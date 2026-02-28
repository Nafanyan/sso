package tests

import (
	"sso/tests/suite"
	"sync"
	"testing"

	ssov1 "github.com/Nafanyan/sso-proto/gen/go/sso"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRegisterLoginRateLimit_ManyRequestLogin_HappyPath(t *testing.T) {
	ctx, st := suite.New(t)
	rateLimitCount := 5
	requestCount := 10

	email := gofakeit.Email()
	pass := randomFakePassword()

	_, err := st.AuthClient.Register(ctx, &ssov1.RegisterRequest{
		Email:    email,
		Password: pass,
	})
	require.NoError(t, err)

	appCode := "test"

	resLogin := make(chan error, requestCount)
	wg := sync.WaitGroup{}

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, loginErr := st.AuthClient.Login(ctx, &ssov1.LoginRequest{
				Email:    email,
				Password: pass,
				AppCode:  appCode,
			})
			resLogin <- loginErr
		}()
	}
	wg.Wait()
	close(resLogin)

	var successCount, rateLimitErrors int
	for resErr := range resLogin {
		if resErr == nil {
			successCount++
			continue
		}
		st, ok := status.FromError(resErr)
		if ok && st.Code() == codes.ResourceExhausted {
			rateLimitErrors++
		}
	}
	require.Equal(t, rateLimitCount, successCount, "должно быть 5 успешных логина ")
	require.Equal(t, requestCount-rateLimitCount, rateLimitErrors, "пять запросов должны получить rate limit (лимит 5 на окно)")
}
