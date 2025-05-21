# Stage 1: Build the Angular application
FROM node:18-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy package.json and package-lock.json (if available)
COPY package.json ./
# If package-lock.json is not present, npm install will generate it or use package.json alone.
# For robustness, one might run `npm i --package-lock-only` before this step if needed in a CI/CD.
COPY package-lock.json ./

# Install dependencies
# Using --legacy-peer-deps for potential compatibility issues with Angular and newer Node versions
RUN npm install --legacy-peer-deps

# Copy the rest of the application source code
COPY . .

# Build the application for production
# The output directory is typically dist/<project-name>, which is dist/app here.
RUN npm run build -- --configuration production

# Stage 2: Serve the application with Nginx
FROM nginx:alpine

# Copy the build output from the builder stage to Nginx's web server directory
COPY --from=builder /app/dist/app /usr/share/nginx/html

# Expose port 80
EXPOSE 80

# Nginx will automatically start and serve files from /usr/share/nginx/html
# Default CMD for nginx image is ["nginx", "-g", "daemon off;"]
