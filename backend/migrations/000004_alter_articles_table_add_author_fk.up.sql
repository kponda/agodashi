-- The articles.author_id column is assumed to exist from '000001_create_articles_table.up.sql'
-- and is defined as 'author_id BIGINT NULL'.

-- The subtask description has a contradiction:
-- 1. "Alter the articles table to make author_id NON NULL."
-- 2. "ON DELETE SET NULL" for the foreign key.
-- If 'ON DELETE SET NULL' is used, the 'author_id' column *must* be nullable.
-- If 'author_id' were set to NON NULL, the 'SET NULL' action would fail.

-- Prioritizing 'ON DELETE SET NULL' means 'author_id' must remain nullable.
-- Therefore, the 'ALTER TABLE articles ALTER COLUMN author_id SET NOT NULL;' step
-- from the subtask description will be omitted. The column will remain nullable as originally defined.

ALTER TABLE articles
ADD CONSTRAINT fk_articles_author
FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL;
