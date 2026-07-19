package auth

import (
	"crypto/rand"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zubayermd-dev/ivy/internal/model"
)

var secretKey []byte

const secretFile = "/opt/ivy/.jwt_secret"

func init() {
	// Try to load from environment
	if envKey := os.Getenv("IVY_JWT_SECRET"); envKey != "" {
		secretKey = []byte(envKey)
		return
	}

	// Try to load from persisted file
	if data, err := os.ReadFile(secretFile); err == nil && len(data) >= 32 {
		secretKey = data
		return
	}

	// Generate new random key and persist it
	secretKey = make([]byte, 32)
	if _, err := rand.Read(secretKey); err != nil {
		panic("failed to generate JWT secret: " + err.Error())
	}
	os.WriteFile(secretFile, secretKey, 0600)
}

type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(user *model.User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey)
}

func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
