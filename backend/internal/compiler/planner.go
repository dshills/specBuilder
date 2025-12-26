package compiler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/llm"
)

// PlannerOutput represents the planner LLM output.
type PlannerOutput struct {
	Rationale   string              `json:"rationale"`
	Targets     []PlannerTarget     `json:"targets"`
	Suggestions []PlannerSuggestion `json:"suggestions"`
}

// PlannerTarget represents a gap to fill.
type PlannerTarget struct {
	SpecPaths              []string `json:"spec_paths"`
	GapType                string   `json:"gap_type"` // missing, conflict, assumption, uncertainty
	WhyNow                 string   `json:"why_now"`
	SuggestedQuestionCount int      `json:"suggested_question_count"`
}

// PlannerSuggestion represents a suggested question.
type PlannerSuggestion struct {
	Key                string   `json:"key"`
	SpecPaths          []string `json:"spec_paths"`
	QuestionIntent     string   `json:"question_intent"`
	RecommendedType    string   `json:"recommended_type"` // single, multi, freeform
	RecommendedOptions []string `json:"recommended_options"`
	Priority           int      `json:"priority"`
	Tags               []string `json:"tags"`
}

// QuestionMode represents the question complexity level.
type QuestionMode string

const (
	// ModeBasic generates simpler questions for non-programmers
	ModeBasic QuestionMode = "basic"
	// ModeAdvanced generates technical questions for developers
	ModeAdvanced QuestionMode = "advanced"
)

// PlanInput holds input for planning.
type PlanInput struct {
	Project           *domain.Project
	CurrentSpec       json.RawMessage
	CurrentIssues     []*domain.Issue
	ExistingQuestions []*domain.Question
	LatestAnswers     []*domain.Answer
	Mode              QuestionMode // basic or advanced
}

// Plan runs the planner to determine next questions.
func (s *Service) Plan(ctx context.Context, input PlanInput) (*PlannerOutput, error) {
	llmClient, err := s.factory.CreateDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	// Select prompt based on mode
	promptName := "planner"
	if input.Mode == ModeBasic {
		promptName = "planner_basic"
	}
	log.Printf("Plan: using prompt %s (mode=%s)", promptName, input.Mode)

	prompt, err := llm.LoadPrompt(promptName, s.promptVersion)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	projectJSON, _ := json.Marshal(input.Project)
	currentSpec := input.CurrentSpec
	if len(currentSpec) == 0 {
		currentSpec = []byte("{}")
	}
	issuesJSON, _ := json.Marshal(input.CurrentIssues)
	questionsJSON, _ := json.Marshal(input.ExistingQuestions)
	answersJSON, _ := json.Marshal(input.LatestAnswers)

	renderedPrompt := prompt.Render(map[string]string{
		"PROJECT":                 string(projectJSON),
		"CURRENT_SPEC_JSON":       string(currentSpec),
		"CURRENT_ISSUES_JSON":     string(issuesJSON),
		"EXISTING_QUESTIONS_JSON": string(questionsJSON),
		"LATEST_ANSWERS_JSON":     string(answersJSON),
	})

	req := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: renderedPrompt},
		},
		Temperature: 0,
		MaxTokens:   4000,
	}

	resp, err := llmClient.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var output PlannerOutput
	if err := json.Unmarshal([]byte(resp.Content), &output); err != nil {
		return nil, fmt.Errorf("parse planner response: %w", err)
	}

	return &output, nil
}

// AskerOutput represents the asker LLM output.
type AskerOutput struct {
	Questions []AskerQuestion `json:"questions"`
}

// AskerQuestion represents a generated question.
type AskerQuestion struct {
	Text      string   `json:"text"`
	Type      string   `json:"type"` // single, multi, freeform
	Options   []string `json:"options"`
	Tags      []string `json:"tags"`
	Priority  int      `json:"priority"`
	SpecPaths []string `json:"spec_paths"`
}

// AskInput holds input for question generation.
type AskInput struct {
	Project            *domain.Project
	PlannerSuggestions []PlannerSuggestion
	CurrentSpec        json.RawMessage
	ExistingQuestions  []*domain.Question
	LatestAnswers      []*domain.Answer
	Mode               QuestionMode // basic or advanced
}

