package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// RegisterRequest defines the structure for user registration.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email,omitempty"` // Optional
	Password string `json:"password"`
}

// UserResponse defines the structure for returning basic user info.
type UserResponse struct {
	ID                  int64     `json:"id"`
	Username            string    `json:"username"`
	Email               string    `json:"email,omitempty"`
	DefaultLanguageCode string    `json:"default_language_code"` // Added new field
	CreatedAt           time.Time `json:"created_at"`
}

// LoginRequest defines the structure for user login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse defines the structure for the login response.
// The refresh token is sent as an HttpOnly cookie.
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	User        UserResponse `json:"user"`
}

// RefreshTokenResponse defines the structure for the refresh token response.
type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Basic email validation regex
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// registerHandler handles user registration.
func (a *App) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate input
	if req.Username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}
	if len(req.Username) < 3 {
		http.Error(w, "Username must be at least 3 characters long", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters long", http.StatusBadRequest)
		return
	}
	if req.Email != "" && !emailRegex.MatchString(req.Email) {
		http.Error(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	// Check if username or email already exists
	var existingUserID int64
	var checkQuery string
	var args []interface{}

	if req.Email != "" {
		checkQuery = "SELECT id FROM users WHERE username = $1 OR email = $2 LIMIT 1"
		args = append(args, req.Username, req.Email)
	} else {
		checkQuery = "SELECT id FROM users WHERE username = $1 LIMIT 1"
		args = append(args, req.Username)
	}

	err := a.DB.QueryRow(context.Background(), checkQuery, args...).Scan(&existingUserID)
	if err != nil && err.Error() != "no rows in result set" { // pgx returns "no rows in result set" instead of sql.ErrNoRows
		http.Error(w, "Database error while checking existing user", http.StatusInternalServerError)
		log.Printf("Error checking existing user: %v", err)
		return
	}
	if err == nil { // No error means a user was found
		http.Error(w, "Username or email already exists", http.StatusConflict)
		return
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		log.Printf("Error hashing password: %v", err)
		return
	}

	// Store the new user
	var userID int64
	var userEmail sql.NullString // Use sql.NullString for nullable email
	if req.Email != "" {
		userEmail = sql.NullString{String: req.Email, Valid: true}
	}

	// The default_language_code will be set by the DB default ('en')
	insertQuery := "INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id, created_at, default_language_code"
	var createdAt time.Time
	var defaultLangCode string
	err = a.DB.QueryRow(context.Background(), insertQuery, req.Username, userEmail, string(hashedPassword)).Scan(&userID, &createdAt, &defaultLangCode)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		log.Printf("Error creating user: %v", err)
		return
	}

	// Return UserResponse
	userResponse := UserResponse{
		ID:                  userID,
		Username:            req.Username,
		Email:               req.Email, // Return the email if provided, even if it's empty string from request
		DefaultLanguageCode: defaultLangCode,
		CreatedAt:           createdAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(userResponse); err != nil {
		log.Printf("Error encoding user response: %v", err)
		// Client already received 201, but log this server-side issue
	}
}

// loginHandler handles user login.
func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Retrieve user by username
	var userID int64
	var storedPasswordHash string
	var userEmail sql.NullString // To fetch email
	var createdAt time.Time      // To fetch created_at
	var defaultLangCode string   // To fetch default_language_code

	queryUser := "SELECT id, password_hash, email, created_at, default_language_code FROM users WHERE username = $1"
	err := a.DB.QueryRow(context.Background(), queryUser, req.Username).Scan(&userID, &storedPasswordHash, &userEmail, &createdAt, &defaultLangCode)
	if err != nil {
		if err.Error() == "no rows in result set" { // pgx returns "no rows in result set" instead of sql.ErrNoRows
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
			log.Printf("Error retrieving user %s: %v", req.Username, err)
		}
		return
	}

	// Compare password with stored hash
	err = bcrypt.CompareHashAndPassword([]byte(storedPasswordHash), []byte(req.Password))
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized) // Password doesn't match
		return
	}

	// Generate tokens
	accessToken, err := GenerateAccessToken(userID)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		log.Printf("Error generating access token for user ID %d: %v", userID, err)
		return
	}

	refreshTokenString, err := GenerateRefreshToken(userID)
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		log.Printf("Error generating refresh token for user ID %d: %v", userID, err)
		return
	}

	// Hash and store refresh token
	refreshTokenHash := HashToken(refreshTokenString)
	// Refresh token expiry is 2 weeks, as set in GenerateRefreshToken
	// We use the same duration for the database record.
	refreshTokenExpiresAt := time.Now().Add(2 * 7 * 24 * time.Hour)

	_, err = a.DB.Exec(context.Background(),
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		userID, refreshTokenHash, refreshTokenExpiresAt)
	if err != nil {
		http.Error(w, "Failed to store refresh token", http.StatusInternalServerError)
		log.Printf("Error storing refresh token for user ID %d: %v", userID, err)
		return
	}

	// Set refresh token as HttpOnly cookie
	// Secure should be true in production (when using HTTPS)
	// For local dev over HTTP, Secure might need to be false, or use a reverse proxy that handles HTTPS.
	// For simplicity, let's assume production or HTTPS-handled dev for now.
	// The Path should be specific to where refresh token is handled, e.g., /api/auth/refresh
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshTokenString,
		Expires:  refreshTokenExpiresAt,
		HttpOnly: true,
		Secure:   true, // Set to true if your app is served over HTTPS
		Path:     "/api/auth", // Path where the refresh token is valid/used
		SameSite: http.SameSiteLaxMode,
	})

	// Prepare UserResponse
	responseUser := UserResponse{
		ID:                  userID,
		Username:            req.Username,
		DefaultLanguageCode: defaultLangCode,
		CreatedAt:           createdAt,
	}
	if userEmail.Valid {
		responseUser.Email = userEmail.String
	}

	// Return LoginResponse
	loginResponse := LoginResponse{
		AccessToken: accessToken,
		User:        responseUser,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(loginResponse); err != nil {
		log.Printf("Error encoding login response for user %s: %v", req.Username, err)
	}
}

