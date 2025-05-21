-- This script rolls back the changes made in 000004_alter_articles_table_add_author_fk.up.sql

-- Drop the foreign key constraint
ALTER TABLE articles
DROP CONSTRAINT IF EXISTS fk_articles_author;

-- The corresponding UP script (000004_alter_articles_table_add_author_fk.up.sql)
-- did NOT alter author_id to be NON NULL, because it would conflict with the
-- 'ON DELETE SET NULL' referential action chosen for the foreign key.
-- The column 'author_id' was originally defined as NULLABLE in '000001_create_articles_table.up.sql'
-- and was kept as NULLABLE.
-- Therefore, there is no 'ALTER TABLE articles ALTER COLUMN author_id DROP NOT NULL;'
-- command here, as the column was never made NOT NULL in the corresponding UP script.
