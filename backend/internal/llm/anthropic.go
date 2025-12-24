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

const anthropicMessagesEndpoint = "https://api.anthropic.com/v1/messages"
const anthropicModelsEndpoint = "https://api.anthropic.com/v1/models"
const anthropicAPIVersion = "2023-06-01"

// AnthropicClient implements Client for Anthropic Claude.
type AnthropicClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &AnthropicClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 180 * time.Second},
	}
}

func (c *AnthropicClient) Provider() Provider { return ProviderAnthropic }
func (c *AnthropicClient) Model() string      { return c.model }

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a completion request to Anthropic.
func (c *AnthropicClient) Complete(ctx context.Context, req Request) (*Response, error) {
	log.Printf("Anthropic: starting request to model %s", c.model)

	// Build messages, extracting system message
	var systemPrompt string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}
		messages = append(messages, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	anthropicReq := anthropicRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    systemPrompt,
		Messages:  messages,
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicMessagesEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	log.Printf("Anthropic: sending HTTP request (prompt size: %d bytes)", len(body))
	resp, err := c.client.Do(httpReq)
	if err != nil {
		log.Printf("Anthropic: HTTP error: %v", err)
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("Anthropic: received response status %d", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	log.Printf("Anthropic: response body size: %d bytes", len(respBody))

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody[:min(500, len(respBody))]))
	}

	if anthropicResp.Error != nil {
		return nil, fmt.Errorf("%w: %s", ErrProviderError, anthropicResp.Error.Message)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("%w: no content in response", ErrInvalidResponse)
	}

	// Extract text content
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	// Strip markdown code blocks if present
	content = stripMarkdownCodeBlock(content)

	return &Response{
		Content: content,
		Model:   c.model,
	}, nil
}

// AnthropicModelsResponse represents the response from the models endpoint.
type AnthropicModelsResponse struct {
	Data []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		CreatedAt   string `json:"created_at"`
		Type        string `json:"type"`
	} `json:"data"`
	HasMore bool   `json:"has_more"`
	FirstID string `json:"first_id"`
	LastID  string `json:"last_id"`
}

// FetchAnthropicModels fetches available models from the Anthropic API.
func FetchAnthropicModels(apiKey string) ([]ModelInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", anthropicModelsEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch models failed: %s", string(body[:min(200, len(body))]))
	}

	var modelsResp AnthropicModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		// Filter to only include claude models suitable for chat
		if !strings.HasPrefix(m.ID, "claude-") {
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.ID
		}
		models = append(models, ModelInfo{
			ID:       m.ID,
			Name:     name,
			Provider: ProviderAnthropic,
		})
	}

	return models, nil
}
