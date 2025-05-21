package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// userIDKey is the key used to store the userID in the request context.
const userIDKey contextKey = "userID"

// authMiddleware protects routes that require authentication.
// It validates the JWT access token from the Authorization header.
// If valid, it adds the userID to the request context.
func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid Authorization header format (expected Bearer token)", http.StatusUnauthorized)
			return
		}
		tokenString := parts[1]

		// ValidateToken is defined in auth.go
		// jwtAccessSecret is a global variable in auth.go, initialized from env
		claims, err := ValidateToken(tokenString, jwtAccessSecret)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Extract userID from claims (Subject field)
		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
			return
		}

		// Store userID in request context
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getUserIDFromContext retrieves the userID from the request context.
// Returns the userID and true if found, otherwise 0 and false.
func getUserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDKey).(int64)
	return userID, ok
}
