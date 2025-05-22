ALTER TABLE users ADD COLUMN default_language_code VARCHAR(10) DEFAULT 'en';

COMMENT ON COLUMN users.default_language_code IS 'User preferred language code for creating new content';
