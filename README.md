# 翻訳機能付きBlog/CMSシステム

This project is a Blog/CMS system with translation functionality.
The backend API is built with Go, and the frontend is built with Angular.
The entire project is managed as a monorepo.

## Project Structure

-   `backend/`: Contains the Go backend API.
-   `frontend/`: Contains the Angular frontend application.
    -   `frontend/app/`: The Angular project source code.
-   `infrastructure/`: Contains infrastructure-related configurations, like database data.

## Getting Started

### Prerequisites

-   Docker
-   Docker Compose

### Running the Application

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd <repository-name>
    ```

2.  **Build and run the services using Docker Compose:**
    ```bash
    docker-compose up --build
    ```

    This command will:
    *   Build the Docker images for the backend and frontend.
    *   Start containers for the backend, frontend, and PostgreSQL database.

3.  **Access the applications:**
    *   Backend API: `http://localhost:8080`
    *   Frontend Application: `http://localhost:4200`
    *   PostgreSQL Database: Connect on port `5432` (credentials in `docker-compose.yml`)

## Development

(Details about development workflows, testing, etc., can be added here later.)

## Deployment

-   Backend: Planned for Cloud Run.
-   Frontend: Planned for Firebase Hosting.

(Further details on deployment will be added as the project progresses.)
