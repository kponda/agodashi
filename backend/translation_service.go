package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiTranslationService provides a client for the Gemini API for translation tasks.
type GeminiTranslationService struct {
	apiKey     string
	genaiClient *genai.GenerativeModel // Using genai.GenerativeModel for text generation
}

// NewGeminiTranslationService creates a new service for translating text using Gemini.
// It expects the API key to be passed directly.
func NewGeminiTranslationService(apiKey string) (*GeminiTranslationService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is required")
	}

	// Initialize the genai client (using a specific model for text generation)
	// The model name might need to be adjusted based on Gemini documentation for general text/translation.
	// "gemini-pro" is a common model for text generation.
	// For more specific translation models, the name might differ, e.g. "gemini-1.5-flash-latest" or similar.
	// Let's assume "gemini-1.5-flash-latest" for now as it's fast and capable.
	// The actual model choice should be based on availability and suitability for translation tasks.
	
	// Note: The SDK uses GOOGLE_API_KEY environment variable by default if no explicit API key is provided
	// via option.WithAPIKey(). Since we have it in AppConfig, we'll pass it.
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	// Specify the model to use.
	model := client.GenerativeModel("gemini-1.5-flash-latest") // Or "gemini-pro" if flash is not available/suitable

	return &GeminiTranslationService{
		apiKey:     apiKey,
		genaiClient: model,
	}, nil
}

// TranslateText translates the given text to the target language, optionally using a source language hint.
func (s *GeminiTranslationService) TranslateText(ctx context.Context, textToTranslate string, targetLanguageCode string, sourceLanguageCode string) (string, error) {
	if s.genaiClient == nil {
		return "", fmt.Errorf("Gemini client not initialized")
	}

	// Construct the prompt for translation.
	// Prompt engineering is key here.
	var prompt string
	if sourceLanguageCode != "" {
		prompt = fmt.Sprintf("Translate the following text from %s to %s: %s", sourceLanguageCode, targetLanguageCode, textToTranslate)
	} else {
		// If source language is not provided, ask the model to detect and translate.
		prompt = fmt.Sprintf("Translate the following text to %s: %s", targetLanguageCode, textToTranslate)
	}

	log.Printf("Gemini Translation Prompt: %s", prompt)

	resp, err := s.genaiClient.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content from Gemini: %w", err)
	}

	// Extract the translated text from the response.
	// The response structure needs to be handled according to the genai SDK.
	// Typically, the response contains parts, and one of them is the text.
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no translated content found in Gemini response")
	}
	
	// Assuming the first part of the first candidate is the translated text.
	// This might need adjustment based on actual API response structure or if streaming.
    var translatedText string
    for _, cand := range resp.Candidates {
        if cand.Content != nil {
            for _, part := range cand.Content.Parts {
                if txt, ok := part.(genai.Text); ok {
                    translatedText += string(txt)
                }
            }
        }
    }

	if translatedText == "" {
		log.Printf("Gemini response: %+v", resp) // Log the full response for debugging
		return "", fmt.Errorf("extracted translated text is empty from Gemini response")
	}
    
	log.Printf("Gemini Translated Text: %s", translatedText)
	return translatedText, nil
}

// Close cleans up resources used by the service, like the genai client.
func (s *GeminiTranslationService) Close() {
	// The genai.Client (created in NewGeminiTranslationService from genai.NewClient)
	// might have a Close() method. Let's assume it does for proper resource management.
	// However, `GenerativeModel` itself doesn't have Close(). The client it's derived from does.
	// For simplicity here, we'll assume the client used to create the model would be closed if needed,
	// but it's not directly held by GeminiTranslationService.
	// If the client was stored on the service:
	// if s.genaiClient != nil { s.genaiClient.Close() } // This is conceptual.
	// The current SDK's `genai.Client` (returned by `NewClient`) does have a `Close()` method.
	// If `NewGeminiTranslationService` stored the `genai.Client` instead of just the `GenerativeModel`,
	// then we could close it here.
	// For now, this is a placeholder as the model itself doesn't need closing.
}


// Helper function to get GEMINI_API_KEY, used if not passed directly
// This is more for local testing or if the service was self-initializing its key.
// In our app, the key is passed via AppConfig.
func getGeminiAPIKeyFromEnv() string {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Println("Warning: GEMINI_API_KEY environment variable is not set.")
	}
	return key
}
