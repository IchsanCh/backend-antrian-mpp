package config

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID    int64  `json:"user_id"`
	Nama      string `json:"nama"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	UnitID    *int64 `json:"unit_id,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64, nama, email, role string, unitID *int64) (string, error) {
	claims := JWTClaims{
		UserID:    userID,
		Nama:      nama,
		Email:     email,
		Role:      role,
		UnitID:    unitID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}