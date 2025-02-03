package auth

import (
	"context"
	"log"
	"net/http"
	"strings"
)

// phoneKeyType is a custom type for storing the phone in context.
type phoneKeyType string

var phoneKey phoneKeyType = "phone"

// JWTMiddleware checks the cookie (or header) for a JWT, validates it, and sets user phone in context.
func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string

		// Prefer reading from Cookie
		cookie, err := r.Cookie("marketsentry")
		if err == nil {
			log.Printf("[Auth] Found 'token' cookie")
			tokenStr = cookie.Value
		} else {
			log.Printf("[Auth] No 'token' cookie found, checking 'Authorization' header")
			// Alternatively, check Authorization header: "Bearer <token>"
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				log.Printf("[Auth] Found Bearer token in header")
			}
		}

		if tokenStr == "" {
			log.Printf("[Auth] No token found; user is unauthorized")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Attempt to parse/validate JWT
		phone, err := ParseJWT(tokenStr)
		if err != nil {
			log.Printf("[Auth] Invalid token: %v", err)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		log.Printf("[Auth] Token validated. Phone: %s", phone)

		// Store phone in context
		ctx := context.WithValue(r.Context(), phoneKey, phone)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserPhone extracts the phone from context
func GetUserPhone(ctx context.Context) string {
	if v, ok := ctx.Value(phoneKey).(string); ok {
		return v
	}
	return ""
}
