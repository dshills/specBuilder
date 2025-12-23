package llm

import (
	"context"
)

// MockClient is a mock LLM client for testing.
type MockClient struct {
	Response    string
	Error       error
	CallCount   int
	LastRequest *Request
}

// NewMockClient creates a new mock LLM client.
func NewMockClient(response string) *MockClient {
	return &MockClient{Response: response}
}

// Complete returns the mock response.
func (c *MockClient) Complete(ctx context.Context, req Request) (*Response, error) {
	c.CallCount++
	c.LastRequest = &req

	if c.Error != nil {
		return nil, c.Error
	}

	return &Response{
		Content: c.Response,
		Model:   "mock-model",
	}, nil
}

// Provider returns the mock provider.
func (c *MockClient) Provider() Provider {
	return "mock"
}

// Model returns the mock model name.
func (c *MockClient) Model() string {
	return "mock-model"
}

// Ensure MockClient implements Client
var _ Client = (*MockClient)(nil)
