package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const openAIEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAIClient implements Client for OpenAI.
type OpenAIClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 300 * time.Second},
	}
}

func (c *OpenAIClient) Provider() Provider { return ProviderOpenAI }
func (c *OpenAIClient) Model() string      { return c.model }

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	Temperature         float64         `json:"temperature,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Seed                *int            `json:"seed,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Model string `json:"model"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// usesCompletionTokens returns true if the model uses max_completion_tokens
// instead of max_tokens. This applies to o1, o3, gpt-4o, gpt-5 and newer models.
// Only legacy models (gpt-3.5, gpt-4 without suffix) use max_tokens.
func usesCompletionTokens(model string) bool {
	m := strings.ToLower(model)
	// Legacy models that use max_tokens
	if strings.HasPrefix(m, "gpt-3.5") || m == "gpt-4" || strings.HasPrefix(m, "gpt-4-") {
		return false
	}
	// All other models (o1, o3, gpt-4o, gpt-5, etc.) use max_completion_tokens
	return true
}

// Complete sends a completion request to OpenAI.
func (c *OpenAIClient) Complete(ctx context.Context, req Request) (*Response, error) {
	log.Printf("OpenAI: starting request to model %s", c.model)
	messages := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}

	oaiReq := openAIRequest{
		Model:    c.model,
		Messages: messages,
		Seed:     req.Seed,
	}

	// Newer models (o1, gpt-4o, etc.) use max_completion_tokens instead of max_tokens
	if usesCompletionTokens(c.model) {
		oaiReq.MaxCompletionTokens = req.MaxTokens
	} else {
		oaiReq.Temperature = req.Temperature
		oaiReq.MaxTokens = req.MaxTokens
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openAIEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	log.Printf("OpenAI: sending HTTP request (prompt size: %d bytes)", len(body))
	resp, err := c.client.Do(httpReq)
	if err != nil {
		log.Printf("OpenAI: HTTP error: %v", err)
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("OpenAI: received response status %d", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	log.Printf("OpenAI: response body size: %d bytes", len(respBody))

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	var oaiResp openAIResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if oaiResp.Error != nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderError, oaiResp.Error.Message)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, ErrInvalidResponse
	}

	return &Response{
		Content: oaiResp.Choices[0].Message.Content,
		Model:   oaiResp.Model,
	}, nil
}

const openAIModelsEndpoint = "https://api.openai.com/v1/models"

// OpenAIModelsResponse represents the response from the models endpoint.
type OpenAIModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// FetchOpenAIModels fetches available models from the OpenAI API.
func FetchOpenAIModels(apiKey string) ([]ModelInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", openAIModelsEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch models failed: %s", string(body[:min(200, len(body))]))
	}

	var modelsResp OpenAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, 0)

	for _, m := range modelsResp.Data {
		// Check if it's a chat-capable model
		if isOpenAIChatModel(m.ID) {
			models = append(models, ModelInfo{
				ID:       m.ID,
				Name:     formatOpenAIModelName(m.ID),
				Provider: ProviderOpenAI,
			})
		}
	}

	return models, nil
}

// isOpenAIChatModel returns true if the model ID indicates a chat-capable model.
func isOpenAIChatModel(id string) bool {
	m := strings.ToLower(id)

	// Exclude non-chat models
	excludePrefixes := []string{
		"whisper",        // Audio transcription
		"tts",            // Text-to-speech
		"dall-e",         // Image generation
		"text-embedding", // Embeddings
		"embedding",      // Embeddings
		"moderation",     // Content moderation
		"babbage",        // Legacy completion
		"davinci",        // Legacy completion
		"curie",          // Legacy completion
		"ada",            // Legacy/embeddings
		"code-",          // Legacy code models
		"text-",          // Legacy text models
		"ft:",            // Fine-tuned models
		"codex",          // Legacy
	}

	for _, prefix := range excludePrefixes {
		if strings.HasPrefix(m, prefix) {
			return false
		}
	}

	// Include chat-capable model patterns
	chatPrefixes := []string{
		"gpt-",     // GPT models
		"o1",       // o1 reasoning models
		"o3",       // o3 reasoning models
		"o4",       // Future o4 models
		"chatgpt-", // ChatGPT models
	}

	for _, prefix := range chatPrefixes {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}

	return false
}

// formatOpenAIModelName creates a display name from a model ID.
func formatOpenAIModelName(id string) string {
	// Special cases for well-known models
	wellKnown := map[string]string{
		"gpt-4o":            "GPT-4o",
		"gpt-4o-mini":       "GPT-4o Mini",
		"gpt-4-turbo":       "GPT-4 Turbo",
		"gpt-4":             "GPT-4",
		"gpt-3.5-turbo":     "GPT-3.5 Turbo",
		"o1":                "o1",
		"o1-mini":           "o1 Mini",
		"o1-preview":        "o1 Preview",
		"o3":                "o3",
		"o3-mini":           "o3 Mini",
		"chatgpt-4o-latest": "ChatGPT-4o Latest",
	}

	if name, ok := wellKnown[id]; ok {
		return name
	}

	// For other models, create a readable name from the ID
	name := id
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Capitalize GPT
	name = strings.ReplaceAll(name, "gpt ", "GPT ")

	return name
}
