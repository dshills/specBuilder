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

const geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

// GeminiClient implements Client for Google Gemini.
type GeminiClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGeminiClient creates a new Gemini client.
func NewGeminiClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &GeminiClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 180 * time.Second},
	}
}

func (c *GeminiClient) Provider() Provider { return ProviderGoogle }
func (c *GeminiClient) Model() string      { return c.model }

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig *geminiGenConfig `json:"generationConfig,omitempty"`
	SystemInstruct   *geminiContent   `json:"system_instruction,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	TopP             float64 `json:"topP,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// Complete sends a completion request to Gemini.
func (c *GeminiClient) Complete(ctx context.Context, req Request) (*Response, error) {
	log.Printf("Gemini: starting request to model %s", c.model)

	// Build contents from messages
	contents := make([]geminiContent, 0, len(req.Messages))
	var systemInstruct *geminiContent

	for _, m := range req.Messages {
		if m.Role == "system" {
			// Gemini uses system_instruction for system messages
			systemInstruct = &geminiContent{
				Parts: []geminiPart{{Text: m.Content}},
			}
			continue
		}

		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	gemReq := geminiRequest{
		Contents:       contents,
		SystemInstruct: systemInstruct,
		GenerationConfig: &geminiGenConfig{
			Temperature:      req.Temperature,
			MaxOutputTokens:  req.MaxTokens,
			TopP:             0.95,
			ResponseMimeType: "application/json", // Force JSON output
		},
	}

	body, err := json.Marshal(gemReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := fmt.Sprintf(geminiEndpoint, c.model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.apiKey)

	log.Printf("Gemini: sending HTTP request (prompt size: %d bytes)", len(body))
	resp, err := c.client.Do(httpReq)
	if err != nil {
		log.Printf("Gemini: HTTP error: %v", err)
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("Gemini: received response status %d", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	log.Printf("Gemini: response body size: %d bytes", len(respBody))

	if resp.StatusCode == 429 {
		return nil, ErrRateLimit
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody[:min(500, len(respBody))]))
	}

	if gemResp.Error != nil {
		return nil, fmt.Errorf("%w: %s (code: %d)", ErrProviderError, gemResp.Error.Message, gemResp.Error.Code)
	}

	if len(gemResp.Candidates) == 0 {
		return nil, fmt.Errorf("%w: no candidates in response", ErrInvalidResponse)
	}

	candidate := gemResp.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("%w: no parts in candidate", ErrInvalidResponse)
	}

	content := candidate.Content.Parts[0].Text
	// Strip markdown code blocks if present (Gemini sometimes wraps JSON in ```json ... ```)
	content = stripMarkdownCodeBlock(content)

	return &Response{
		Content: content,
		Model:   c.model,
	}, nil
}

// stripMarkdownCodeBlock removes ```json or ``` wrappers from content.
func stripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)

	// Check for ```json or ```JSON at the start
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```JSON") {
		s = strings.TrimPrefix(s, "```JSON")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}

	// Remove trailing ```
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}

	return strings.TrimSpace(s)
}

const geminiModelsEndpoint = "https://generativelanguage.googleapis.com/v1beta/models"

// GeminiModelsResponse represents the response from the models endpoint.
type GeminiModelsResponse struct {
	Models []struct {
		Name                       string   `json:"name"`
		DisplayName                string   `json:"displayName"`
		Description                string   `json:"description"`
		SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	} `json:"models"`
	NextPageToken string `json:"nextPageToken"`
}

// FetchGeminiModels fetches available models from the Google Gemini API.
func FetchGeminiModels(apiKey string) ([]ModelInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	url := geminiModelsEndpoint + "?key=" + apiKey
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch models failed: %s", string(body[:min(200, len(body))]))
	}

	var modelsResp GeminiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, 0)
	for _, m := range modelsResp.Models {
		// Only include models that support generateContent
		supportsGenerate := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}
		if !supportsGenerate {
			continue
		}

		// Extract model ID from name (format: "models/gemini-2.0-flash")
		modelID := strings.TrimPrefix(m.Name, "models/")

		// Filter to gemini models only
		if !strings.HasPrefix(modelID, "gemini-") {
			continue
		}

		models = append(models, ModelInfo{
			ID:       modelID,
			Name:     m.DisplayName,
			Provider: ProviderGoogle,
		})
	}

	return models, nil
}
