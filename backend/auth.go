package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtAccessSecret  []byte
	jwtRefreshSecret []byte
)

func init() {
	// Load secrets from environment variables
	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	if accessSecret == "" {
		// For development, use a default. In production, this should cause a fatal error or be handled.
		fmt.Println("Warning: JWT_ACCESS_SECRET not set, using default insecure key.")
		accessSecret = "default_insecure_access_secret_key_for_dev_only_min_32_chars"
	}
	jwtAccessSecret = []byte(accessSecret)

	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if refreshSecret == "" {
		fmt.Println("Warning: JWT_REFRESH_SECRET not set, using default insecure key.")
		refreshSecret = "default_insecure_refresh_secret_key_for_dev_only_min_32_chars"
	}
	jwtRefreshSecret = []byte(refreshSecret)

    // Basic check for key length (HS256 recommends at least 256 bits / 32 bytes)
    if len(jwtAccessSecret) < 32 {
        fmt.Println("Warning: JWT_ACCESS_SECRET is less than 32 bytes. This is insecure.")
    }
    if len(jwtRefreshSecret) < 32 {
         fmt.Println("Warning: JWT_REFRESH_SECRET is less than 32 bytes. This is insecure.")
    }
}

// GenerateAccessToken creates a new JWT access token for a given user ID.
func GenerateAccessToken(userID int64) (string, error) {
	claims := &jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", userID),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    "my-app", // Optional: identify the issuer
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtAccessSecret)
}

// GenerateRefreshToken creates a new JWT refresh token for a given user ID.
func GenerateRefreshToken(userID int64) (string, error) {
	claims := &jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", userID),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * 7 * 24 * time.Hour)), // 2 weeks
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    "my-app/refresh", // Optional: differentiate refresh token issuer
	}
	// Note: For refresh tokens, some prefer opaque tokens stored in DB vs. self-contained JWTs.
	// Here, we generate a JWT as per instruction.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtRefreshSecret)
}

// ValidateToken parses and validates a JWT token string using the provided secret key.
// It returns the registered claims if the token is valid.
func ValidateToken(tokenString string, secretKey []byte) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Check the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token or claims type assertion failed")
}

// HashToken generates a SHA256 hash of a token string.
func HashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token)) // Should not error for sha256.New().Write
	return hex.EncodeToString(hasher.Sum(nil))
}

// CompareTokenWithHash compares a plain text token with its SHA256 hash.
// This is primarily for comparing a new refresh token with a stored hash.
// Note: This is direct comparison, not constant-time. For passwords, use bcrypt.
// For refresh tokens, this might be acceptable depending on risk assessment.
func CompareTokenWithHash(token string, hash string) bool {
	return HashToken(token) == hash
}
