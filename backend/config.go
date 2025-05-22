package main

import (
	"log"
	"os"
)

// AppConfig holds application configuration values.
type AppConfig struct {
	DatabaseURL         string
	JwtAccessSecret     string
	JwtRefreshSecret    string
	GeminiAPIKey        string
	FileStorageBasePath string // Added
	FileStorageBaseURL  string // Added
}

// LoadConfig loads application configuration from environment variables.
func LoadConfig() AppConfig {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("Warning: DATABASE_URL environment variable is not set in LoadConfig.")
	}

	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	if accessSecret == "" {
		log.Println("Warning: JWT_ACCESS_SECRET not set, using default in auth.go if not overridden there.")
	}

	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if refreshSecret == "" {
		log.Println("Warning: JWT_REFRESH_SECRET not set, using default in auth.go if not overridden there.")
	}
	
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Println("Warning: GEMINI_API_KEY environment variable is not set.")
	}

	// File Storage Config
	fileStorageBasePath := os.Getenv("FILESTORAGE_BASE_PATH")
	if fileStorageBasePath == "" {
		fileStorageBasePath = "./uploads" // Default relative to app root for dev
		log.Printf("Warning: FILESTORAGE_BASE_PATH not set, defaulting to %s", fileStorageBasePath)
	}

	fileStorageBaseURL := os.Getenv("FILESTORAGE_BASE_URL")
	if fileStorageBaseURL == "" {
		fileStorageBaseURL = "/uploads" // Default
		log.Printf("Warning: FILESTORAGE_BASE_URL not set, defaulting to %s", fileStorageBaseURL)
	}

	return AppConfig{
		DatabaseURL:         dbURL,
		JwtAccessSecret:     accessSecret,
		JwtRefreshSecret:    refreshSecret,
		GeminiAPIKey:        geminiKey,
		FileStorageBasePath: fileStorageBasePath,
		FileStorageBaseURL:  fileStorageBaseURL,
	}
}
