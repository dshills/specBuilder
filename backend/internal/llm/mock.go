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

// MockFactory is a mock LLM factory for testing.
// It always returns the embedded MockClient regardless of provider/model.
type MockFactory struct {
	Client *MockClient
}

// NewMockFactory creates a new mock factory with the given response.
func NewMockFactory(response string) *MockFactory {
	return &MockFactory{
		Client: NewMockClient(response),
	}
}

// NewMockFactoryWithClient creates a mock factory with a custom MockClient.
func NewMockFactoryWithClient(client *MockClient) *MockFactory {
	return &MockFactory{
		Client: client,
	}
}

// Available always returns true for mock factory.
func (f *MockFactory) Available() bool {
	return true
}

// DefaultProvider returns a mock provider.
func (f *MockFactory) DefaultProvider() Provider {
	return "mock"
}

// DefaultModel returns a mock model.
func (f *MockFactory) DefaultModel() string {
	return "mock-model"
}

// ListProviders returns mock providers.
func (f *MockFactory) ListProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			ID:        "mock",
			Name:      "Mock Provider",
			Available: true,
			Models: []ModelInfo{
				{ID: "mock-model", Name: "Mock Model", Provider: "mock"},
			},
		},
	}
}

// CreateClient returns the mock client.
func (f *MockFactory) CreateClient(provider Provider, model string) (Client, error) {
	return f.Client, nil
}

// CreateDefaultClient returns the mock client.
func (f *MockFactory) CreateDefaultClient() (Client, error) {
	return f.Client, nil
}

// Ensure MockFactory implements ClientFactory
var _ ClientFactory = (*MockFactory)(nil)
