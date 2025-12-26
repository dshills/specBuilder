package repository

import (
	"context"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/google/uuid"
)

// Repository defines the interface for persistent storage.
type Repository interface {
	// Projects
	CreateProject(ctx context.Context, project *domain.Project) error
	GetProject(ctx context.Context, id uuid.UUID) (*domain.Project, error)
	ListProjects(ctx context.Context) ([]*domain.Project, error)
	UpdateProject(ctx context.Context, project *domain.Project) error
	DeleteProject(ctx context.Context, id uuid.UUID) error
	GetLatestSnapshotID(ctx context.Context, projectID uuid.UUID) (*uuid.UUID, error)

	// Questions
	CreateQuestion(ctx context.Context, question *domain.Question) error
	GetQuestion(ctx context.Context, id uuid.UUID) (*domain.Question, error)
	ListQuestions(ctx context.Context, projectID uuid.UUID, status *domain.QuestionStatus, tag *string) ([]*domain.Question, error)
	UpdateQuestionStatus(ctx context.Context, id uuid.UUID, status domain.QuestionStatus) error

	// Answers
	CreateAnswer(ctx context.Context, answer *domain.Answer) error
	GetAnswer(ctx context.Context, id uuid.UUID) (*domain.Answer, error)
	GetLatestAnswer(ctx context.Context, questionID uuid.UUID) (*domain.Answer, error)
	GetAnswerByVersion(ctx context.Context, questionID uuid.UUID, version int) (*domain.Answer, error)
	ListAnswers(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error)
	GetLatestAnswersForProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error)

	// Snapshots
	CreateSnapshot(ctx context.Context, snapshot *domain.SpecSnapshot) error
	GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.SpecSnapshot, error)
	ListSnapshots(ctx context.Context, projectID uuid.UUID, limit int) ([]*domain.SpecSnapshot, error)

	// Issues
	CreateIssue(ctx context.Context, issue *domain.Issue) error
	ListIssuesForSnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*domain.Issue, error)

	// Transaction support
	WithTx(ctx context.Context, fn func(Repository) error) error

	// Lifecycle
	Close() error
}
