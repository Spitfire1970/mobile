package auth

import (
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
	"net/http"
	"errors"
	"strings"
	"crypto/rand"
	"encoding/hex"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Issuer: "chirpy", IssuedAt: &jwt.NumericDate{Time: now}, ExpiresAt: &jwt.NumericDate{Time: now.Add(expiresIn)}, Subject: userID.String()})
	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := jwt.MapClaims{}
	t, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	s, err := t.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(s)
}

func GetBearerToken(headers http.Header) (string, error) {
	s := headers.Get("Authorization")
	if s == "" || !strings.HasPrefix(s, "Bearer ") {
		return "", errors.New("no header")
	}
	t := s[7:]
	return t, nil
}

func MakeRefreshToken() (string, error) {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key), nil
}

func GetAPIKey(headers http.Header) (string, error) {
	s := headers.Get("Authorization")
	
	if s == "" || !strings.HasPrefix(s, "ApiKey ") {
		return "", errors.New("no header")
	}
	t := s[7:]
	return t, nil
}