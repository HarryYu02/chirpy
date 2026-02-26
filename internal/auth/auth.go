package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hashed, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return hashed, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return match, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	nowDate := jwt.NewNumericDate(time.Now())
	expiresDate := jwt.NewNumericDate(nowDate.Add(expiresIn))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  nowDate,
		ExpiresAt: expiresDate,
		Subject:   userID.String(),
	})
	signed, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return signed, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	userID, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, err
	}
	return userUUID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	token := headers.Get("Authorization")
	if len(token) == 0 {
		return "", fmt.Errorf("Authorization header not found")
	}
	if len(token) < 8 || token[:7] != "Bearer " {
		return "", fmt.Errorf("Authorization header invalid")
	}
	return token[7:], nil
}

func MakeRefreshToken() string {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)
}
