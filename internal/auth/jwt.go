package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UID       uuid.UUID
	ExpiresAt time.Time
}

type Signer interface {
	Sign(uid uuid.UUID) (token string, expiresAt time.Time, err error)
}

type Verifier interface {
	Verify(token string) (*Claims, error)
}

type HS256JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewHS256JWT(secret []byte, ttl time.Duration) *HS256JWT {
	return &HS256JWT{secret: secret, ttl: ttl}
}

func (j *HS256JWT) Sign(uid uuid.UUID) (string, time.Time, error) {
	expiresAt := time.Now().Add(j.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   uid.String(),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, expiresAt, nil
}

func (j *HS256JWT) Verify(tokenStr string) (*Claims, error) {
	var claims jwt.RegisteredClaims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return j.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return &Claims{
		UID:       uid,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
