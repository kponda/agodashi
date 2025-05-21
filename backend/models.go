package main

import (
	"time"
)

type Article struct {
	ID        int64      `json:"id"`
	Slug      string     `json:"slug"`
	AuthorID  *int64     `json:"author_id,omitempty"` // Nullable
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type ArticleTranslation struct {
	ID             int64      `json:"id"`
	ArticleID      int64      `json:"article_id"`
	LanguageCode   string     `json:"language_code"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	IsPublished    bool       `json:"is_published"`
	PublishedAt    *time.Time `json:"published_at,omitempty"` // Nullable
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateArticleTranslation is used within CreateArticleRequest
type CreateArticleTranslation struct {
	LanguageCode string `json:"language_code" binding:"required"`
	Title        string `json:"title" binding:"required"`
	Content      string `json:"content" binding:"required"`
	IsPublished  bool   `json:"is_published"`
}

// CreateArticleRequest is the request body for creating a new article
type CreateArticleRequest struct {
	Slug         string                     `json:"slug" binding:"required"`
	AuthorID     *int64                     `json:"author_id,omitempty"`
	Translations []CreateArticleTranslation `json:"translations" binding:"required,dive"`
}

// ArticleResponse is used for GET requests to return an article with its translations
type ArticleResponse struct {
	ID           int64                `json:"id"`
	Slug         string               `json:"slug"`
	AuthorID     *int64               `json:"author_id,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
	Translations []ArticleTranslation `json:"translations"`
}
