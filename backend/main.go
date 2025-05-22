package main

import (
	"context"
	"database/sql" // For sql.NullInt64, sql.NullTime
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// App holds application-wide dependencies, like the database pool, config, translation service, and file storage service.
type App struct {
	DB                 *pgxpool.Pool
	Config             AppConfig
	TranslationService *GeminiTranslationService
	FileStorage        filestorage.FileStorageService // Added FileStorageService
}

// rootHandler is a simple handler to check if the server is running under /api.
func (a *App) rootHandler(w http.ResponseWriter, r *http.Request) {
	// If path is exactly /api/ show status, otherwise 404 for other /api/ paths not explicitly handled.
	if r.URL.Path == "/api/" {
		fmt.Fprintf(w, "Hello from Backend API! Database connected: %t", a.DB != nil)
		return
	}
	http.NotFound(w, r)
}

func main() {
	// It's good practice to use a context for application lifecycle
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	// connectDB is defined in db.go and now accepts a context
	dbpool, err := connectDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbpool.Close() // Ensure pool is closed on application exit
	log.Println("Successfully connected to the database.")

	// Load application configuration
	appConfig := LoadConfig() // LoadConfig is defined in config.go
	// DATABASE_URL is already checked above and will be used for connectDB
	// JWT secrets are loaded in auth.go init()
	// Gemini API Key is loaded by LoadConfig() and logged if missing.

	app := &App{
		DB:     dbpool,
		Config: appConfig,
		// TranslationService will be initialized next
	}

	// Initialize Gemini Translation Service
	if appConfig.GeminiAPIKey != "" {
		translationSvc, err := NewGeminiTranslationService(appConfig.GeminiAPIKey)
		if err != nil {
			// Log the error but don't necessarily make it fatal,
			// as the app might still function for non-translation tasks.
			log.Printf("Warning: Failed to initialize GeminiTranslationService: %v", err)
			// app.TranslationService will remain nil
		} else {
			log.Println("GeminiTranslationService initialized successfully.")
			app.TranslationService = translationSvc
			// defer translationSvc.Close() // If a Close method is implemented and needed
		}
	} else {
		log.Println("Warning: GeminiAPIKey not set. GeminiTranslationService will not be available.")
	}

	// Initialize File Storage Service
	if appConfig.FileStorageBasePath != "" && appConfig.FileStorageBaseURL != "" {
		fileStorageSvc, err := filestorage.NewLocalFileStorage(appConfig.FileStorageBasePath, appConfig.FileStorageBaseURL)
		if err != nil {
			// Depending on requirements, this could be a fatal error.
			// For now, log as warning, service will be nil.
			log.Printf("Warning: Failed to initialize LocalFileStorage: %v", err)
		} else {
			log.Println("LocalFileStorage initialized successfully.")
			app.FileStorage = fileStorageSvc
		}
	} else {
		log.Println("Warning: FileStorageBasePath or FileStorageBaseURL not set. LocalFileStorage will not be available.")
	}

	mux := http.NewServeMux()

	// Register handlers using app instance with /api prefix
	mux.HandleFunc("/api/", app.rootHandler) // Health check for /api/
	
	// Article routes - using new dispatcher functions
	mux.HandleFunc("/api/articles", app.handleArticles)
	mux.HandleFunc("/api/articles/", app.handleArticleBySlug)

	// Auth routes
	mux.HandleFunc("/api/auth/register", app.registerHandler)     // registerHandler is in auth_handlers.go
	mux.HandleFunc("/api/auth/login", app.loginHandler)           // loginHandler is in auth_handlers.go
	mux.HandleFunc("/api/auth/refresh", app.refreshTokenHandler) // refreshTokenHandler is in auth_handlers.go

	// Image Upload Route (Protected)
	// Note: Using Handle for routes with middleware that isn't a simple HandleFunc
	mux.Handle("/api/images/upload", app.authMiddleware(http.HandlerFunc(app.uploadImageHandler)))

	// Static File Server for Uploaded Images
	// Ensure BaseURL has a leading slash and no trailing slash for StripPrefix consistency
	// cfg.FileStorageBaseURL is like "/uploads"
	// cfg.FileStorageBasePath is like "/app/uploads" (inside Docker) or "./uploads" (local dev)
	if app.Config.FileStorageBaseURL != "" && app.Config.FileStorageBasePath != "" {
		urlPath := strings.TrimSuffix(app.Config.FileStorageBaseURL, "/")
		if !strings.HasPrefix(urlPath, "/") {
			urlPath = "/" + urlPath
		}
		
		// The path for mux.Handle needs a trailing slash to correctly match directory prefixes
		servePath := urlPath + "/" 
		
		fs := http.FileServer(http.Dir(app.Config.FileStorageBasePath))
		mux.Handle(servePath, http.StripPrefix(urlPath, fs)) // Use urlPath for StripPrefix (without trailing slash)
		
		log.Printf("Serving static files from %s at %s", app.Config.FileStorageBasePath, servePath)
	} else {
		log.Println("Warning: FileStorageBaseURL or FileStorageBasePath not configured. Static file server for uploads is disabled.")
	}


	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
		// Basic timeouts
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Println("Backend server starting on port 8080 with /api prefix for routes...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	log.Println("Shutting down server gracefully...")

	// Give outstanding requests a deadline to complete
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server gracefully stopped.")
}

// handleArticles routes requests for /api/articles based on HTTP method
// GET is public, POST is protected by authMiddleware.
func (a *App) handleArticles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listArticlesHandler(w, r) // Public
	case http.MethodPost:
		// Apply authMiddleware only for POST requests to this path
		a.authMiddleware(http.HandlerFunc(a.createArticleHandler)).ServeHTTP(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleArticleBySlug routes requests for /api/articles/{slug} based on HTTP method
// GET is public, PUT and DELETE (article) are protected by authMiddleware.
// DELETE /api/articles/{slug}/translations/{lang} is also handled here and protected.
func (a *App) handleArticleBySlug(w http.ResponseWriter, r *http.Request) {
	pathSuffix := strings.TrimPrefix(r.URL.Path, "/api/articles/")
	if pathSuffix == "" {
		http.Error(w, "Article slug or further path not provided", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(pathSuffix, "/", 3) // Split into at most 3 parts: slug, "translations", lang
	articleSlug := parts[0]

	if len(parts) == 1 { // Path is /api/articles/{slug}
		switch r.Method {
		case http.MethodGet:
			a.getArticleHandler(w, r, articleSlug) // Public
		case http.MethodPut:
			protectedUpdateHandler := http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
				a.updateArticleHandler(wr, req, articleSlug)
			})
			a.authMiddleware(protectedUpdateHandler).ServeHTTP(w, r)
		case http.MethodDelete:
			protectedDeleteHandler := http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
				a.deleteArticleHandler(wr, req, articleSlug)
			})
			a.authMiddleware(protectedDeleteHandler).ServeHTTP(w, r)
		default:
			http.Error(w, "Method Not Allowed for /api/articles/{slug}", http.StatusMethodNotAllowed)
		}
		return
	}

	// Path is potentially /api/articles/{slug}/translations/{lang}
	if len(parts) == 3 && parts[1] == "translations" {
		langCode := parts[2]
		if langCode == "" {
			http.Error(w, "Language code not provided for translation operation", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodDelete:
			// This is the new DELETE /api/articles/{slug}/translations/{lang} endpoint
			protectedDeleteTranslationHandler := http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
				a.deleteArticleTranslationHandler(wr, req, articleSlug, langCode)
			})
			a.authMiddleware(protectedDeleteTranslationHandler).ServeHTTP(w, r)
		// Add other methods like GET, PUT for specific translations here if needed in the future
		default:
			http.Error(w, "Method Not Allowed for /api/articles/{slug}/translations/{lang}", http.StatusMethodNotAllowed)
		}
		return
	}

	// If path structure is not recognized (e.g., /api/articles/{slug}/somethingelse)
	http.Error(w, "Not Found", http.StatusNotFound)
}

