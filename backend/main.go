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

// App holds application-wide dependencies, like the database pool.
type App struct {
	DB *pgxpool.Pool
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

	app := &App{DB: dbpool}

	mux := http.NewServeMux()

	// Register handlers using app instance with /api prefix
	mux.HandleFunc("/api/", app.rootHandler) // Health check for /api/
	mux.HandleFunc("/api/articles", app.articlesRouter)
	mux.HandleFunc("/api/articles/", app.articleBySlugRouter) // Handles /api/articles/{slug}

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

// articlesRouter routes requests for /api/articles based on HTTP method
func (a *App) articlesRouter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.createArticleHandler(w, r)
	case http.MethodGet:
		a.listArticlesHandler(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// articleBySlugRouter routes requests for /api/articles/{slug} based on HTTP method
func (a *App) articleBySlugRouter(w http.ResponseWriter, r *http.Request) {
	// Slug extraction from path like /api/articles/{slug}
	// The +1 is to skip the trailing slash if path is /api/articles/
	slug := r.URL.Path[len("/api/articles/"):]
	if slug == "" {
		// This case might be hit if the path is just "/api/articles/" without a slug.
		// Depending on desired behavior, could be a 404 or a different handler.
		// For now, treat as slug not provided.
		http.Error(w, "Slug not provided", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getArticleHandler(w, r, slug)
	case http.MethodPut:
		a.updateArticleHandler(w, r, slug)
	case http.MethodDelete:
		a.deleteArticleHandler(w, r, slug)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) createArticleHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	tx, err := a.DB.Begin(r.Context()) // Use request context for transaction
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		log.Printf("Error starting transaction: %v", err)
		return
	}
	defer tx.Rollback(r.Context())

	var articleID int64
	err = tx.QueryRow(r.Context(),
		"INSERT INTO articles (slug, author_id) VALUES ($1, $2) RETURNING id",
		req.Slug, req.AuthorID).Scan(&articleID)
	if err != nil {
		http.Error(w, "Failed to insert article", http.StatusInternalServerError)
		log.Printf("Error inserting article: %v", err)
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

    tx, err := a.DB.Begin(r.Context())
    if err != nil {
        http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
        log.Printf("Error starting transaction: %v", err)
        return
    }
    defer tx.Rollback(r.Context())

    var articleID int64
    err = tx.QueryRow(r.Context(), "SELECT id FROM articles WHERE slug = $1", slug).Scan(&articleID)
    if err != nil {
        if err.Error() == "no rows in result set" { 
            http.Error(w, "Article not found", http.StatusNotFound)
        } else {
            http.Error(w, "Failed to find article", http.StatusInternalServerError)
        }
        log.Printf("Error finding article by slug %s: %v", slug, err)
        return
    }
    
    newSlug := slug
    if req.Slug != "" && req.Slug != slug { // If slug in request is different, update it
        newSlug = req.Slug
         _, err = tx.Exec(r.Context(), "UPDATE articles SET slug = $1, author_id = $2, updated_at = $3 WHERE id = $4",
            newSlug, req.AuthorID, time.Now().UTC(), articleID)
    } else { // Slug is not changing or not provided in request body, only update other fields
         _, err = tx.Exec(r.Context(), "UPDATE articles SET author_id = $1, updated_at = $2 WHERE id = $3",
            req.AuthorID, time.Now().UTC(), articleID)
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


func (a *App) deleteArticleHandler(w http.ResponseWriter, r *http.Request, slug string) {
	result, err := a.DB.Exec(r.Context(), "DELETE FROM articles WHERE slug = $1", slug)
	if err != nil {
		http.Error(w, "Failed to delete article", http.StatusInternalServerError)
		log.Printf("Error deleting article by slug %s: %v", slug, err)
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
