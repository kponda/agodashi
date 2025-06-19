package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestConfig holds test configuration
type TestConfig struct {
	DatabaseURL string
	App         *App
	Server      *httptest.Server
}

// IntegrationTestSuite holds all integration tests
type IntegrationTestSuite struct {
	config *TestConfig
	t      *testing.T
}

// setupTestEnvironment initializes the test environment
func setupTestEnvironment(t *testing.T) *TestConfig {
	// Use test database URL or fallback to dev
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost:5432/appdb?sslmode=disable"
	}

	// Set test environment variables
	os.Setenv("DATABASE_URL", dbURL)
	os.Setenv("JWT_ACCESS_SECRET", "test_jwt_access_secret_key_min_32_bytes")
	os.Setenv("JWT_REFRESH_SECRET", "test_jwt_refresh_secret_key_min_32_bytes")
	os.Setenv("GEMINI_API_KEY", "test_gemini_key")
	os.Setenv("FILESTORAGE_BASE_PATH", "./test_uploads")
	os.Setenv("FILESTORAGE_BASE_URL", "/test_uploads")

	// Connect to test database
	ctx := context.Background()
	dbpool, err := connectDB(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping integration tests: Failed to connect to test database: %v\nPlease start PostgreSQL with: docker compose up -d postgres", err)
		return nil
	}

	// Load test config
	appConfig := LoadConfig()

	// Create test app
	app := &App{
		DB:     dbpool,
		Config: appConfig,
	}

	// Initialize services (optional for basic tests)
	if appConfig.GeminiAPIKey != "" && appConfig.GeminiAPIKey != "test_gemini_key" {
		translationSvc, err := NewGeminiTranslationService(appConfig.GeminiAPIKey)
		if err == nil {
			app.TranslationService = translationSvc
		}
	}

	// Clean up any existing test data
	app.DB.Exec(context.Background(), "DELETE FROM articles WHERE slug LIKE 'test-%'")
	app.DB.Exec(context.Background(), "DELETE FROM users WHERE username = 'testuser'")

	// Create test server
	mux := createTestMux(app)
	server := httptest.NewServer(mux)

	return &TestConfig{
		DatabaseURL: dbURL,
		App:         app,
		Server:      server,
	}
}

// createTestMux creates the same mux as main but for testing
func createTestMux(app *App) http.Handler {
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("/api/", app.rootHandler)
	mux.HandleFunc("/api/articles", app.handleArticles)
	mux.HandleFunc("/api/articles/", app.handleArticleBySlug)
	mux.HandleFunc("/api/auth/register", app.registerHandler)
	mux.HandleFunc("/api/auth/login", app.loginHandler)
	mux.HandleFunc("/api/auth/refresh", app.refreshTokenHandler)
	mux.Handle("/api/images/upload", app.authMiddleware(http.HandlerFunc(app.uploadImageHandler)))

	// Wrap with CORS middleware like in main
	return corsMiddleware(mux)
}

// teardownTestEnvironment cleans up test resources
func (tc *TestConfig) teardownTestEnvironment() {
	if tc.Server != nil {
		tc.Server.Close()
	}
	if tc.App != nil && tc.App.DB != nil {
		tc.App.DB.Close()
	}
	// Clean up test uploads directory
	os.RemoveAll("./test_uploads")
}

// Test helper functions
func (suite *IntegrationTestSuite) makeRequest(method, path string, body interface{}, headers map[string]string) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, suite.config.Server.URL+path, reqBody)
	if err != nil {
		return nil, nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}

	return resp, respBody, nil
}

