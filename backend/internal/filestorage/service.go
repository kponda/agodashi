package filestorage

import (
	"mime/multipart"
)

// FileStorageService defines the operations for storing and retrieving files.
type FileStorageService interface {
	// Upload saves the file from the given io.Reader and returns its relative storage path or an error.
	// The original filename is used to determine the file extension.
	// subDirectory can be used to organize files (e.g., "images", "avatars").
	Upload(file multipart.File, header *multipart.FileHeader, subDirectory string) (filePath string, err error)

	// GetPublicURL returns the publicly accessible URL for a given file path.
	// filePath is the relative path returned by Upload.
	GetPublicURL(filePath string) string

	// Delete removes a file specified by its relative filePath.
	// Returns error if the deletion fails.
	Delete(filePath string) error
}
