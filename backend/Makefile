# Default Database URL. Can be overridden by setting the DATABASE_URL environment variable.
DB_URL ?= postgres://user:password@postgres:5432/appdb?sslmode=disable

# Migrations directory
MIGRATIONS_DIR = ./migrations

# Target to create a new migration file
# Example: make migrate-create NAME=create_users_table
migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME variable is not set. Usage: make migrate-create NAME=<migration_name>"; \
		exit 1; \
	fi
	@echo "Creating migration file: $(NAME)"
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)

# Target to apply all pending migrations
migrate-up:
	@echo "Applying migrations..."
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

# Target to roll back the last applied migration
migrate-down:
	@echo "Rolling back the last migration..."
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

# Phony targets to avoid conflicts with file names
.PHONY: migrate-create migrate-up migrate-down
