package llm

import (
	"fmt"
	"log"
	"os"
)

// ModelInfo describes an available model.
type ModelInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Provider Provider `json:"provider"`
}

// ProviderInfo describes an available provider.
type ProviderInfo struct {
	ID        Provider    `json:"id"`
	Name      string      `json:"name"`
	Available bool        `json:"available"`
	Models    []ModelInfo `json:"models"`
}

// ClientFactory is the interface for LLM client factories.
type ClientFactory interface {
	Available() bool
	DefaultProvider() Provider
	DefaultModel() string
	ListProviders() []ProviderInfo
	CreateClient(provider Provider, model string) (Client, error)
	CreateDefaultClient() (Client, error)
}

// Factory creates LLM clients on demand.
type Factory struct {
	geminiKey    string
	openAIKey    string
	anthropicKey string
	defaultMod   string
	defaultPrv   Provider
	providers    []ProviderInfo
}

// NewFactory creates a new LLM client factory.
// It fetches available models from each configured provider's API.
func NewFactory() *Factory {
	f := &Factory{
		geminiKey:    os.Getenv("GEMINI_API_KEY"),
		openAIKey:    os.Getenv("OPENAI_API_KEY"),
		anthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
	}

	// Fetch models from each provider
	f.providers = f.fetchAllProviders()

	// Set defaults based on available keys (prefer Anthropic > Google > OpenAI)
	if f.anthropicKey != "" {
		f.defaultPrv = ProviderAnthropic
		f.defaultMod = f.getFirstModel(ProviderAnthropic, "claude-sonnet-4-20250514")
	} else if f.geminiKey != "" {
		f.defaultPrv = ProviderGoogle
		f.defaultMod = f.getFirstModel(ProviderGoogle, "gemini-2.5-flash")
	} else if f.openAIKey != "" {
		f.defaultPrv = ProviderOpenAI
		f.defaultMod = f.getFirstModel(ProviderOpenAI, "gpt-4o")
	}

	return f
}

// fetchAllProviders fetches models from all configured providers.
func (f *Factory) fetchAllProviders() []ProviderInfo {
	providers := make([]ProviderInfo, 0, 3)

	// Anthropic
	anthropicInfo := ProviderInfo{
		ID:        ProviderAnthropic,
		Name:      "Anthropic Claude",
		Available: f.anthropicKey != "",
		Models:    []ModelInfo{},
	}
	if f.anthropicKey != "" {
		models, err := FetchAnthropicModels(f.anthropicKey)
		if err != nil {
			log.Printf("Warning: failed to fetch Anthropic models: %v", err)
			// Fall back to known models
			anthropicInfo.Models = []ModelInfo{
				{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: ProviderAnthropic},
				{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Provider: ProviderAnthropic},
				{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: ProviderAnthropic},
			}
		} else {
			anthropicInfo.Models = models
		}
	}
	providers = append(providers, anthropicInfo)

	// Google Gemini
	geminiInfo := ProviderInfo{
		ID:        ProviderGoogle,
		Name:      "Google Gemini",
		Available: f.geminiKey != "",
		Models:    []ModelInfo{},
	}
	if f.geminiKey != "" {
		models, err := FetchGeminiModels(f.geminiKey)
		if err != nil {
			log.Printf("Warning: failed to fetch Gemini models: %v", err)
			// Fall back to known models
			geminiInfo.Models = []ModelInfo{
				{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: ProviderGoogle},
				{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: ProviderGoogle},
				{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: ProviderGoogle},
			}
		} else {
			geminiInfo.Models = models
		}
	}
	providers = append(providers, geminiInfo)

	// OpenAI
	openAIInfo := ProviderInfo{
		ID:        ProviderOpenAI,
		Name:      "OpenAI",
		Available: f.openAIKey != "",
		Models:    []ModelInfo{},
	}
	if f.openAIKey != "" {
		models, err := FetchOpenAIModels(f.openAIKey)
		if err != nil {
			log.Printf("Warning: failed to fetch OpenAI models: %v", err)
			// Fall back to known models
			openAIInfo.Models = []ModelInfo{
				{ID: "gpt-4o", Name: "GPT-4o", Provider: ProviderOpenAI},
				{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: ProviderOpenAI},
				{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: ProviderOpenAI},
				{ID: "o1", Name: "o1", Provider: ProviderOpenAI},
				{ID: "o1-mini", Name: "o1 Mini", Provider: ProviderOpenAI},
			}
		} else {
			openAIInfo.Models = models
		}
	}
	providers = append(providers, openAIInfo)

	return providers
}

// getFirstModel returns the first available model for a provider, or a fallback.
func (f *Factory) getFirstModel(provider Provider, fallback string) string {
	for _, p := range f.providers {
		if p.ID == provider && len(p.Models) > 0 {
			return p.Models[0].ID
		}
	}
	return fallback
}

// Available returns true if at least one provider is configured.
func (f *Factory) Available() bool {
	return f.geminiKey != "" || f.openAIKey != "" || f.anthropicKey != ""
}

// DefaultProvider returns the default provider.
func (f *Factory) DefaultProvider() Provider {
	return f.defaultPrv
}

// DefaultModel returns the default model.
func (f *Factory) DefaultModel() string {
	return f.defaultMod
}

// ListProviders returns all providers with their availability status.
func (f *Factory) ListProviders() []ProviderInfo {
	return f.providers
}

// CreateClient creates a client for the specified provider and model.
func (f *Factory) CreateClient(provider Provider, model string) (Client, error) {
	switch provider {
	case ProviderAnthropic:
		if f.anthropicKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not configured")
		}
		return NewAnthropicClient(f.anthropicKey, model), nil

	case ProviderGoogle:
		if f.geminiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY not configured")
		}
		return NewGeminiClient(f.geminiKey, model), nil

	case ProviderOpenAI:
		if f.openAIKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not configured")
		}
		return NewOpenAIClient(f.openAIKey, model), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// CreateDefaultClient creates a client with the default provider and model.
func (f *Factory) CreateDefaultClient() (Client, error) {
	if !f.Available() {
		return nil, fmt.Errorf("no LLM API keys configured")
	}
	return f.CreateClient(f.defaultPrv, f.defaultMod)
}

// Ensure Factory implements ClientFactory
var _ ClientFactory = (*Factory)(nil)
