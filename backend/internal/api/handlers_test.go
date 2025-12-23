package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/repository/mock"
	"github.com/google/uuid"
)

func setupHandler() (*Handler, *mock.Repository) {
	repo := mock.New()
	handler := NewHandler(repo, nil) // No compiler for basic tests
	return handler, repo
}

func TestCreateProject(t *testing.T) {
	handler, _ := setupHandler()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid project",
			body:       `{"name": "Test Project"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "empty name",
			body:       `{"name": ""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       `{invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing name",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/projects", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateProject(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("CreateProject() status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusCreated {
				var resp createProjectResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.ProjectID == uuid.Nil {
					t.Error("Expected non-nil project ID")
				}
			}
		})
	}
}

func TestGetProject(t *testing.T) {
	handler, repo := setupHandler()

	// Create a project
	projectID := uuid.New()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	tests := []struct {
		name       string
		projectID  string
		wantStatus int
	}{
		{
			name:       "existing project",
			projectID:  projectID.String(),
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent project",
			projectID:  uuid.New().String(),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid uuid",
			projectID:  "invalid-uuid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/projects/"+tt.projectID, nil)
			req.SetPathValue("projectId", tt.projectID)
			w := httptest.NewRecorder()

			handler.GetProject(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GetProject() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp getProjectResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.Project.ID != projectID {
					t.Errorf("GetProject() ID = %v, want %v", resp.Project.ID, projectID)
				}
			}
		})
	}
}

func TestListQuestions(t *testing.T) {
	handler, repo := setupHandler()

	// Create a project with questions
	projectID := uuid.New()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	q1 := &domain.Question{
		ID:        uuid.New(),
		ProjectID: projectID,
		Text:      "Question 1",
		Type:      domain.QuestionTypeFreeform,
		Status:    domain.QuestionStatusUnanswered,
		Tags:      []string{"seed"},
		CreatedAt: time.Now().UTC(),
	}
	q2 := &domain.Question{
		ID:        uuid.New(),
		ProjectID: projectID,
		Text:      "Question 2",
		Type:      domain.QuestionTypeSingle,
		Status:    domain.QuestionStatusAnswered,
		Tags:      []string{"generated"},
		CreatedAt: time.Now().UTC(),
	}
	repo.CreateQuestion(nil, q1)
	repo.CreateQuestion(nil, q2)

	tests := []struct {
		name       string
		projectID  string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "all questions",
			projectID:  projectID.String(),
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "filter by status",
			projectID:  projectID.String(),
			query:      "?status=unanswered",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "filter by tag",
			projectID:  projectID.String(),
			query:      "?tag=seed",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "non-existent project",
			projectID:  uuid.New().String(),
			query:      "",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/projects/"+tt.projectID+"/questions"+tt.query, nil)
			req.SetPathValue("projectId", tt.projectID)
			w := httptest.NewRecorder()

			handler.ListQuestions(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ListQuestions() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp listQuestionsResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if len(resp.Questions) != tt.wantCount {
					t.Errorf("ListQuestions() count = %d, want %d", len(resp.Questions), tt.wantCount)
				}
			}
		})
	}
}

func TestSubmitAnswer(t *testing.T) {
	handler, repo := setupHandler()

	// Setup project and question
	projectID := uuid.New()
	questionID := uuid.New()

	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	question := &domain.Question{
		ID:        questionID,
		ProjectID: projectID,
		Text:      "Test Question",
		Type:      domain.QuestionTypeFreeform,
		Status:    domain.QuestionStatusUnanswered,
		CreatedAt: time.Now().UTC(),
	}
	repo.CreateQuestion(nil, question)

	tests := []struct {
		name       string
		projectID  string
		body       string
		wantStatus int
	}{
		{
			name:      "valid answer",
			projectID: projectID.String(),
			body: `{
				"question_id": "` + questionID.String() + `",
				"value": "My answer"
			}`,
			wantStatus: http.StatusOK,
		},
		{
			name:      "missing value",
			projectID: projectID.String(),
			body: `{
				"question_id": "` + questionID.String() + `"
			}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:      "non-existent question",
			projectID: projectID.String(),
			body: `{
				"question_id": "` + uuid.New().String() + `",
				"value": "My answer"
			}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:      "non-existent project",
			projectID: uuid.New().String(),
			body: `{
				"question_id": "` + questionID.String() + `",
				"value": "My answer"
			}`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/projects/"+tt.projectID+"/answers", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("projectId", tt.projectID)
			w := httptest.NewRecorder()

			handler.SubmitAnswer(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("SubmitAnswer() status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp submitAnswerResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.AnswerID == uuid.Nil {
					t.Error("Expected non-nil answer ID")
				}
			}
		})
	}
}

func TestAnswerVersioning(t *testing.T) {
	handler, repo := setupHandler()

	// Setup
	projectID := uuid.New()
	questionID := uuid.New()

	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	question := &domain.Question{
		ID:        questionID,
		ProjectID: projectID,
		Text:      "Test Question",
		Type:      domain.QuestionTypeFreeform,
		Status:    domain.QuestionStatusUnanswered,
		CreatedAt: time.Now().UTC(),
	}
	repo.CreateQuestion(nil, question)

	// Submit first answer
	body1 := `{"question_id": "` + questionID.String() + `", "value": "First answer"}`
	req1 := httptest.NewRequest("POST", "/projects/"+projectID.String()+"/answers", bytes.NewBufferString(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.SetPathValue("projectId", projectID.String())
	w1 := httptest.NewRecorder()
	handler.SubmitAnswer(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("First answer failed: %s", w1.Body.String())
	}

	// Submit second answer (should create version 2)
	body2 := `{"question_id": "` + questionID.String() + `", "value": "Second answer"}`
	req2 := httptest.NewRequest("POST", "/projects/"+projectID.String()+"/answers", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.SetPathValue("projectId", projectID.String())
	w2 := httptest.NewRecorder()
	handler.SubmitAnswer(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Second answer failed: %s", w2.Body.String())
	}

	// Verify version
	latest, err := repo.GetLatestAnswer(nil, questionID)
	if err != nil {
		t.Fatalf("GetLatestAnswer failed: %v", err)
	}
	if latest.Version != 2 {
		t.Errorf("Expected version 2, got %d", latest.Version)
	}
	if latest.Supersedes == nil {
		t.Error("Expected supersedes to be set")
	}
}

func TestListSnapshots(t *testing.T) {
	handler, repo := setupHandler()

	projectID := uuid.New()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	// Create snapshots
	for i := 0; i < 3; i++ {
		snapshot := &domain.SpecSnapshot{
			ID:        uuid.New(),
			ProjectID: projectID,
			Spec:      json.RawMessage(`{"version": ` + string(rune('0'+i)) + `}`),
			CreatedAt: time.Now().UTC(),
			Compiler: domain.CompilerConfig{
				Model:         "gpt-4o",
				PromptVersion: "v1",
			},
		}
		repo.CreateSnapshot(nil, snapshot)
	}

	req := httptest.NewRequest("GET", "/projects/"+projectID.String()+"/snapshots", nil)
	req.SetPathValue("projectId", projectID.String())
	w := httptest.NewRecorder()

	handler.ListSnapshots(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListSnapshots() status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp listSnapshotsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(resp.Snapshots) != 3 {
		t.Errorf("ListSnapshots() count = %d, want 3", len(resp.Snapshots))
	}
}

func TestGetSnapshot(t *testing.T) {
	handler, repo := setupHandler()

	projectID := uuid.New()
	snapshotID := uuid.New()

	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	snapshot := &domain.SpecSnapshot{
		ID:        snapshotID,
		ProjectID: projectID,
		Spec:      json.RawMessage(`{"test": true}`),
		CreatedAt: time.Now().UTC(),
		Compiler: domain.CompilerConfig{
			Model:         "gpt-4o",
			PromptVersion: "v1",
		},
	}
	repo.CreateSnapshot(nil, snapshot)

	// Create an issue for the snapshot
	issue := &domain.Issue{
		ID:         uuid.New(),
		ProjectID:  projectID,
		SnapshotID: snapshotID,
		Type:       domain.IssueTypeMissing,
		Severity:   domain.IssueSeverityWarn,
		Message:    "Test issue",
		CreatedAt:  time.Now().UTC(),
	}
	repo.CreateIssue(nil, issue)

	tests := []struct {
		name       string
		snapshotID string
		wantStatus int
	}{
		{
			name:       "existing snapshot",
			snapshotID: snapshotID.String(),
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent snapshot",
			snapshotID: uuid.New().String(),
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/projects/"+projectID.String()+"/snapshots/"+tt.snapshotID, nil)
			req.SetPathValue("projectId", projectID.String())
			req.SetPathValue("snapshotId", tt.snapshotID)
			w := httptest.NewRecorder()

			handler.GetSnapshot(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GetSnapshot() status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp getSnapshotResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.Snapshot.ID != snapshotID {
					t.Errorf("GetSnapshot() ID = %v, want %v", resp.Snapshot.ID, snapshotID)
				}
				if len(resp.Issues) != 1 {
					t.Errorf("GetSnapshot() issues count = %d, want 1", len(resp.Issues))
				}
			}
		})
	}
}

func TestDiffSnapshots(t *testing.T) {
	handler, repo := setupHandler()

	projectID := uuid.New()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	baseID := uuid.New()
	targetID := uuid.New()

	baseSnapshot := &domain.SpecSnapshot{
		ID:        baseID,
		ProjectID: projectID,
		Spec:      json.RawMessage(`{"product": {"name": "Test"}}`),
		CreatedAt: time.Now().UTC(),
		Compiler:  domain.CompilerConfig{Model: "gpt-4o", PromptVersion: "v1"},
	}
	targetSnapshot := &domain.SpecSnapshot{
		ID:        targetID,
		ProjectID: projectID,
		Spec:      json.RawMessage(`{"product": {"name": "Updated Test"}}`),
		CreatedAt: time.Now().UTC(),
		Compiler:  domain.CompilerConfig{Model: "gpt-4o", PromptVersion: "v1"},
	}
	repo.CreateSnapshot(nil, baseSnapshot)
	repo.CreateSnapshot(nil, targetSnapshot)

	tests := []struct {
		name       string
		targetID   string
		baseID     string
		wantStatus int
	}{
		{
			name:       "valid diff",
			targetID:   targetID.String(),
			baseID:     baseID.String(),
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing base",
			targetID:   targetID.String(),
			baseID:     "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-existent target",
			targetID:   uuid.New().String(),
			baseID:     baseID.String(),
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/projects/" + projectID.String() + "/snapshots/" + tt.targetID + "/diff"
			if tt.baseID != "" {
				url += "?base=" + tt.baseID
			}
			req := httptest.NewRequest("GET", url, nil)
			req.SetPathValue("projectId", projectID.String())
			req.SetPathValue("snapshotId", tt.targetID)
			w := httptest.NewRecorder()

			handler.DiffSnapshots(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("DiffSnapshots() status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp diffResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.Diff == nil {
					t.Error("Expected non-nil diff")
				}
				if resp.Impact == nil {
					t.Error("Expected non-nil impact")
				}
				// Should have 1 change (product.name modified)
				if resp.Diff.Summary.Modified != 1 {
					t.Errorf("Expected 1 modification, got %d", resp.Diff.Summary.Modified)
				}
			}
		})
	}
}

func TestCompileWithoutCompiler(t *testing.T) {
	handler, repo := setupHandler()

	projectID := uuid.New()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	req := httptest.NewRequest("POST", "/projects/"+projectID.String()+"/compile", bytes.NewBufferString(`{"mode": "latest_answers"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("projectId", projectID.String())
	w := httptest.NewRecorder()

	handler.Compile(w, req)

	// Should return service unavailable since compiler is nil
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Compile() without compiler status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestExportWithoutSnapshot(t *testing.T) {
	handler, repo := setupHandler()

	projectID := uuid.New()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	req := httptest.NewRequest("GET", "/projects/"+projectID.String()+"/export", nil)
	req.SetPathValue("projectId", projectID.String())
	w := httptest.NewRecorder()

	handler.ExportPack(w, req)

	// Should return error since no snapshot exists
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("ExportPack() without snapshot status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
}

func TestExportWithSnapshot(t *testing.T) {
	handler, repo := setupHandler()

	projectID := uuid.New()
	snapshotID := uuid.New()

	project := &domain.Project{
		ID:        projectID,
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	repo.CreateProject(nil, project)

	snapshot := &domain.SpecSnapshot{
		ID:          snapshotID,
		ProjectID:   projectID,
		Spec:        json.RawMessage(`{"product": {"name": "Test"}}`),
		CreatedAt:   time.Now().UTC(),
		DerivedFrom: map[uuid.UUID]int{},
		Compiler:    domain.CompilerConfig{Model: "gpt-4o", PromptVersion: "v1"},
	}
	repo.CreateSnapshot(nil, snapshot)

	req := httptest.NewRequest("GET", "/projects/"+projectID.String()+"/export", nil)
	req.SetPathValue("projectId", projectID.String())
	w := httptest.NewRecorder()

	handler.ExportPack(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ExportPack() status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify content type is zip
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("ExportPack() Content-Type = %s, want application/zip", contentType)
	}

	// Verify content disposition
	contentDisp := w.Header().Get("Content-Disposition")
	if contentDisp == "" {
		t.Error("Expected Content-Disposition header")
	}
}
