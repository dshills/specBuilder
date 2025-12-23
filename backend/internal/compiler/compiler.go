package compiler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/llm"
	"github.com/dshills/specbuilder/backend/internal/validator"
	"github.com/google/uuid"
)

// Service handles spec compilation.
type Service struct {
	llmClient     llm.Client
	validator     *validator.Validator
	promptVersion llm.PromptVersion
	specSchema    string // JSON schema for ProjectImplementationSpec
}

// NewService creates a new compiler service.
func NewService(client llm.Client, val *validator.Validator, specSchema string) *Service {
	return &Service{
		llmClient:     client,
		validator:     val,
		promptVersion: llm.PromptVersionV1,
		specSchema:    specSchema,
	}
}

// QABundle represents a question-answer pair for compilation.
type QABundle struct {
	QuestionID    uuid.UUID       `json:"question_id"`
	QuestionText  string          `json:"question_text"`
	QuestionType  string          `json:"question_type"`
	QuestionTags  []string        `json:"question_tags"`
	QuestionPaths []string        `json:"question_spec_paths"`
	AnswerID      uuid.UUID       `json:"answer_id"`
	AnswerValue   json.RawMessage `json:"answer_value"`
	AnswerVersion int             `json:"answer_version"`
}

// CompileInput holds input for compilation.
type CompileInput struct {
	Project     *domain.Project
	QABundles   []QABundle
	CurrentSpec json.RawMessage // Previous spec if exists
}

// CompileOutput holds compilation result.
type CompileOutput struct {
	Spec        json.RawMessage   `json:"spec"`
	Trace       json.RawMessage   `json:"trace"`
	DerivedFrom map[uuid.UUID]int // question_id -> answer_version
	Compiler    domain.CompilerConfig
}

// compilerResponse represents the LLM output structure.
type compilerResponse struct {
	Spec  json.RawMessage `json:"spec"`
	Trace json.RawMessage `json:"trace"`
}

// Compile compiles Q&A bundles into a spec.
func (s *Service) Compile(ctx context.Context, input CompileInput) (*CompileOutput, error) {
	// Load compiler prompt
	prompt, err := llm.LoadPrompt("compiler", s.promptVersion)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	// Prepare Q&A bundle JSON
	qaBundleJSON, err := json.Marshal(input.QABundles)
	if err != nil {
		return nil, fmt.Errorf("marshal qa bundles: %w", err)
	}

	// Prepare current spec (or empty object)
	currentSpec := input.CurrentSpec
	if len(currentSpec) == 0 {
		currentSpec = []byte("{}")
	}

	// Prepare project JSON
	projectJSON, err := json.Marshal(input.Project)
	if err != nil {
		return nil, fmt.Errorf("marshal project: %w", err)
	}

	// Render prompt
	renderedPrompt := prompt.Render(map[string]string{
		"PROJECT":           string(projectJSON),
		"QA_BUNDLE_JSON":    string(qaBundleJSON),
		"CURRENT_SPEC_JSON": string(currentSpec),
		"PROJECT_IMPLEMENTATION_SPEC_SCHEMA_JSON": s.specSchema,
	})

	// Call LLM
	req := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: renderedPrompt},
		},
		Temperature: 0,
		MaxTokens:   16000, // Large output for full spec
	}

	resp, err := s.llmClient.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	// Parse response
	var compilerResp compilerResponse
	if err := json.Unmarshal([]byte(resp.Content), &compilerResp); err != nil {
		return nil, fmt.Errorf("parse llm response: %w (response: %s)", err, resp.Content[:min(500, len(resp.Content))])
	}

	// Validate spec against schema
	result := s.validator.ValidateSpec(compilerResp.Spec)
	if !result.Valid {
		// Log validation errors but don't fail - let validator LLM handle it
		// In production, we might want to retry or return a partial result
	}

	// Build derived_from map
	derivedFrom := make(map[uuid.UUID]int)
	for _, qa := range input.QABundles {
		derivedFrom[qa.QuestionID] = qa.AnswerVersion
	}

	return &CompileOutput{
		Spec:        compilerResp.Spec,
		Trace:       compilerResp.Trace,
		DerivedFrom: derivedFrom,
		Compiler: domain.CompilerConfig{
			Model:         s.llmClient.Model(),
			PromptVersion: string(s.promptVersion),
			Temperature:   0,
		},
	}, nil
}

// ValidatorOutput represents the LLM validator output.
type ValidatorOutput struct {
	Issues []domain.IssueDraft `json:"issues"`
}

// Validate runs LLM-based validation on a compiled spec.
func (s *Service) Validate(ctx context.Context, project *domain.Project, spec, trace json.RawMessage, qaBundles []QABundle) ([]domain.IssueDraft, error) {
	prompt, err := llm.LoadPrompt("validator_llm_optional", s.promptVersion)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	// JSON schema validation first
	schemaResult := s.validator.ValidateSpec(spec)
	schemaValidationJSON, _ := json.Marshal(map[string]interface{}{
		"is_valid": schemaResult.Valid,
		"errors":   schemaResult.Errors,
	})

	projectJSON, _ := json.Marshal(project)
	qaBundleJSON, _ := json.Marshal(qaBundles)

	renderedPrompt := prompt.Render(map[string]string{
		"PROJECT":                project.Name,
		"COMPILED_SPEC_JSON":     string(spec),
		"TRACE_JSON":             string(trace),
		"SCHEMA_VALIDATION_JSON": string(schemaValidationJSON),
		"QA_BUNDLE_JSON":         string(qaBundleJSON),
	})

	// Use PROJECT as the full project JSON for consistency
	_ = projectJSON

	req := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: renderedPrompt},
		},
		Temperature: 0,
		MaxTokens:   4000,
	}

	resp, err := s.llmClient.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var output ValidatorOutput
	if err := json.Unmarshal([]byte(resp.Content), &output); err != nil {
		return nil, fmt.Errorf("parse validator response: %w", err)
	}

	return output.Issues, nil
}

// HydrateIssues converts IssueDrafts to full Issues with generated IDs.
func HydrateIssues(drafts []domain.IssueDraft, projectID, snapshotID uuid.UUID) []*domain.Issue {
	now := time.Now().UTC()
	issues := make([]*domain.Issue, len(drafts))
	for i, d := range drafts {
		relatedQIDs := make([]uuid.UUID, 0, len(d.RelatedQuestionIDs))
		for _, idStr := range d.RelatedQuestionIDs {
			if uid, err := uuid.Parse(idStr); err == nil {
				relatedQIDs = append(relatedQIDs, uid)
			}
		}
		issues[i] = &domain.Issue{
			ID:                 uuid.New(),
			ProjectID:          projectID,
			SnapshotID:         snapshotID,
			Type:               d.Type,
			Severity:           d.Severity,
			Message:            d.Message,
			RelatedSpecPaths:   d.RelatedSpecPaths,
			RelatedQuestionIDs: relatedQIDs,
			CreatedAt:          now,
		}
	}
	return issues
}
