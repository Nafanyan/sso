package jwt

import (
	"errors"
	"fmt"
	"sso/internal/domain/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

func NewToken(user models.User, app models.App, duration time.Duration) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["uid"] = user.ID
	claims["email"] = user.Email
	claims["exp"] = time.Now().Add(duration).Unix()
	claims["app_code"] = app.Code

	tokenString, err := token.SignedString([]byte(app.Secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ValidateToken(token string, secretApp string) (email string, err error) {
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretApp), nil
	})

	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrTokenInvalid, err)
	}

	if !parsedToken.Valid {
		return "", ErrTokenInvalid
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", ErrTokenInvalid
	}

	emailClaim, ok := claims["email"].(string)
	if !ok {
		return "", fmt.Errorf("%w: email claim is missing or invalid", ErrTokenInvalid)
	}

	expClaim, ok := claims["exp"].(float64)
	if !ok {
		return "", fmt.Errorf("%w: exp claim is missing or invalid", ErrTokenInvalid)
	}

	expTime := time.Unix(int64(expClaim), 0)
	if time.Now().After(expTime) {
		return "", ErrTokenExpired
	}

	return emailClaim, nil
}
