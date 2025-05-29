# CLAUDE.md - Build Commands & Code Style Guidelines

## Build Commands
### Backend (Go)
- Run backend: `docker-compose -f docker-compose.yml -f docker-compose.dev.yml up --build backend`
- Database migrations: `make migrate-up` or `make migrate-down` (run inside backend container)
- Create migration: `make migrate-create NAME=migration_name`

### Frontend (Angular)
- Run frontend: `docker-compose -f docker-compose.yml -f docker-compose.dev.yml up --build frontend`
- Build: `cd frontend/app && npm run build`
- Test: `cd frontend/app && npm run test`
- Run single test: `cd frontend/app && ng test --include=**/component-name.component.spec.ts`

## Code Style Guidelines
### Go Backend
- Error handling: Check errors immediately, log with context, return appropriate HTTP status codes
- Variable naming: camelCase for variables, PascalCase for exported functions/types
- Use structured logging with context (`log.Printf("Error: %v", err)`)
- Group imports: standard library first, then external packages, then local packages
- Always use context propagation for database operations

### Angular Frontend
- TypeScript strict mode is enabled - use proper typing throughout
- Use RxJS observables for asynchronous operations
- Follow Angular-style component structure (component class, HTML template, SCSS)
- Prefer services for shared logic and state management
- Prefer interfaces for data models and API responses