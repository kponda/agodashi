-- Remove the comment first if it was added and the DB system requires it
-- For PostgreSQL, a comment on a column is automatically dropped when the column is dropped.
-- So, no explicit 'COMMENT ON COLUMN users.default_language_code IS NULL;' is strictly needed here.

ALTER TABLE users DROP COLUMN default_language_code;
