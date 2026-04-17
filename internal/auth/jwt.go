package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UserID    uint64
	ExpiresAt time.Time
}

type Signer interface {
	Sign(userID uint64) (token string, expiresAt time.Time, err error)
}

type Verifier interface {
	Verify(token string) (*Claims, error)
}

type jwtClaims struct {
	UserID uint64 `json:"uid"`
	jwt.RegisteredClaims
}

type HS256JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewHS256JWT(secret []byte, ttl time.Duration) *HS256JWT {
	return &HS256JWT{secret: secret, ttl: ttl}
}

func (j *HS256JWT) Sign(userID uint64) (string, time.Time, error) {
	expiresAt := time.Now().Add(j.ttl)
	claims := jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, expiresAt, nil
}

func (j *HS256JWT) Verify(tokenStr string) (*Claims, error) {
	var claims jwtClaims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return j.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return &Claims{
		UserID:    claims.UserID,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
