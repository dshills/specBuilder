package compiler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/llm"
	"github.com/dshills/specbuilder/backend/internal/validator"
	"github.com/google/uuid"
)

func setupCompilerService(t *testing.T, mockResponse string) *Service {
	t.Helper()
	mockClient := llm.NewMockClient(mockResponse)
	val, err := validator.New()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	return NewService(mockClient, val, `{"type": "object"}`)
}

// testContext returns a context with timeout for tests.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestCompile(t *testing.T) {
	// Mock LLM response matching expected compiler output format
	mockResponse := `{
		"spec": {
			"product": {"name": "Test Product", "purpose": "Testing"},
			"scope": {"in_scope": ["Testing"], "out_of_scope": []}
		},
		"trace": {"product.name": {"question_id": "q1", "answer_id": "a1"}}
	}`

	service := setupCompilerService(t, mockResponse)
	ctx := testContext(t)

	now := time.Now().UTC()
	project := &domain.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		CreatedAt: now,
		UpdatedAt: now,
	}

	qaBundles := []QABundle{
		{
			QuestionID:    uuid.New(),
			QuestionText:  "What is the product name?",
			QuestionType:  "freeform",
			QuestionTags:  []string{"seed"},
			QuestionPaths: []string{"/product/name"},
			AnswerID:      uuid.New(),
			AnswerValue:   json.RawMessage(`"Test Product"`),
			AnswerVersion: 1,
		},
	}

	input := CompileInput{
		Project:     project,
		QABundles:   qaBundles,
		CurrentSpec: nil,
	}

	output, err := service.Compile(ctx, input)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if output == nil {
		t.Fatal("Compile() output is nil")
	}

	if len(output.Spec) == 0 {
		t.Error("Compile() spec is empty")
	}

	if output.Compiler.Model != "mock-model" {
		t.Errorf("Compile() model = %s, want mock-model", output.Compiler.Model)
	}

	// Verify derived_from is populated
	if len(output.DerivedFrom) != 1 {
		t.Errorf("Compile() DerivedFrom len = %d, want 1", len(output.DerivedFrom))
	}
}

func TestCompileWithCurrentSpec(t *testing.T) {
	mockResponse := `{
		"spec": {"product": {"name": "Updated"}, "scope": {}},
		"trace": {}
	}`

	service := setupCompilerService(t, mockResponse)
	ctx := testContext(t)

	project := &domain.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
	}

	currentSpec := json.RawMessage(`{"product": {"name": "Original"}}`)

	input := CompileInput{
		Project:     project,
		QABundles:   []QABundle{},
		CurrentSpec: currentSpec,
	}

	output, err := service.Compile(ctx, input)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if output == nil {
		t.Fatal("Compile() output is nil")
	}
}

func TestCompileLLMError(t *testing.T) {
	mockClient := llm.NewMockClient("")
	mockClient.Error = llm.ErrProviderError
	val, err := validator.New()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	service := NewService(mockClient, val, `{}`)
	ctx := testContext(t)

	project := &domain.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
	}

	input := CompileInput{
		Project:   project,
		QABundles: []QABundle{},
	}

	_, err = service.Compile(ctx, input)
	if err == nil {
		t.Error("Compile() expected error for LLM failure")
	}
}

func TestCompileInvalidJSON(t *testing.T) {
	mockResponse := `not valid json`
	service := setupCompilerService(t, mockResponse)
	ctx := testContext(t)

	project := &domain.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
	}

	input := CompileInput{
		Project:   project,
		QABundles: []QABundle{},
	}

	_, err := service.Compile(ctx, input)
	if err == nil {
		t.Error("Compile() expected error for invalid JSON response")
	}
}

func TestValidate(t *testing.T) {
	mockResponse := `{
		"issues": [
			{
				"type": "missing",
				"severity": "warning",
				"message": "Missing data model details",
				"related_spec_paths": ["/data_model"],
				"related_question_ids": []
			}
		]
	}`

	service := setupCompilerService(t, mockResponse)
	ctx := testContext(t)

	project := &domain.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
	}

	spec := json.RawMessage(`{"product": {"name": "Test"}}`)
	trace := json.RawMessage(`{}`)

	issues, err := service.Validate(ctx, project, spec, trace, nil)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("Validate() issues count = %d, want 1", len(issues))
	}

	if issues[0].Type != "missing" {
		t.Errorf("Validate() issue type = %s, want missing", issues[0].Type)
	}
}

func TestHydrateIssues(t *testing.T) {
	projectID := uuid.New()
	snapshotID := uuid.New()
	questionID := uuid.New()

	drafts := []domain.IssueDraft{
		{
			Type:               "conflict",
			Severity:           "error",
			Message:            "Test conflict",
			RelatedSpecPaths:   []string{"/api/endpoints"},
			RelatedQuestionIDs: []string{questionID.String()},
		},
		{
			Type:               "assumption",
			Severity:           "info",
			Message:            "Test assumption",
			RelatedSpecPaths:   []string{"/scope"},
			RelatedQuestionIDs: []string{},
		},
	}

	issues := HydrateIssues(drafts, projectID, snapshotID)

	if len(issues) != 2 {
		t.Fatalf("HydrateIssues() count = %d, want 2", len(issues))
	}

	// Check first issue
	if issues[0].ProjectID != projectID {
		t.Error("HydrateIssues() project ID mismatch")
	}
	if issues[0].SnapshotID != snapshotID {
		t.Error("HydrateIssues() snapshot ID mismatch")
	}
	if issues[0].Type != "conflict" {
		t.Errorf("HydrateIssues() type = %s, want conflict", issues[0].Type)
	}
	if len(issues[0].RelatedQuestionIDs) != 1 {
		t.Errorf("HydrateIssues() related questions = %d, want 1", len(issues[0].RelatedQuestionIDs))
	}

	// Check second issue
	if issues[1].Type != "assumption" {
		t.Errorf("HydrateIssues() type = %s, want assumption", issues[1].Type)
	}
}
