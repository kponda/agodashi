package main

import (
	"encoding/json"
	"log"
	"net/http"
)

const maxUploadSize = 10 << 20 // 10 MB

// ImageUploadResponse defines the JSON response for a successful image upload.
type ImageUploadResponse struct {
	URL string `json:"url"`
}

// uploadImageHandler handles image uploads.
// It expects a multipart form with an "image" field.
func (a *App) uploadImageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if FileStorage service is available
	if a.FileStorage == nil {
		log.Println("Error: FileStorage service not available for uploadImageHandler")
		http.Error(w, "File storage service is not configured", http.StatusInternalServerError)
		return
	}

	// Parse the multipart form
	// The request body needs to be parsed first before reading form files.
	// maxUploadSize is used to limit the size of the request body.
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		if err.Error() == "http: request body too large" { // Check specific error for payload too large
			http.Error(w, "File too large. Maximum upload size is 10MB.", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Could not parse multipart form: "+err.Error(), http.StatusBadRequest)
		}
		return
	}

	// Get the file from the form data
	file, header, err := r.FormFile("image") // "image" is the form field name
	if err != nil {
		http.Error(w, "Invalid image file in request: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Log userID from context (set by authMiddleware) for audit or ownership if needed later
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		// This should not happen if authMiddleware is applied correctly.
		// Depending on policy, could be an error or proceed without userID for the file itself.
		// For now, just log it. If file ownership by user is critical, this should be an error.
		log.Println("Warning: User ID not found in context for uploadImageHandler, but proceeding.")
	} else {
		log.Printf("User ID %d uploading image: %s", userID, header.Filename)
	}


	// Upload the file using the FileStorageService
	// Using "images" as the subdirectory. This could be dynamic based on user or content type.
	filePath, err := a.FileStorage.Upload(file, header, "images")
	if err != nil {
		log.Printf("Error uploading file: %v", err)
		http.Error(w, "Failed to upload file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the public URL of the uploaded file
	publicURL := a.FileStorage.GetPublicURL(filePath)

	// Return the public URL in the response
	response := ImageUploadResponse{URL: publicURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 Created is often appropriate for successful uploads
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding image upload response: %v", err)
		// Client already received 201, but log this server-side issue
	}
}
