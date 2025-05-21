CREATE TABLE IF NOT EXISTS article_translations (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL,
    language_code VARCHAR(10) NOT NULL,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT article_translations_article_id_fkey FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE,
    CONSTRAINT article_translations_article_id_language_code_key UNIQUE (article_id, language_code)
);

CREATE INDEX IF NOT EXISTS idx_article_translations_article_id ON article_translations(article_id);
CREATE INDEX IF NOT EXISTS idx_article_translations_language_code ON article_translations(language_code);
