package tests

import (
	"sso/tests/suite"
	"testing"

	ssov1 "github.com/Nafanyan/sso-proto/gen/go/sso"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/require"
)

func TestRegisterLoginLogout_Logout_HappyPath(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()
	pass := randomFakePassword()

	_, err := st.AuthClient.Register(ctx, &ssov1.RegisterRequest{
		Email:    email,
		Password: pass,
	})
	require.NoError(t, err)

	respLogin, err := st.AuthClient.Login(ctx, &ssov1.LoginRequest{
		Email:    email,
		Password: pass,
		AppCode:  appCode,
	})
	require.NoError(t, err)
	require.NotEmpty(t, respLogin.GetToken())
	token := respLogin.GetToken()

	respLogout, err := st.AuthClient.Logout(ctx, &ssov1.LogoutRequest{
		Email:   email,
		AppCode: appCode,
	})
	require.NoError(t, err)
	require.True(t, respLogout.GetSuccess())

	respValidateToken, err := st.AuthClient.Validate(ctx, &ssov1.ValidateTokenRequest{
		Token:   token,
		AppCode: appCode,
	})
	require.False(t, respValidateToken.GetSuccess())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Access denied")
}

func TestLogout_FailCases(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()
	pass := randomFakePassword()

	_, _ = st.AuthClient.Register(ctx, &ssov1.RegisterRequest{
		Email:    email,
		Password: pass,
	})
	_, _ = st.AuthClient.Login(ctx, &ssov1.LoginRequest{
		Email:    email,
		Password: pass,
		AppCode:  appCode,
	})

	tests := []struct {
		name        string
		email       string
		appCode     string
		expectedErr string
	}{
		{
			name:        "email is empty",
			email:       "",
			appCode:     appCode,
			expectedErr: "email is required",
		},
		{
			name:        "appCode is empty",
			email:       email,
			appCode:     "",
			expectedErr: "app_code is required",
		},
		{
			name:        "email is not correct",
			email:       "notExist@mail.ru",
			appCode:     appCode,
			expectedErr: "User not found",
		},
		{
			name:        "app is not found",
			email:       email,
			appCode:     "app1241232",
			expectedErr: "App not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := st.AuthClient.Logout(ctx, &ssov1.LogoutRequest{
				Email:   tt.email,
				AppCode: tt.appCode,
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}