// Test: API Root Endpoint
func (suite *IntegrationTestSuite) testAPIRootEndpoint() {
	suite.t.Run("API Root Endpoint", func(t *testing.T) {
		resp, body, err := suite.makeRequest("GET", "/api/", nil, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		if !strings.Contains(string(body), "Hello from Backend API") {
			t.Errorf("Expected root message in response, got: %s", string(body))
		}
	})
}

// Test: User Registration
func (suite *IntegrationTestSuite) testUserRegistration() {
	suite.t.Run("User Registration", func(t *testing.T) {
		// Test successful registration
		registerReq := RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "testpassword123",
		}

		resp, body, err := suite.makeRequest("POST", "/api/auth/register", registerReq, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status %d, got %d. Response: %s", http.StatusCreated, resp.StatusCode, string(body))
		}

		var userResp UserResponse
		if err := json.Unmarshal(body, &userResp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if userResp.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", userResp.Username)
		}
	})

	suite.t.Run("User Registration - Duplicate Username", func(t *testing.T) {
		// Test duplicate username
		registerReq := RegisterRequest{
			Username: "testuser", // Same as previous test
			Email:    "test2@example.com",
			Password: "testpassword123",
		}

		resp, _, err := suite.makeRequest("POST", "/api/auth/register", registerReq, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected status %d for duplicate username, got %d", http.StatusConflict, resp.StatusCode)
		}
	})

	suite.t.Run("User Registration - Invalid Input", func(t *testing.T) {
		// Test invalid input
		registerReq := RegisterRequest{
			Username: "ab", // Too short
			Password: "123",   // Too short
		}

		resp, _, err := suite.makeRequest("POST", "/api/auth/register", registerReq, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status %d for invalid input, got %d", http.StatusBadRequest, resp.StatusCode)
		}
	})
}

