package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// QuestionType represents the type of question.
type QuestionType string

const (
	QuestionTypeSingle   QuestionType = "single"
	QuestionTypeMulti    QuestionType = "multi"
	QuestionTypeFreeform QuestionType = "freeform"
)

// QuestionStatus represents the status of a question.
type QuestionStatus string

const (
	QuestionStatusUnanswered  QuestionStatus = "unanswered"
	QuestionStatusAnswered    QuestionStatus = "answered"
	QuestionStatusNeedsReview QuestionStatus = "needs_review"
)

// IssueType represents the type of issue.
type IssueType string

const (
	IssueTypeConflict   IssueType = "conflict"
	IssueTypeMissing    IssueType = "missing"
	IssueTypeAssumption IssueType = "assumption"
)

// IssueSeverity represents the severity of an issue.
type IssueSeverity string

const (
	IssueSeverityInfo  IssueSeverity = "info"
	IssueSeverityWarn  IssueSeverity = "warn"
	IssueSeverityError IssueSeverity = "error"
)

// CompileMode represents the compilation mode.
type CompileMode string

const (
	CompileModeLatestAnswers          CompileMode = "latest_answers"
	CompileModeSpecificAnswerVersions CompileMode = "specific_answer_versions"
)

// Project represents a specification project.
type Project struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Question represents a question in a project.
type Question struct {
	ID        uuid.UUID      `json:"id"`
	ProjectID uuid.UUID      `json:"project_id"`
	Text      string         `json:"text"`
	Type      QuestionType   `json:"type"`
	Options   []string       `json:"options"` // nil for freeform, non-nil for single/multi
	Tags      []string       `json:"tags"`
	Priority  int            `json:"priority"`
	SpecPaths []string       `json:"spec_paths"`
	Status    QuestionStatus `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
}

// Answer represents an answer to a question.
// Answers are immutable; edits create new versions.
type Answer struct {
	ID         uuid.UUID       `json:"id"`
	ProjectID  uuid.UUID       `json:"project_id"`
	QuestionID uuid.UUID       `json:"question_id"`
	Value      json.RawMessage `json:"value"` // Any JSON value
	Version    int             `json:"version"`
	Supersedes *uuid.UUID      `json:"supersedes"` // nil for first version
	CreatedAt  time.Time       `json:"created_at"`
}

// CompilerConfig holds the configuration used during compilation.
type CompilerConfig struct {
	Model         string  `json:"model"`
	PromptVersion string  `json:"prompt_version"`
	Temperature   float64 `json:"temperature"`
	Seed          *int    `json:"seed,omitempty"`
}

// SpecSnapshot represents an immutable compiled specification snapshot.
type SpecSnapshot struct {
	ID          uuid.UUID         `json:"id"`
	ProjectID   uuid.UUID         `json:"project_id"`
	Spec        json.RawMessage   `json:"spec"` // ProjectImplementationSpec JSON
	CreatedAt   time.Time         `json:"created_at"`
	DerivedFrom map[uuid.UUID]int `json:"derived_from"` // question_id -> answer_version
	Compiler    CompilerConfig    `json:"compiler"`
}

// Issue represents a validation issue for a snapshot.
type Issue struct {
	ID                 uuid.UUID     `json:"id"`
	ProjectID          uuid.UUID     `json:"project_id"`
	SnapshotID         uuid.UUID     `json:"snapshot_id"`
	Type               IssueType     `json:"type"`
	Severity           IssueSeverity `json:"severity"`
	Message            string        `json:"message"`
	RelatedSpecPaths   []string      `json:"related_spec_paths"`
	RelatedQuestionIDs []uuid.UUID   `json:"related_question_ids"`
	CreatedAt          time.Time     `json:"created_at"`
}

// IssueDraft is an issue without server-assigned fields (used by LLM validator).
type IssueDraft struct {
	Type               IssueType     `json:"type"`
	Severity           IssueSeverity `json:"severity"`
	Message            string        `json:"message"`
	RelatedSpecPaths   []string      `json:"related_spec_paths"`
	RelatedQuestionIDs []string      `json:"related_question_ids"` // string UUIDs from LLM
}