// refreshTokenHandler handles refreshing JWT access tokens using a refresh token.
func (a *App) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Retrieve refresh token from HttpOnly cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		if err == http.ErrNoCookie {
			http.Error(w, "Refresh token not found", http.StatusUnauthorized)
		} else {
			http.Error(w, "Error reading cookie", http.StatusBadRequest) // Other error
		}
		return
	}
	refreshTokenString := cookie.Value
	if refreshTokenString == "" {
		http.Error(w, "Refresh token is empty", http.StatusUnauthorized)
		return
	}

	// 2. Hash the provided token for DB comparison
	hashedTokenFromCookie := HashToken(refreshTokenString)

	// 3. Query refresh_tokens table
	var storedUserID int64
	var storedTokenHash string
	var storedExpiresAt time.Time

	query := "SELECT user_id, token_hash, expires_at FROM refresh_tokens WHERE token_hash = $1"
	err = a.DB.QueryRow(context.Background(), query, hashedTokenFromCookie).Scan(&storedUserID, &storedTokenHash, &storedExpiresAt)
	if err != nil {
		if err.Error() == "no rows in result set" { // pgx returns "no rows in result set" instead of sql.ErrNoRows
			// Token not found in DB (potentially already used/revoked, or invalid)
			http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		} else {
			http.Error(w, "Database error validating refresh token", http.StatusInternalServerError)
			log.Printf("Error retrieving refresh token by hash: %v", err)
		}
		// It's good practice to clear the potentially invalid refresh token cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Expires:  time.Unix(0, 0), // Expire immediately
			HttpOnly: true,
			Secure:   true,
			Path:     "/api/auth",
			SameSite: http.SameSiteLaxMode,
		})
		return
	}

	// 4. Check if token is expired
	if time.Now().After(storedExpiresAt) {
		// Token is expired. Delete it from DB.
		_, delErr := a.DB.Exec(context.Background(), "DELETE FROM refresh_tokens WHERE token_hash = $1", storedTokenHash)
		if delErr != nil {
			log.Printf("Failed to delete expired refresh token %s: %v", storedTokenHash, delErr)
		}
		http.Error(w, "Refresh token expired", http.StatusUnauthorized)
		// Clear the cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   true,
			Path:     "/api/auth",
			SameSite: http.SameSiteLaxMode,
		})
		return
	}

	// At this point, the refresh token from the cookie is valid and matches a non-expired one in the DB.

	// 5. Generate a new access token
	newAccessToken, err := GenerateAccessToken(storedUserID)
	if err != nil {
		http.Error(w, "Failed to generate new access token", http.StatusInternalServerError)
		log.Printf("Error generating new access token for user ID %d: %v", storedUserID, err)
		return
	}

	// 6. Implement Refresh Token Rotation (Optional but Recommended)
	// Generate new refresh token, update DB, set new cookie.
	// This requires a transaction to ensure atomicity for DB operations.
	tx, err := a.DB.Begin(context.Background())
	if err != nil {
		http.Error(w, "Failed to start transaction for token rotation", http.StatusInternalServerError)
		log.Printf("Error starting transaction for token rotation: %v", err)
		return
	}
	defer tx.Rollback(context.Background()) // Rollback if not committed

	// Delete the old refresh token
	_, err = tx.Exec(context.Background(), "DELETE FROM refresh_tokens WHERE token_hash = $1", storedTokenHash)
	if err != nil {
		http.Error(w, "Failed to delete old refresh token during rotation", http.StatusInternalServerError)
		log.Printf("Error deleting old refresh token %s: %v", storedTokenHash, err)
		return
	}

	// Generate new refresh token
	newRefreshTokenString, err := GenerateRefreshToken(storedUserID)
	if err != nil {
		http.Error(w, "Failed to generate new refresh token during rotation", http.StatusInternalServerError)
		log.Printf("Error generating new refresh token for user ID %d: %v", storedUserID, err)
		return
	}
	newRefreshTokenHash := HashToken(newRefreshTokenString)
	newRefreshTokenExpiresAt := time.Now().Add(2 * 7 * 24 * time.Hour) // Consistent with login

	// Store the new refresh token
	_, err = tx.Exec(context.Background(),
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		storedUserID, newRefreshTokenHash, newRefreshTokenExpiresAt)
	if err != nil {
		http.Error(w, "Failed to store new refresh token during rotation", http.StatusInternalServerError)
		log.Printf("Error storing new refresh token for user ID %d: %v", storedUserID, err)
		return
	}

	// Commit transaction
	if err := tx.Commit(context.Background()); err != nil {
		http.Error(w, "Failed to commit transaction for token rotation", http.StatusInternalServerError)
		log.Printf("Error committing transaction for token rotation: %v", err)
		return
	}

	// Set the new refresh token as an HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshTokenString,
		Expires:  newRefreshTokenExpiresAt,
		HttpOnly: true,
		Secure:   true, // Set to true if your app is served over HTTPS
		Path:     "/api/auth",
		SameSite: http.SameSiteLaxMode,
	})

	// 7. Return new access token
	response := RefreshTokenResponse{
		AccessToken: newAccessToken,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding refresh token response for user ID %d: %v", storedUserID, err)
	}
}