// Test: User Login
func (suite *IntegrationTestSuite) testUserLogin() string {
	var accessToken string

	suite.t.Run("User Login", func(t *testing.T) {
		loginReq := LoginRequest{
			Username: "testuser",
			Password: "testpassword123",
		}

		resp, body, err := suite.makeRequest("POST", "/api/auth/login", loginReq, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d. Response: %s", http.StatusOK, resp.StatusCode, string(body))
		}

		var loginResp LoginResponse
		if err := json.Unmarshal(body, &loginResp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if loginResp.AccessToken == "" {
			t.Error("Expected access token in response")
		}

		if loginResp.User.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", loginResp.User.Username)
		}

		accessToken = loginResp.AccessToken

		// Check for refresh token cookie
		cookies := resp.Cookies()
		found := false
		for _, cookie := range cookies {
			if cookie.Name == "refresh_token" {
				found = true
				if cookie.Value == "" {
					t.Error("Refresh token cookie is empty")
				}
				if !cookie.HttpOnly {
					t.Error("Refresh token cookie should be HttpOnly")
				}
				break
			}
		}
		if !found {
			t.Error("Expected refresh token cookie in response")
		}
	})

	suite.t.Run("User Login - Invalid Credentials", func(t *testing.T) {
		loginReq := LoginRequest{
			Username: "testuser",
			Password: "wrongpassword",
		}

		resp, _, err := suite.makeRequest("POST", "/api/auth/login", loginReq, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d for invalid credentials, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	return accessToken
}

// Test: Article CRUD Operations
func (suite *IntegrationTestSuite) testArticleCRUD(accessToken string) {
	suite.t.Run("Create Article", func(t *testing.T) {
		createReq := CreateArticleRequest{
			Slug: "test-article",
			Translations: []CreateArticleTranslation{
				{
					LanguageCode: "en",
					Title:        "Test Article",
					Content:      "This is a test article content.",
					IsPublished:  true,
				},
			},
		}

		headers := map[string]string{
			"Authorization": "Bearer " + accessToken,
		}

		resp, body, err := suite.makeRequest("POST", "/api/articles", createReq, headers)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status %d, got %d. Response: %s", http.StatusCreated, resp.StatusCode, string(body))
		}

		var articleResp ArticleResponse
		if err := json.Unmarshal(body, &articleResp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if articleResp.Slug != "test-article" {
			t.Errorf("Expected slug 'test-article', got '%s'", articleResp.Slug)
		}
	})

	suite.t.Run("Create Article - Unauthorized", func(t *testing.T) {
		createReq := CreateArticleRequest{
			Slug: "unauthorized-article",
			Translations: []CreateArticleTranslation{
				{
					LanguageCode: "en",
					Title:        "Unauthorized Article",
					Content:      "This should not be created.",
					IsPublished:  false,
				},
			},
		}

		resp, _, err := suite.makeRequest("POST", "/api/articles", createReq, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d for unauthorized request, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	suite.t.Run("List Articles", func(t *testing.T) {
		resp, body, err := suite.makeRequest("GET", "/api/articles", nil, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		var articles []ArticleResponse
		if err := json.Unmarshal(body, &articles); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if len(articles) == 0 {
			t.Error("Expected at least one article in response")
		}
	})

	suite.t.Run("Get Article by Slug", func(t *testing.T) {
		resp, body, err := suite.makeRequest("GET", "/api/articles/test-article", nil, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		var articleResp ArticleResponse
		if err := json.Unmarshal(body, &articleResp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if articleResp.Slug != "test-article" {
			t.Errorf("Expected slug 'test-article', got '%s'", articleResp.Slug)
		}
	})

	suite.t.Run("Get Article - Not Found", func(t *testing.T) {
		resp, _, err := suite.makeRequest("GET", "/api/articles/non-existent", nil, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status %d for non-existent article, got %d", http.StatusNotFound, resp.StatusCode)
		}
	})

	suite.t.Run("Update Article", func(t *testing.T) {
		updateReq := CreateArticleRequest{
			Slug: "test-article-updated",
			Translations: []CreateArticleTranslation{
				{
					LanguageCode: "en",
					Title:        "Updated Test Article",
					Content:      "This is updated content.",
					IsPublished:  true,
				},
			},
		}

		headers := map[string]string{
			"Authorization": "Bearer " + accessToken,
		}

		resp, body, err := suite.makeRequest("PUT", "/api/articles/test-article", updateReq, headers)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d. Response: %s", http.StatusOK, resp.StatusCode, string(body))
		}
	})

	suite.t.Run("Delete Article", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "Bearer " + accessToken,
		}

		resp, _, err := suite.makeRequest("DELETE", "/api/articles/test-article-updated", nil, headers)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
		}
	})
}

// Test: Authentication Flow
func (suite *IntegrationTestSuite) testAuthenticationFlow() {
	suite.t.Run("Authentication Middleware", func(t *testing.T) {
		// Test accessing protected endpoint without token
		resp, _, err := suite.makeRequest("POST", "/api/articles", nil, nil)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d for missing token, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	suite.t.Run("Invalid Bearer Token", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "Bearer invalid_token",
		}

		resp, _, err := suite.makeRequest("POST", "/api/articles", nil, headers)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d for invalid token, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})

	suite.t.Run("Invalid Authorization Header Format", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "InvalidFormat token",
		}

		resp, _, err := suite.makeRequest("POST", "/api/articles", nil, headers)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status %d for invalid auth format, got %d", http.StatusUnauthorized, resp.StatusCode)
		}
	})
}

// Test: Error Handling
func (suite *IntegrationTestSuite) testErrorHandling() {
	suite.t.Run("Invalid JSON Request", func(t *testing.T) {
		req, err := http.NewRequest("POST", suite.config.Server.URL+"/api/auth/register", strings.NewReader("{invalid json"))
		if err != nil {
			suite.t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			suite.t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			suite.t.Errorf("Expected status %d for invalid JSON, got %d", http.StatusBadRequest, resp.StatusCode)
		}
	})

	suite.t.Run("Method Not Allowed", func(t *testing.T) {
		resp, _, err := suite.makeRequest("PUT", "/api/auth/register", nil, nil)
		if err != nil {
			suite.t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != http.StatusMethodNotAllowed {
			suite.t.Errorf("Expected status %d for wrong method, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
		}
	})
}

// Test: CORS Headers (if implemented)
func (suite *IntegrationTestSuite) testCORSConfiguration() {
	suite.t.Run("CORS Preflight", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", suite.config.Server.URL+"/api/articles", nil)
		if err != nil {
			suite.t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Origin", "http://localhost:4200")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			suite.t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Note: CORS implementation may not be present yet, so this test might fail
		// This is expected and will help identify if CORS needs to be implemented
		suite.t.Logf("CORS preflight response status: %d", resp.StatusCode)
		suite.t.Logf("CORS headers: %v", resp.Header)
	})
}

// Main test runner
func TestIntegrationSuite(t *testing.T) {
	// Setup test environment
	config := setupTestEnvironment(t)
	if config == nil {
		return // Test was skipped
	}
	defer config.teardownTestEnvironment()

	suite := &IntegrationTestSuite{
		config: config,
		t:      t,
	}

	// Run all tests in order
	suite.testAPIRootEndpoint()
	suite.testUserRegistration()
	accessToken := suite.testUserLogin()
	suite.testArticleCRUD(accessToken)
	suite.testAuthenticationFlow()
	suite.testErrorHandling()
	suite.testCORSConfiguration()

	fmt.Println("All integration tests completed!")
}

// Standalone test runner for development
func runIntegrationTests() {
	testing.Main(
		func(pat, str string) (bool, error) { return true, nil },
		[]testing.InternalTest{
			{
				Name: "TestIntegrationSuite",
				F:    TestIntegrationSuite,
			},
		},
		[]testing.InternalBenchmark{},
		[]testing.InternalExample{},
	)
}