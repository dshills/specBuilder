package sqlite

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/google/uuid"
)

func TestSQLiteRepository(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "specbuilder-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	repo, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Test Project CRUD
	t.Run("Project", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		project := &domain.Project{
			ID:        uuid.New(),
			Name:      "Test Project",
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := repo.CreateProject(ctx, project); err != nil {
			t.Fatalf("CreateProject failed: %v", err)
		}

		got, err := repo.GetProject(ctx, project.ID)
		if err != nil {
			t.Fatalf("GetProject failed: %v", err)
		}

		if got.Name != project.Name {
			t.Errorf("Name mismatch: got %q, want %q", got.Name, project.Name)
		}

		// Test not found
		_, err = repo.GetProject(ctx, uuid.New())
		if err != domain.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	// Test Question CRUD
	t.Run("Question", func(t *testing.T) {
		projectID := uuid.New()
		now := time.Now().UTC().Truncate(time.Second)

		// Create project first
		project := &domain.Project{
			ID: projectID, Name: "Q Test", CreatedAt: now, UpdatedAt: now,
		}
		if err := repo.CreateProject(ctx, project); err != nil {
			t.Fatalf("CreateProject failed: %v", err)
		}

		question := &domain.Question{
			ID:        uuid.New(),
			ProjectID: projectID,
			Text:      "What is the purpose?",
			Type:      domain.QuestionTypeFreeform,
			Options:   nil,
			Tags:      []string{"seed", "product"},
			Priority:  100,
			SpecPaths: []string{"/product"},
			Status:    domain.QuestionStatusUnanswered,
			CreatedAt: now,
		}

		if err := repo.CreateQuestion(ctx, question); err != nil {
			t.Fatalf("CreateQuestion failed: %v", err)
		}

		got, err := repo.GetQuestion(ctx, question.ID)
		if err != nil {
			t.Fatalf("GetQuestion failed: %v", err)
		}

		if got.Text != question.Text {
			t.Errorf("Text mismatch: got %q, want %q", got.Text, question.Text)
		}
		if len(got.Tags) != 2 {
			t.Errorf("Tags count mismatch: got %d, want 2", len(got.Tags))
		}

		// Test list
		questions, err := repo.ListQuestions(ctx, projectID, nil, nil)
		if err != nil {
			t.Fatalf("ListQuestions failed: %v", err)
		}
		if len(questions) != 1 {
			t.Errorf("Expected 1 question, got %d", len(questions))
		}

		// Test status update
		if err := repo.UpdateQuestionStatus(ctx, question.ID, domain.QuestionStatusAnswered); err != nil {
			t.Fatalf("UpdateQuestionStatus failed: %v", err)
		}

		got, _ = repo.GetQuestion(ctx, question.ID)
		if got.Status != domain.QuestionStatusAnswered {
			t.Errorf("Status not updated: got %q, want %q", got.Status, domain.QuestionStatusAnswered)
		}
	})

	// Test Answer versioning
	t.Run("Answer versioning", func(t *testing.T) {
		projectID := uuid.New()
		questionID := uuid.New()
		now := time.Now().UTC().Truncate(time.Second)

		// Setup
		project := &domain.Project{ID: projectID, Name: "A Test", CreatedAt: now, UpdatedAt: now}
		repo.CreateProject(ctx, project)

		question := &domain.Question{
			ID: questionID, ProjectID: projectID, Text: "Test?", Type: domain.QuestionTypeFreeform,
			Tags: []string{}, SpecPaths: []string{}, Status: domain.QuestionStatusUnanswered, CreatedAt: now,
		}
		repo.CreateQuestion(ctx, question)

		// Create first answer
		answer1 := &domain.Answer{
			ID: uuid.New(), ProjectID: projectID, QuestionID: questionID,
			Value: json.RawMessage(`"first answer"`), Version: 1, CreatedAt: now,
		}
		if err := repo.CreateAnswer(ctx, answer1); err != nil {
			t.Fatalf("CreateAnswer v1 failed: %v", err)
		}

		// Create second answer (supersedes first)
		answer2 := &domain.Answer{
			ID: uuid.New(), ProjectID: projectID, QuestionID: questionID,
			Value: json.RawMessage(`"second answer"`), Version: 2, Supersedes: &answer1.ID, CreatedAt: now,
		}
		if err := repo.CreateAnswer(ctx, answer2); err != nil {
			t.Fatalf("CreateAnswer v2 failed: %v", err)
		}

		// Test GetLatestAnswer
		latest, err := repo.GetLatestAnswer(ctx, questionID)
		if err != nil {
			t.Fatalf("GetLatestAnswer failed: %v", err)
		}
		if latest.Version != 2 {
			t.Errorf("Latest version mismatch: got %d, want 2", latest.Version)
		}

		// Test GetAnswerByVersion
		v1, err := repo.GetAnswerByVersion(ctx, questionID, 1)
		if err != nil {
			t.Fatalf("GetAnswerByVersion failed: %v", err)
		}
		if string(v1.Value) != `"first answer"` {
			t.Errorf("V1 value mismatch: got %s", v1.Value)
		}

		// Test GetLatestAnswersForProject
		latestAll, err := repo.GetLatestAnswersForProject(ctx, projectID)
		if err != nil {
			t.Fatalf("GetLatestAnswersForProject failed: %v", err)
		}
		if len(latestAll) != 1 {
			t.Errorf("Expected 1 latest answer, got %d", len(latestAll))
		}
		if latestAll[0].Version != 2 {
			t.Errorf("Latest answer version mismatch: got %d, want 2", latestAll[0].Version)
		}
	})

	// Test Snapshot
	t.Run("Snapshot", func(t *testing.T) {
		projectID := uuid.New()
		now := time.Now().UTC().Truncate(time.Second)

		project := &domain.Project{ID: projectID, Name: "S Test", CreatedAt: now, UpdatedAt: now}
		repo.CreateProject(ctx, project)

		questionID := uuid.New()
		snapshot := &domain.SpecSnapshot{
			ID:        uuid.New(),
			ProjectID: projectID,
			Spec:      json.RawMessage(`{"product":{"name":"Test"}}`),
			CreatedAt: now,
			DerivedFrom: map[uuid.UUID]int{
				questionID: 1,
			},
			Compiler: domain.CompilerConfig{
				Model:         "gpt-4",
				PromptVersion: "v1",
				Temperature:   0,
			},
		}

		if err := repo.CreateSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("CreateSnapshot failed: %v", err)
		}

		got, err := repo.GetSnapshot(ctx, snapshot.ID)
		if err != nil {
			t.Fatalf("GetSnapshot failed: %v", err)
		}

		if got.Compiler.Model != "gpt-4" {
			t.Errorf("Compiler model mismatch: got %q", got.Compiler.Model)
		}
		if got.DerivedFrom[questionID] != 1 {
			t.Errorf("DerivedFrom mismatch")
		}

		// Test GetLatestSnapshotID
		latestID, err := repo.GetLatestSnapshotID(ctx, projectID)
		if err != nil {
			t.Fatalf("GetLatestSnapshotID failed: %v", err)
		}
		if *latestID != snapshot.ID {
			t.Errorf("Latest snapshot ID mismatch")
		}

		// Test ListSnapshots
		snapshots, err := repo.ListSnapshots(ctx, projectID, 10)
		if err != nil {
			t.Fatalf("ListSnapshots failed: %v", err)
		}
		if len(snapshots) != 1 {
			t.Errorf("Expected 1 snapshot, got %d", len(snapshots))
		}
	})

	// Test Issue
	t.Run("Issue", func(t *testing.T) {
		projectID := uuid.New()
		snapshotID := uuid.New()
		now := time.Now().UTC().Truncate(time.Second)

		project := &domain.Project{ID: projectID, Name: "I Test", CreatedAt: now, UpdatedAt: now}
		repo.CreateProject(ctx, project)

		snapshot := &domain.SpecSnapshot{
			ID: snapshotID, ProjectID: projectID, Spec: json.RawMessage(`{}`),
			CreatedAt: now, DerivedFrom: map[uuid.UUID]int{},
			Compiler: domain.CompilerConfig{Model: "test", PromptVersion: "v1", Temperature: 0},
		}
		repo.CreateSnapshot(ctx, snapshot)

		questionID := uuid.New()
		issue := &domain.Issue{
			ID:                 uuid.New(),
			ProjectID:          projectID,
			SnapshotID:         snapshotID,
			Type:               domain.IssueTypeMissing,
			Severity:           domain.IssueSeverityWarn,
			Message:            "Missing trace for /product",
			RelatedSpecPaths:   []string{"/product"},
			RelatedQuestionIDs: []uuid.UUID{questionID},
			CreatedAt:          now,
		}

		if err := repo.CreateIssue(ctx, issue); err != nil {
			t.Fatalf("CreateIssue failed: %v", err)
		}

		issues, err := repo.ListIssuesForSnapshot(ctx, snapshotID)
		if err != nil {
			t.Fatalf("ListIssuesForSnapshot failed: %v", err)
		}
		if len(issues) != 1 {
			t.Errorf("Expected 1 issue, got %d", len(issues))
		}
		if issues[0].Type != domain.IssueTypeMissing {
			t.Errorf("Issue type mismatch: got %q", issues[0].Type)
		}
	})
}
