package llm

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed prompts/v1/*.txt
var promptsFS embed.FS

// PromptVersion represents a prompt version.
type PromptVersion string

const (
	PromptVersionV1 PromptVersion = "v1"
)

// PromptTemplate holds a loaded prompt template.
type PromptTemplate struct {
	Version  PromptVersion
	Role     string // planner, asker, compiler, validator
	Template string
}

// LoadPrompt loads a prompt template by role and version.
func LoadPrompt(role string, version PromptVersion) (*PromptTemplate, error) {
	filename := fmt.Sprintf("prompts/%s/%s.txt", version, role)
	data, err := promptsFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("load prompt %s/%s: %w", version, role, err)
	}
	return &PromptTemplate{
		Version:  version,
		Role:     role,
		Template: string(data),
	}, nil
}

// Render renders the template with the given variables.
func (p *PromptTemplate) Render(vars map[string]string) string {
	result := p.Template
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		result = strings.ReplaceAll(result, placeholder, v)
	}
	return result
}
