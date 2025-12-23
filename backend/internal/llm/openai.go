package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		client: &http.Client{Timeout: 120 * time.Second},
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

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

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
