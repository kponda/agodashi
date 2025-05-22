package filestorage

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// LocalFileStorage implements FileStorageService using the local filesystem.
type LocalFileStorage struct {
	BasePath string // Absolute path on filesystem, e.g., /app/uploads
	BaseURL  string // Base URL for serving files, e.g., /uploads
}

// NewLocalFileStorage creates a new LocalFileStorage instance.
// It ensures the basePath directory exists and is writable.
func NewLocalFileStorage(basePath string, baseURL string) (*LocalFileStorage, error) {
	// Ensure basePath is an absolute path
	if !filepath.IsAbs(basePath) {
		absPath, err := filepath.Abs(basePath)
		if err != nil {
			return nil, fmt.Errorf("could not convert basePath to absolute path: %w", err)
		}
		basePath = absPath
	}

	// Create the base path directory if it doesn't exist
	err := os.MkdirAll(basePath, 0755) // rwxr-xr-x
	if err != nil {
		return nil, fmt.Errorf("failed to create base storage path '%s': %w", basePath, err)
	}

	// Check if basePath is writable (simple check by trying to create a temp file)
	tempFile, err := os.CreateTemp(basePath, "test_writable_")
	if err != nil {
		return nil, fmt.Errorf("basePath '%s' is not writable: %w", basePath, err)
	}
	tempFile.Close()
	os.Remove(tempFile.Name()) // Clean up temp file

	return &LocalFileStorage{
		BasePath: basePath,
		BaseURL:  baseURL,
	}, nil
}

// Upload saves the file from the given multipart.File and returns its relative storage path.
func (s *LocalFileStorage) Upload(file multipart.File, header *multipart.FileHeader, subDirectory string) (string, error) {
	if file == nil {
		return "", fmt.Errorf("file is nil")
	}
	if header == nil {
		return "", fmt.Errorf("file header is nil")
	}

	// Generate unique filename (UUID + original extension)
	originalFilename := header.Filename
	extension := filepath.Ext(originalFilename)
	uniqueFilename := uuid.New().String() + extension

	// Construct target directory path
	targetDir := filepath.Join(s.BasePath, subDirectory)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target subdirectory '%s': %w", targetDir, err)
	}

	// Construct full destination path for the file
	destinationPath := filepath.Join(targetDir, uniqueFilename)

	// Create the destination file
	dst, err := os.Create(destinationPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file '%s': %w", destinationPath, err)
	}
	defer dst.Close()

	// Copy the uploaded file content to the destination file
	if _, err := io.Copy(dst, file); err != nil {
		// Attempt to remove partially written file on error
		os.Remove(destinationPath)
		return "", fmt.Errorf("failed to copy file content to '%s': %w", destinationPath, err)
	}

	// Return relative path for storage
	relativePath := filepath.Join(subDirectory, uniqueFilename)
	return relativePath, nil
}

// GetPublicURL returns the publicly accessible URL for a given file path.
func (s *LocalFileStorage) GetPublicURL(filePath string) string {
	// Ensure forward slashes for URL, even if filePath might use backslashes (e.g., on Windows)
	urlPath := strings.ReplaceAll(filePath, string(filepath.Separator), "/")
	return s.BaseURL + "/" + strings.TrimLeft(urlPath, "/")
}

// Delete removes a file specified by its relative filePath.
func (s *LocalFileStorage) Delete(filePath string) error {
	fullPath := filepath.Join(s.BasePath, filePath)

	// Check if file exists before attempting to delete
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found at '%s': %w", fullPath, err)
	}

	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete file '%s': %w", fullPath, err)
	}
	return nil
}
