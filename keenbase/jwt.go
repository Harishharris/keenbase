package keenbase

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type tokenType string

const (
	tokenTypeAuth tokenType = "auth" // regular auth record (any auth collection)
)

type authClaims struct {
	jwt.RegisteredClaims
	Type         tokenType `json:"type"`
	CollectionID string    `json:"collectionId"`

	Refreshable bool `json:"refreshable"`
}

const tokenDuration = 7 * 24 * time.Hour

func generateAuthToken(recordID, collectionID, secretKey string) (string, error) {
	if secretKey == "" {
		return "", errors.New("token secret key is empty")
	}

	now := time.Now()
	claims := authClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        newID(),
			Subject:   recordID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenDuration)),
		},
		Type:         tokenTypeAuth,
		CollectionID: collectionID,
		Refreshable:  true,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func parseAuthToken(tokenStr, secretKey string) (*authClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&authClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secretKey), nil
		},
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*authClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
