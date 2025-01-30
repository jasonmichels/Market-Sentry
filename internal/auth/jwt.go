package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var secretKey = []byte("supersecretkey") // replace with your real secret, from config or env

func GenerateJWT(phone string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"phone": phone,
		"exp":   time.Now().Add(8 * time.Hour).Unix(),
	})

	return token.SignedString(secretKey)
}

func ParseJWT(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secretKey, nil
	})

	if err != nil {
		return "", err
	}

	// Extract claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		phone, ok := claims["phone"].(string)
		if !ok {
			return "", errors.New("phone not found in token claims")
		}
		return phone, nil
	}
	return "", errors.New("invalid token claims")
}
