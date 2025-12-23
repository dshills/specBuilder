package compiler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/llm"
	"github.com/dshills/specbuilder/backend/internal/validator"
	"github.com/google/uuid"
)

func TestPlan(t *testing.T) {
	mockResponse := `{
		"rationale": "The spec is missing data model details",
		"targets": [
			{
				"spec_paths": ["/data_model"],
				"gap_type": "missing",
				"why_now": "Core entity definitions needed",
				"suggested_question_count": 2
			}
		],
		"suggestions": [
			{
				"key": "data_entities",
				"spec_paths": ["/data_model/entities"],
				"question_intent": "Identify main data entities",
				"recommended_type": "freeform",
				"recommended_options": [],
				"priority": 100,
				"tags": ["data_model"]
			}
		]
	}`

	mockClient := llm.NewMockClient(mockResponse)
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

	input := PlanInput{
		Project:           project,
		CurrentSpec:       json.RawMessage(`{"product": {"name": "Test"}}`),
		CurrentIssues:     nil,
		ExistingQuestions: nil,
		LatestAnswers:     nil,
	}

	output, err := service.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if output == nil {
		t.Fatal("Plan() output is nil")
	}

	if output.Rationale == "" {
		t.Error("Plan() rationale is empty")
	}

	if len(output.Targets) != 1 {
		t.Fatalf("Plan() targets count = %d, want 1", len(output.Targets))
	}

	if len(output.Suggestions) != 1 {
		t.Fatalf("Plan() suggestions count = %d, want 1", len(output.Suggestions))
	}

	suggestion := output.Suggestions[0]
	if suggestion.Key != "data_entities" {
		t.Errorf("Plan() suggestion key = %s, want data_entities", suggestion.Key)
	}
	if suggestion.RecommendedType != "freeform" {
		t.Errorf("Plan() suggestion type = %s, want freeform", suggestion.RecommendedType)
	}
}

func TestPlanWithExistingContext(t *testing.T) {
	mockResponse := `{
		"rationale": "Building on existing answers",
		"targets": [],
		"suggestions": []
	}`

	mockClient := llm.NewMockClient(mockResponse)
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

	existingQuestions := []*domain.Question{
		{
			ID:        uuid.New(),
			ProjectID: project.ID,
			Text:      "What is the product?",
			Type:      domain.QuestionTypeFreeform,
			Status:    domain.QuestionStatusAnswered,
		},
	}

	latestAnswers := []*domain.Answer{
		{
			ID:         uuid.New(),
			ProjectID:  project.ID,
			QuestionID: existingQuestions[0].ID,
			Value:      json.RawMessage(`"A testing tool"`),
			Version:    1,
		},
	}

	input := PlanInput{
		Project:           project,
		CurrentSpec:       json.RawMessage(`{"product": {"name": "Test"}}`),
		CurrentIssues:     nil,
		ExistingQuestions: existingQuestions,
		LatestAnswers:     latestAnswers,
	}

	output, err := service.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if output == nil {
		t.Fatal("Plan() output is nil")
	}

	// Verify the mock client received the request
	if mockClient.CallCount != 1 {
		t.Errorf("Plan() call count = %d, want 1", mockClient.CallCount)
	}
}

func TestPlanInvalidJSON(t *testing.T) {
	mockClient := llm.NewMockClient(`not valid json`)
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

	input := PlanInput{
		Project:     project,
		CurrentSpec: nil,
	}

	_, err = service.Plan(ctx, input)
	if err == nil {
		t.Error("Plan() expected error for invalid JSON")
	}
}

func TestAsk(t *testing.T) {
	mockResponse := `{
		"questions": [
			{
				"text": "What are the main data entities?",
				"type": "freeform",
				"options": [],
				"tags": ["data_model"],
				"priority": 100,
				"spec_paths": ["/data_model/entities"]
			},
			{
				"text": "Which database type?",
				"type": "single",
				"options": ["PostgreSQL", "MySQL", "SQLite"],
				"tags": ["infrastructure"],
				"priority": 90,
				"spec_paths": ["/infrastructure/database"]
			}
		]
	}`

	mockClient := llm.NewMockClient(mockResponse)
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

	suggestions := []PlannerSuggestion{
		{
			Key:                "data_entities",
			SpecPaths:          []string{"/data_model/entities"},
			QuestionIntent:     "Identify entities",
			RecommendedType:    "freeform",
			RecommendedOptions: nil,
			Priority:           100,
			Tags:               []string{"data_model"},
		},
	}

	input := AskInput{
		Project:            project,
		PlannerSuggestions: suggestions,
		CurrentSpec:        json.RawMessage(`{}`),
		ExistingQuestions:  nil,
		LatestAnswers:      nil,
	}

	output, err := service.Ask(ctx, input)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}

	if output == nil {
		t.Fatal("Ask() output is nil")
	}

	if len(output.Questions) != 2 {
		t.Fatalf("Ask() questions count = %d, want 2", len(output.Questions))
	}

	// Check first question
	q1 := output.Questions[0]
	if q1.Type != "freeform" {
		t.Errorf("Ask() q1 type = %s, want freeform", q1.Type)
	}

	// Check second question
	q2 := output.Questions[1]
	if q2.Type != "single" {
		t.Errorf("Ask() q2 type = %s, want single", q2.Type)
	}
	if len(q2.Options) != 3 {
		t.Errorf("Ask() q2 options count = %d, want 3", len(q2.Options))
	}
}

func TestAskEmptyOutput(t *testing.T) {
	mockResponse := `{"questions": []}`

	mockClient := llm.NewMockClient(mockResponse)
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

	input := AskInput{
		Project:            project,
		PlannerSuggestions: nil,
		CurrentSpec:        nil,
	}

	output, err := service.Ask(ctx, input)
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}

	if len(output.Questions) != 0 {
		t.Errorf("Ask() questions count = %d, want 0", len(output.Questions))
	}
}

func TestAskLLMError(t *testing.T) {
	mockClient := llm.NewMockClient("")
	mockClient.Error = llm.ErrRateLimit
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

	input := AskInput{
		Project: project,
	}

	_, err = service.Ask(ctx, input)
	if err == nil {
		t.Fatal("Ask() expected error for LLM failure")
	}
}
