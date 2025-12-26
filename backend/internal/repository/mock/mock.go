package mock

import (
	"context"
	"sync"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/repository"
	"github.com/google/uuid"
)

// Repository is an in-memory mock repository for testing.
type Repository struct {
	mu        sync.RWMutex
	projects  map[uuid.UUID]*domain.Project
	questions map[uuid.UUID]*domain.Question
	answers   map[uuid.UUID]*domain.Answer
	snapshots map[uuid.UUID]*domain.SpecSnapshot
	issues    map[uuid.UUID]*domain.Issue
	closed    bool
}

// New creates a new mock repository.
func New() *Repository {
	return &Repository{
		projects:  make(map[uuid.UUID]*domain.Project),
		questions: make(map[uuid.UUID]*domain.Question),
		answers:   make(map[uuid.UUID]*domain.Answer),
		snapshots: make(map[uuid.UUID]*domain.SpecSnapshot),
		issues:    make(map[uuid.UUID]*domain.Issue),
	}
}

// Projects

func (r *Repository) CreateProject(ctx context.Context, project *domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projects[project.ID] = project
	return nil
}

func (r *Repository) GetProject(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.projects[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (r *Repository) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Project
	for _, p := range r.projects {
		result = append(result, p)
	}
	return result, nil
}

func (r *Repository) DeleteProject(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.projects[id]; !ok {
		return domain.ErrNotFound
	}
	// Delete related data
	for issueID, issue := range r.issues {
		if issue.ProjectID == id {
			delete(r.issues, issueID)
		}
	}
	for snapID, snap := range r.snapshots {
		if snap.ProjectID == id {
			delete(r.snapshots, snapID)
		}
	}
	for ansID, ans := range r.answers {
		if ans.ProjectID == id {
			delete(r.answers, ansID)
		}
	}
	for qID, q := range r.questions {
		if q.ProjectID == id {
			delete(r.questions, qID)
		}
	}
	delete(r.projects, id)
	return nil
}

func (r *Repository) UpdateProject(ctx context.Context, project *domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.projects[project.ID]; !ok {
		return domain.ErrNotFound
	}
	r.projects[project.ID] = project
	return nil
}

func (r *Repository) GetLatestSnapshotID(ctx context.Context, projectID uuid.UUID) (*uuid.UUID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var latest *domain.SpecSnapshot
	for _, s := range r.snapshots {
		if s.ProjectID == projectID {
			if latest == nil || s.CreatedAt.After(latest.CreatedAt) {
				latest = s
			}
		}
	}
	if latest == nil {
		return nil, nil
	}
	return &latest.ID, nil
}

// Questions

func (r *Repository) CreateQuestion(ctx context.Context, question *domain.Question) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.questions[question.ID] = question
	return nil
}

func (r *Repository) GetQuestion(ctx context.Context, id uuid.UUID) (*domain.Question, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	q, ok := r.questions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return q, nil
}

func (r *Repository) ListQuestions(ctx context.Context, projectID uuid.UUID, status *domain.QuestionStatus, tag *string) ([]*domain.Question, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Question
	for _, q := range r.questions {
		if q.ProjectID != projectID {
			continue
		}
		if status != nil && q.Status != *status {
			continue
		}
		if tag != nil {
			found := false
			for _, t := range q.Tags {
				if t == *tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, q)
	}
	return result, nil
}

func (r *Repository) UpdateQuestionStatus(ctx context.Context, id uuid.UUID, status domain.QuestionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	q, ok := r.questions[id]
	if !ok {
		return domain.ErrNotFound
	}
	q.Status = status
	return nil
}

// Answers

func (r *Repository) CreateAnswer(ctx context.Context, answer *domain.Answer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.answers[answer.ID] = answer
	return nil
}

func (r *Repository) GetAnswer(ctx context.Context, id uuid.UUID) (*domain.Answer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.answers[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (r *Repository) GetLatestAnswer(ctx context.Context, questionID uuid.UUID) (*domain.Answer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var latest *domain.Answer
	for _, a := range r.answers {
		if a.QuestionID == questionID {
			if latest == nil || a.Version > latest.Version {
				latest = a
			}
		}
	}
	if latest == nil {
		return nil, domain.ErrNotFound
	}
	return latest, nil
}

func (r *Repository) GetAnswerByVersion(ctx context.Context, questionID uuid.UUID, version int) (*domain.Answer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, a := range r.answers {
		if a.QuestionID == questionID && a.Version == version {
			return a, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *Repository) ListAnswers(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Answer
	for _, a := range r.answers {
		if a.ProjectID == projectID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (r *Repository) GetLatestAnswersForProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Group answers by question
	byQuestion := make(map[uuid.UUID]*domain.Answer)
	for _, a := range r.answers {
		if a.ProjectID != projectID {
			continue
		}
		existing, ok := byQuestion[a.QuestionID]
		if !ok || a.Version > existing.Version {
			byQuestion[a.QuestionID] = a
		}
	}

	var result []*domain.Answer
	for _, a := range byQuestion {
		result = append(result, a)
	}
	return result, nil
}

// Snapshots

func (r *Repository) CreateSnapshot(ctx context.Context, snapshot *domain.SpecSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots[snapshot.ID] = snapshot
	return nil
}

func (r *Repository) GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.SpecSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.snapshots[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (r *Repository) ListSnapshots(ctx context.Context, projectID uuid.UUID, limit int) ([]*domain.SpecSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.SpecSnapshot
	for _, s := range r.snapshots {
		if s.ProjectID == projectID {
			result = append(result, s)
		}
	}
	// Sort by created_at desc would be done here in production
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// Issues

func (r *Repository) CreateIssue(ctx context.Context, issue *domain.Issue) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.issues[issue.ID] = issue
	return nil
}

func (r *Repository) ListIssuesForSnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*domain.Issue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*domain.Issue
	for _, i := range r.issues {
		if i.SnapshotID == snapshotID {
			result = append(result, i)
		}
	}
	return result, nil
}

// Transaction support (simplified for testing)

func (r *Repository) WithTx(ctx context.Context, fn func(repository.Repository) error) error {
	return fn(r)
}

// Lifecycle

func (r *Repository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

// Ensure Repository implements repository.Repository
var _ repository.Repository = (*Repository)(nil)