func (a *App) createArticleHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Retrieve userID from context (set by authMiddleware)
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		// This should ideally not happen if middleware is correctly applied
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		log.Println("Error: User ID not found in context for createArticleHandler")
		return
	}

	tx, err := a.DB.Begin(r.Context()) // Use request context for transaction
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback(r.Context())

	var articleID int64
	// AuthorID is now taken from the authenticated user's ID from the context
	err = tx.QueryRow(r.Context(),
		"INSERT INTO articles (slug, author_id) VALUES ($1, $2) RETURNING id",
		req.Slug, userID).Scan(&articleID)
	if err != nil {
		http.Error(w, "Failed to insert article", http.StatusInternalServerError)
		log.Printf("Error inserting article (slug: %s, author_id: %d): %v", req.Slug, userID, err)
		return
	}

	for _, trans := range req.Translations {
		var publishedAt *time.Time
		if trans.IsPublished {
			now := time.Now().UTC()
			publishedAt = &now
		}
		_, err = tx.Exec(r.Context(),
			"INSERT INTO article_translations (article_id, language_code, title, content, is_published, published_at) VALUES ($1, $2, $3, $4, $5, $6)",
			articleID, trans.LanguageCode, trans.Title, trans.Content, trans.IsPublished, publishedAt)
		if err != nil {
			http.Error(w, "Failed to insert article translation", http.StatusInternalServerError)
			log.Printf("Error inserting translation: %v", err)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		log.Printf("Error committing transaction: %v", err)
		return
	}

	articleResponse, err := a.fetchArticleByID(r.Context(), articleID)
	if err != nil {
		log.Printf("Error fetching created article: %v. Article ID: %d", err, articleID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Article created successfully", "article_id": articleID})
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(articleResponse)
}

func (a *App) listArticlesHandler(w http.ResponseWriter, r *http.Request) {
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
    if limit <= 0 {
        limit = 10 
    }
    if offset < 0 {
        offset = 0 
    }
    defaultLang := r.URL.Query().Get("lang")
    // If lang is not specified, we fetch articles and their translations in *all* available languages.
    // The current fetchArticleWithFilter/fetchArticleBySlugWithTranslations with empty lang param already does this for single articles.
    // For listing, the query needs to be adapted or client needs to specify a preferred language.
    // For simplicity, this list handler will only join translations for a *specific requested language* or a default one.
    // If no lang is given, we can default to 'en' or not join translations at all for the list view.
    // Let's adjust to fetch specific language, or no translations if lang is omitted.
    // Or, as per previous code, it defaults to 'en'. Let's stick to that for now.
     if defaultLang == "" {
        defaultLang = "en" 
    }


    query := `
        SELECT 
            a.id, a.slug, a.author_id, a.created_at, a.updated_at,
            COALESCE(t.id, 0) as trans_id, -- Ensure trans_id is not null for scanning
            COALESCE(t.language_code, '') as lang_code, 
            COALESCE(t.title, '') as title, 
            COALESCE(t.content, '') as content,
            COALESCE(t.is_published, false) as is_published,
            t.published_at as trans_published_at,
            COALESCE(t.created_at, '0001-01-01T00:00:00Z') as trans_created_at, -- Ensure not null
            COALESCE(t.updated_at, '0001-01-01T00:00:00Z') as trans_updated_at  -- Ensure not null
        FROM articles a
        LEFT JOIN article_translations t ON a.id = t.article_id AND t.language_code = $1
        ORDER BY a.created_at DESC
        LIMIT $2 OFFSET $3
    `
    rows, err := a.DB.Query(r.Context(), query, defaultLang, limit, offset)
    if err != nil {
        http.Error(w, "Failed to list articles", http.StatusInternalServerError)
        log.Printf("Error listing articles: %v", err)
        return
    }
    defer rows.Close()

    var articles []ArticleResponse
    for rows.Next() {
        var art Article
        var trans ArticleTranslation
        var authorID sql.NullInt64 
        var transPublishedAt sql.NullTime

        err := rows.Scan(
            &art.ID, &art.Slug, &authorID, &art.CreatedAt, &art.UpdatedAt,
            &trans.ID, &trans.LanguageCode, &trans.Title, &trans.Content, &trans.IsPublished,
            &transPublishedAt, &trans.CreatedAt, &trans.UpdatedAt,
        )
        if err != nil {
            http.Error(w, "Failed to scan article data", http.StatusInternalServerError)
            log.Printf("Error scanning article: %v", err)
            return
        }
        
        if authorID.Valid {
            art.AuthorID = &authorID.Int64
        }

        var translationsInResponse []ArticleTranslation
        if trans.LanguageCode != "" { // A translation matching defaultLang was found
            trans.ArticleID = art.ID 
            if transPublishedAt.Valid {
                trans.PublishedAt = &transPublishedAt.Time
            }
            translationsInResponse = append(translationsInResponse, trans)
        }

        articles = append(articles, ArticleResponse{
            ID:           art.ID,
            Slug:         art.Slug,
            AuthorID:     art.AuthorID,
            CreatedAt:    art.CreatedAt,
            UpdatedAt:    art.UpdatedAt,
            Translations: translationsInResponse,
        })
    }
    if rows.Err() != nil {
        http.Error(w, "Error iterating over articles", http.StatusInternalServerError)
        log.Printf("Error after iterating articles: %v", rows.Err())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(articles)
}


func (a *App) getArticleHandler(w http.ResponseWriter, r *http.Request, slug string) {
	lang := r.URL.Query().Get("lang") 

	articleResponse, err := a.fetchArticleBySlugWithTranslations(r.Context(), slug, lang)
	if err != nil {
		if err.Error() == "not found" { 
			http.Error(w, "Article not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get article", http.StatusInternalServerError)
		}
		log.Printf("Error getting article by slug %s: %v", slug, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(articleResponse)
}

func (a *App) updateArticleHandler(w http.ResponseWriter, r *http.Request, slug string) {
    var req CreateArticleRequest 
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    authUserID, ok := getUserIDFromContext(r.Context())
    if !ok {
        http.Error(w, "User ID not found in context", http.StatusInternalServerError)
        log.Println("Error: User ID not found in context for updateArticleHandler")
        return
    }

    tx, err := a.DB.Begin(r.Context())
    if err != nil {
        http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
        log.Printf("Error starting transaction: %v", err)
        return
    }
    defer tx.Rollback(r.Context())

    // Authorization Check: Fetch current author_id and compare with authUserID
    var articleID int64
    var currentAuthorID sql.NullInt64
    err = tx.QueryRow(r.Context(), "SELECT id, author_id FROM articles WHERE slug = $1", slug).Scan(&articleID, &currentAuthorID)
    if err != nil {
        if err.Error() == "no rows in result set" { 
            http.Error(w, "Article not found", http.StatusNotFound)
        } else {
            http.Error(w, "Failed to find article", http.StatusInternalServerError)
        }
        log.Printf("Error finding article by slug %s for update: %v", slug, err)
        return
    }

    if !currentAuthorID.Valid || currentAuthorID.Int64 != authUserID {
        http.Error(w, "Forbidden: You are not authorized to update this article", http.StatusForbidden)
        return
    }
    
    newSlug := slug
    // AuthorID is not updated from the request. It's set at creation and protected.
    if req.Slug != "" && req.Slug != slug { // If slug in request is different, update it
        newSlug = req.Slug
         _, err = tx.Exec(r.Context(), "UPDATE articles SET slug = $1, updated_at = $2 WHERE id = $3",
            newSlug, time.Now().UTC(), articleID)
    } else { // Slug is not changing, only update other fields
         _, err = tx.Exec(r.Context(), "UPDATE articles SET updated_at = $1 WHERE id = $2",
            time.Now().UTC(), articleID)
    }
    if err != nil {
        http.Error(w, "Failed to update article details", http.StatusInternalServerError)
        log.Printf("Error updating article details for ID %d: %v", articleID, err)
        return
    }
    
    _, err = tx.Exec(r.Context(), "DELETE FROM article_translations WHERE article_id = $1", articleID)
    if err != nil {
        http.Error(w, "Failed to clear existing translations", http.StatusInternalServerError)
        log.Printf("Error deleting existing translations for article ID %d: %v", articleID, err)
        return
    }

    for _, trans := range req.Translations {
        var publishedAt *time.Time
        if trans.IsPublished {
            now := time.Now().UTC()
            publishedAt = &now
        }
        _, err = tx.Exec(r.Context(),
            "INSERT INTO article_translations (article_id, language_code, title, content, is_published, published_at) VALUES ($1, $2, $3, $4, $5, $6)",
            articleID, trans.LanguageCode, trans.Title, trans.Content, trans.IsPublished, publishedAt)
        if err != nil {
            http.Error(w, "Failed to insert article translation during update", http.StatusInternalServerError)
            log.Printf("Error inserting translation for article ID %d: %v", articleID, err)
            return
        }
    }

    if err := tx.Commit(r.Context()); err != nil {
        http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
        log.Printf("Error committing transaction for article ID %d: %v", articleID, err)
        return
    }

    articleResponse, err := a.fetchArticleBySlugWithTranslations(r.Context(), newSlug, "") 
    if err != nil {
        log.Printf("Error fetching updated article %s: %v. Update was successful.", newSlug, err)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"message": "Article updated successfully"})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(articleResponse)
}


func (a *App) deleteArticleTranslationHandler(w http.ResponseWriter, r *http.Request, articleSlug string, langCode string) {
	authUserID, ok := getUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		log.Println("Error: User ID not found in context for deleteArticleTranslationHandler")
		return
	}

	// Start a transaction
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		log.Printf("Error starting transaction for delete translation: %v", err)
		return
	}
	defer tx.Rollback(r.Context())

	// 1. Fetch article ID and author ID by slug
	var articleID int64
	var authorID sql.NullInt64 // author_id can be NULL
	err = tx.QueryRow(r.Context(), "SELECT id, author_id FROM articles WHERE slug = $1", articleSlug).Scan(&articleID, &authorID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Article not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error fetching article", http.StatusInternalServerError)
			log.Printf("Error fetching article by slug %s: %v", articleSlug, err)
		}
		return
	}

	// 2. Authorization: Check if the authenticated user is the author
	if !authorID.Valid || authorID.Int64 != authUserID {
		http.Error(w, "Forbidden: You are not authorized to modify this article's translations", http.StatusForbidden)
		return
	}

	// 3. Check if the translation to be deleted is the only one for the article
	var translationCount int
	err = tx.QueryRow(r.Context(), "SELECT COUNT(*) FROM article_translations WHERE article_id = $1", articleID).Scan(&translationCount)
	if err != nil {
		http.Error(w, "Database error counting translations", http.StatusInternalServerError)
		log.Printf("Error counting translations for article ID %d: %v", articleID, err)
		return
	}

	if translationCount <= 1 {
		// Check if the specific translation actually exists before denying deletion of the "last one"
		var specificTranslationExists int
		err = tx.QueryRow(r.Context(), "SELECT COUNT(*) FROM article_translations WHERE article_id = $1 AND language_code = $2", articleID, langCode).Scan(&specificTranslationExists)
		if err != nil {
			http.Error(w, "Database error checking specific translation existence", http.StatusInternalServerError)
			log.Printf("Error checking specific translation (article ID %d, lang %s): %v", articleID, langCode, err)
			return
		}
		if specificTranslationExists > 0 && translationCount <= 1 {
			http.Error(w, "Cannot delete the last translation of an article. Consider deleting the entire article or adding another translation first.", http.StatusBadRequest)
			return
		}
	}
	
	// 4. Delete the specific translation
	result, err := tx.Exec(r.Context(), "DELETE FROM article_translations WHERE article_id = $1 AND language_code = $2", articleID, langCode)
	if err != nil {
		http.Error(w, "Failed to delete article translation", http.StatusInternalServerError)
		log.Printf("Error deleting translation (article ID %d, lang %s): %v", articleID, langCode, err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// This error is less common for DELETE but good to check
		http.Error(w, "Error checking affected rows after delete", http.StatusInternalServerError)
		log.Printf("Error checking affected rows for translation (article ID %d, lang %s): %v", articleID, langCode, err)
		return
	}
	if rowsAffected == 0 {
		// This means the translation for the given langCode didn't exist for this article.
		// This can be treated as a 404 or a success (idempotent delete).
		// For DELETE, idempotency is often preferred, so 204 is fine.
		// If strict "must exist to be deleted" is required, then return 404.
		// Let's assume 204 is okay even if it didn't exist.
	}

	// Commit the transaction
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		log.Printf("Error committing transaction for delete translation: %v", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}


func (a *App) deleteArticleHandler(w http.ResponseWriter, r *http.Request, slug string) {
	authUserID, ok := getUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		log.Println("Error: User ID not found in context for deleteArticleHandler")
		return
	}

	// Authorization Check: Fetch current author_id and compare with authUserID
	var currentAuthorID sql.NullInt64
	err := a.DB.QueryRow(r.Context(), "SELECT author_id FROM articles WHERE slug = $1", slug).Scan(&currentAuthorID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Article not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error during delete pre-check", http.StatusInternalServerError)
			log.Printf("Error fetching article author for delete (slug: %s): %v", slug, err)
		}
		return
	}

	if !currentAuthorID.Valid || currentAuthorID.Int64 != authUserID {
		http.Error(w, "Forbidden: You are not authorized to delete this article", http.StatusForbidden)
		return
	}

	// Proceed with deletion
	result, err := a.DB.Exec(r.Context(), "DELETE FROM articles WHERE slug = $1", slug)
	if err != nil {
		// This error is after the authorization check, so it's a server error if deletion fails.
		http.Error(w, "Failed to delete article", http.StatusInternalServerError)
		log.Printf("Error deleting article by slug %s (authorization passed): %v", slug, err)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Article not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) fetchArticleByID(ctx context.Context, articleID int64) (*ArticleResponse, error) {
    return a.fetchArticleWithFilter(ctx, "id", articleID, "")
}

func (a *App) fetchArticleBySlugWithTranslations(ctx context.Context, slug string, lang string) (*ArticleResponse, error) {
    return a.fetchArticleWithFilter(ctx, "slug", slug, lang)
}

func (a *App) fetchArticleWithFilter(ctx context.Context, filterKey string, filterValue interface{}, lang string) (*ArticleResponse, error) {
	var article Article
	var authorID sql.NullInt64

	queryArticle := fmt.Sprintf("SELECT id, slug, author_id, created_at, updated_at FROM articles WHERE %s = $1", filterKey)
	err := a.DB.QueryRow(ctx, queryArticle, filterValue).Scan(
		&article.ID, &article.Slug, &authorID, &article.CreatedAt, &article.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" { 
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("querying article by %s %v: %w", filterKey, filterValue, err)
	}
    if authorID.Valid {
        article.AuthorID = &authorID.Int64
    }

	var translations []ArticleTranslation
	queryTranslations := "SELECT id, article_id, language_code, title, content, is_published, published_at, created_at, updated_at FROM article_translations WHERE article_id = $1"
	args := []interface{}{article.ID}
	if lang != "" {
		queryTranslations += " AND language_code = $2"
		args = append(args, lang)
	} else {
        // If no specific lang, fetch all translations for this article
    }

	rows, err := a.DB.Query(ctx, queryTranslations, args...)
	if err != nil {
		return nil, fmt.Errorf("querying translations for article ID %d: %w", article.ID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var trans ArticleTranslation
		var transPublishedAt sql.NullTime
		
		err := rows.Scan(
			&trans.ID, &trans.ArticleID, &trans.LanguageCode, &trans.Title, &trans.Content,
			&trans.IsPublished, &transPublishedAt, &trans.CreatedAt, &trans.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning translation for article ID %d: %w", article.ID, err)
		}
		if transPublishedAt.Valid {
			trans.PublishedAt = &transPublishedAt.Time
		}
		translations = append(translations, trans)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating translations for article ID %d: %w", article.ID, rows.Err())
	}
    
	return &ArticleResponse{
		ID:           article.ID,
		Slug:         article.Slug,
		AuthorID:     article.AuthorID,
		CreatedAt:    article.CreatedAt,
		UpdatedAt:    article.UpdatedAt,
		Translations: translations,
	}, nil
}
