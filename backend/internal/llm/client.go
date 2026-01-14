package llm

import (
	"context"
	"errors"
)

// Provider represents an LLM provider.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGoogle    Provider = "google"
	ProviderOllama    Provider = "ollama"
)

// Config holds LLM configuration.
type Config struct {
	Provider    Provider
	Model       string
	APIKey      string
	Temperature float64
	Seed        *int
}

// Request represents a chat completion request.
type Request struct {
	Messages    []Message
	Temperature float64
	Seed        *int
	MaxTokens   int
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// Response represents a chat completion response.
type Response struct {
	Content string
	Model   string
}

// Client is the interface for LLM providers.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
	Provider() Provider
	Model() string
}

var (
	// ErrInvalidResponse indicates the LLM returned an invalid response.
	ErrInvalidResponse = errors.New("invalid LLM response")

	// ErrRateLimit indicates rate limiting was hit.
	ErrRateLimit = errors.New("rate limit exceeded")

	// ErrProviderError indicates a provider-specific error.
	ErrProviderError = errors.New("provider error")
)