// Ask generates questions based on planner suggestions.
func (s *Service) Ask(ctx context.Context, input AskInput) (*AskerOutput, error) {
	llmClient, err := s.factory.CreateDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	// Select prompt based on mode
	promptName := "asker"
	if input.Mode == ModeBasic {
		promptName = "asker_basic"
	}

	prompt, err := llm.LoadPrompt(promptName, s.promptVersion)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	projectJSON, _ := json.Marshal(input.Project)
	suggestionsJSON, _ := json.Marshal(input.PlannerSuggestions)
	currentSpec := input.CurrentSpec
	if len(currentSpec) == 0 {
		currentSpec = []byte("{}")
	}
	questionsJSON, _ := json.Marshal(input.ExistingQuestions)
	answersJSON, _ := json.Marshal(input.LatestAnswers)

	renderedPrompt := prompt.Render(map[string]string{
		"PROJECT":                  string(projectJSON),
		"PLANNER_SUGGESTIONS_JSON": string(suggestionsJSON),
		"CURRENT_SPEC_JSON":        string(currentSpec),
		"EXISTING_QUESTIONS_JSON":  string(questionsJSON),
		"LATEST_ANSWERS_JSON":      string(answersJSON),
	})

	req := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: renderedPrompt},
		},
		Temperature: 0,
		MaxTokens:   4000,
	}

	resp, err := llmClient.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var output AskerOutput
	if err := json.Unmarshal([]byte(resp.Content), &output); err != nil {
		return nil, fmt.Errorf("parse asker response: %w", err)
	}

	return &output, nil
}

// SuggesterOutput represents the suggester LLM output.
type SuggesterOutput struct {
	Suggestions []SuggesterSuggestion `json:"suggestions"`
}

// SuggesterSuggestion represents a suggested answer from the LLM.
type SuggesterSuggestion struct {
	QuestionID     string          `json:"question_id"`
	SuggestedValue json.RawMessage `json:"suggested_value"`
	Confidence     string          `json:"confidence"` // high, medium, low
	Reasoning      string          `json:"reasoning"`
}

// SuggestInput holds input for answer suggestion.
type SuggestInput struct {
	Project             *domain.Project
	UnansweredQuestions []*domain.Question
	LatestAnswers       []*domain.Answer
	CurrentSpec         json.RawMessage
	Mode                QuestionMode // basic or advanced
}

// Suggest generates suggested answers for unanswered questions.
func (s *Service) Suggest(ctx context.Context, input SuggestInput) (*SuggesterOutput, error) {
	if len(input.UnansweredQuestions) == 0 {
		return &SuggesterOutput{Suggestions: []SuggesterSuggestion{}}, nil
	}

	llmClient, err := s.factory.CreateDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	// Select prompt based on mode
	promptName := "suggester"
	if input.Mode == ModeBasic {
		promptName = "suggester_basic"
	}
	log.Printf("Suggest: using prompt %s (mode=%s)", promptName, input.Mode)

	prompt, err := llm.LoadPrompt(promptName, s.promptVersion)
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	// Format existing answers for the prompt
	answersText := "No answers yet."
	if len(input.LatestAnswers) > 0 {
		answersJSON, _ := json.MarshalIndent(input.LatestAnswers, "", "  ")
		answersText = string(answersJSON)
	}

	// Format unanswered questions for the prompt
	type questionForPrompt struct {
		ID       string   `json:"id"`
		Text     string   `json:"text"`
		Type     string   `json:"type"`
		Options  []string `json:"options,omitempty"`
		SpecPath string   `json:"spec_path,omitempty"`
	}
	questionsForPrompt := make([]questionForPrompt, len(input.UnansweredQuestions))
	for i, q := range input.UnansweredQuestions {
		specPath := ""
		if len(q.SpecPaths) > 0 {
			specPath = q.SpecPaths[0]
		}
		questionsForPrompt[i] = questionForPrompt{
			ID:       q.ID.String(),
			Text:     q.Text,
			Type:     string(q.Type),
			Options:  q.Options,
			SpecPath: specPath,
		}
	}
	questionsJSON, _ := json.MarshalIndent(questionsForPrompt, "", "  ")

	currentSpec := input.CurrentSpec
	if len(currentSpec) == 0 {
		currentSpec = []byte("{}")
	}

	modeStr := "advanced"
	if input.Mode == ModeBasic {
		modeStr = "basic"
	}

	renderedPrompt := prompt.Render(map[string]string{
		"PROJECT_NAME":         input.Project.Name,
		"PROJECT_MODE":         modeStr,
		"EXISTING_ANSWERS":     answersText,
		"CURRENT_SPEC":         string(currentSpec),
		"UNANSWERED_QUESTIONS": string(questionsJSON),
	})

	req := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: renderedPrompt},
		},
		Temperature: 0.3, // Slightly higher temp for more creative suggestions
		MaxTokens:   4000,
	}

	resp, err := llmClient.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var output SuggesterOutput
	if err := json.Unmarshal([]byte(resp.Content), &output); err != nil {
		return nil, fmt.Errorf("parse suggester response: %w", err)
	}

	return &output, nil
}
