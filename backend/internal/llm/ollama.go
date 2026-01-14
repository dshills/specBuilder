package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Default Ollama endpoint
const defaultOllamaHost = "http://localhost:11434"

// OllamaClient implements Client for Ollama local LLM server.
type OllamaClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaClient creates a new Ollama client.
// It uses OLLAMA_HOST environment variable if set, otherwise defaults to localhost:11434.
func NewOllamaClient(model string) *OllamaClient {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = defaultOllamaHost
	}
	// Ensure no trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	if model == "" {
		model = "llama3.2" // Default to a common model
	}

	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 600 * time.Second}, // Longer timeout for local inference
	}
}

func (c *OllamaClient) Provider() Provider { return ProviderOllama }
func (c *OllamaClient) Model() string      { return c.model }

// ollamaRequest represents the request to Ollama's /api/chat endpoint.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
	Format   string          `json:"format,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	Seed        int     `json:"seed,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // max tokens
}

// ollamaResponse represents the response from Ollama's /api/chat endpoint.
type ollamaResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason,omitempty"`
	TotalDuration   int64         `json:"total_duration,omitempty"`
	LoadDuration    int64         `json:"load_duration,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
	Error           string        `json:"error,omitempty"`
}

// Complete sends a completion request to Ollama.
func (c *OllamaClient) Complete(ctx context.Context, req Request) (*Response, error) {
	log.Printf("Ollama: starting request to model %s at %s", c.model, c.baseURL)

	// Build messages (Ollama uses the same role format as OpenAI)
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Build options
	options := &ollamaOptions{
		Temperature: req.Temperature,
	}
	if req.Seed != nil {
		options.Seed = *req.Seed
	}
	if req.MaxTokens > 0 {
		options.NumPredict = req.MaxTokens
	}

	ollamaReq := ollamaRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false, // Non-streaming for simplicity
		Options:  options,
		Format:   "json", // Request JSON output
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	log.Printf("Ollama: sending HTTP request (prompt size: %d bytes)", len(body))
	resp, err := c.client.Do(httpReq)
	if err != nil {
		log.Printf("Ollama: HTTP error: %v", err)
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("Ollama: received response status %d", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	log.Printf("Ollama: response body size: %d bytes", len(respBody))

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrProviderError, resp.StatusCode, string(respBody[:min(500, len(respBody))]))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody[:min(500, len(respBody))]))
	}

	if ollamaResp.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrProviderError, ollamaResp.Error)
	}

	if ollamaResp.Message.Content == "" {
		return nil, fmt.Errorf("%w: no content in response", ErrInvalidResponse)
	}

	log.Printf("Ollama: done_reason=%s, prompt_tokens=%d, completion_tokens=%d",
		ollamaResp.DoneReason, ollamaResp.PromptEvalCount, ollamaResp.EvalCount)

	// Check if output was truncated
	if ollamaResp.DoneReason == "length" {
		return nil, fmt.Errorf("%w: response truncated (hit token limit)", ErrInvalidResponse)
	}

	content := ollamaResp.Message.Content

	// Strip markdown code blocks if present
	content = stripMarkdownCodeBlock(content)

	return &Response{
		Content: content,
		Model:   c.model,
	}, nil
}

// ollamaTagsResponse represents the response from Ollama's /api/tags endpoint.
type ollamaTagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		Model      string `json:"model"`
		ModifiedAt string `json:"modified_at"`
		Size       int64  `json:"size"`
		Digest     string `json:"digest"`
		Details    struct {
			ParameterSize     string   `json:"parameter_size"`
			QuantizationLevel string   `json:"quantization_level"`
			Family            string   `json:"family"`
			Families          []string `json:"families"`
		} `json:"details"`
	} `json:"models"`
}

// FetchOllamaModels fetches available models from a running Ollama instance.
func FetchOllamaModels(baseURL string) ([]ModelInfo, error) {
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_HOST")
		if baseURL == "" {
			baseURL = defaultOllamaHost
		}
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	client := &http.Client{Timeout: 5 * time.Second}

	endpoint := baseURL + "/api/tags"
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch models failed: status %d: %s", resp.StatusCode, string(body[:min(200, len(body))]))
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		// Use the model name (which includes the tag)
		modelID := m.Name
		if modelID == "" {
			modelID = m.Model
		}

		// Create a friendly display name
		displayName := modelID
		if m.Details.ParameterSize != "" {
			displayName = fmt.Sprintf("%s (%s)", modelID, m.Details.ParameterSize)
		}

		models = append(models, ModelInfo{
			ID:       modelID,
			Name:     displayName,
			Provider: ProviderOllama,
		})
	}

	return models, nil
}

// CheckOllamaAvailable checks if Ollama server is running and accessible.
func CheckOllamaAvailable() bool {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = defaultOllamaHost
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	client := &http.Client{Timeout: 2 * time.Second}

	// Try to hit the tags endpoint
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}
