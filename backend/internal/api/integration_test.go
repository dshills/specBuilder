package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dshills/specbuilder/backend/internal/compiler"
	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/llm"
	"github.com/dshills/specbuilder/backend/internal/repository/mock"
	"github.com/dshills/specbuilder/backend/internal/validator"
	"github.com/google/uuid"
)

// setupIntegrationTest creates a fully wired handler with mock dependencies.
func setupIntegrationTest(t *testing.T, llmResponse string) (*Handler, *mock.Repository, *llm.MockClient) {
	t.Helper()
	repo := mock.New()
	mockClient := llm.NewMockClient(llmResponse)
	val, err := validator.New()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	compilerSvc := compiler.NewService(mockClient, val, `{"type": "object"}`)
	handler := NewHandler(repo, compilerSvc)
	return handler, repo, mockClient
}

// TestIntegration_FullSpecBuildingFlow tests the complete flow of building a spec.
func TestIntegration_FullSpecBuildingFlow(t *testing.T) {
	// Mock LLM responses for the compile step
	compileResponse := `{
		"spec": {
			"product": {"name": "Test App", "purpose": "Testing"},
			"scope": {"in_scope": ["User authentication"], "out_of_scope": []}
		},
		"trace": {"product.name": {"question_id": "q1", "answer_id": "a1"}}
	}`

	handler, _, _ := setupIntegrationTest(t, compileResponse)

	// Step 1: Create a project
	createBody := []byte(`{"name": "Integration Test Project"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateProject status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var projectResp struct {
		ProjectID uuid.UUID `json:"project_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&projectResp); err != nil {
		t.Fatalf("Failed to decode project response: %v", err)
	}

	projectID := projectResp.ProjectID
	if projectID == uuid.Nil {
		t.Fatal("CreateProject returned nil project ID")
	}

	// Step 2: List questions (seeded by CreateProject)
	req = httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/questions", nil)
	req.SetPathValue("projectId", projectID.String())
	rec = httptest.NewRecorder()

	handler.ListQuestions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListQuestions status = %d, want %d", rec.Code, http.StatusOK)
	}

	var questionsResp struct {
		Questions []*domain.Question `json:"questions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&questionsResp); err != nil {
		t.Fatalf("Failed to decode questions: %v", err)
	}

	// CreateProject seeds some questions
	if len(questionsResp.Questions) == 0 {
		t.Error("Expected seeded questions from CreateProject")
	}

	// Step 3: Submit answers to the first 2 seeded questions
	questions := questionsResp.Questions
	answerCount := 2
	if len(questions) < answerCount {
		answerCount = len(questions)
	}
	for i := 0; i < answerCount; i++ {
		q := questions[i]
		answerValue := "Test answer for question " + q.Text
		answerBody := []byte(`{"question_id": "` + q.ID.String() + `", "value": "` + answerValue + `"}`)

		req = httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/answers", bytes.NewReader(answerBody))
		req.SetPathValue("projectId", projectID.String())
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()

		handler.SubmitAnswer(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("SubmitAnswer status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	// Step 4: Verify questions are now answered
	req = httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/questions?status=answered", nil)
	req.SetPathValue("projectId", projectID.String())
	rec = httptest.NewRecorder()

	handler.ListQuestions(rec, req)

	var answeredQuestionsResp struct {
		Questions []*domain.Question `json:"questions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&answeredQuestionsResp); err != nil {
		t.Fatalf("Failed to decode answered questions: %v", err)
	}

	if len(answeredQuestionsResp.Questions) != answerCount {
		t.Errorf("Answered questions count = %d, want %d", len(answeredQuestionsResp.Questions), answerCount)
	}

	// Step 5: Compile the spec
	compileBody := []byte(`{"mode": "latest_answers"}`)
	req = httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/compile", bytes.NewReader(compileBody))
	req.SetPathValue("projectId", projectID.String())
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	handler.Compile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Compile status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var compileResp struct {
		SnapshotID uuid.UUID       `json:"snapshot_id"`
		Issues     []*domain.Issue `json:"issues"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&compileResp); err != nil {
		t.Fatalf("Failed to decode compile response: %v", err)
	}

	if compileResp.SnapshotID == uuid.Nil {
		t.Error("Compile returned empty snapshot ID")
	}

	// Step 6: List snapshots
	req = httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/snapshots", nil)
	req.SetPathValue("projectId", projectID.String())
	rec = httptest.NewRecorder()

	handler.ListSnapshots(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ListSnapshots status = %d, want %d", rec.Code, http.StatusOK)
	}

	var snapshotsResp struct {
		Snapshots []*domain.SpecSnapshot `json:"snapshots"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&snapshotsResp); err != nil {
		t.Fatalf("Failed to decode snapshots: %v", err)
	}

	if len(snapshotsResp.Snapshots) != 1 {
		t.Errorf("Snapshots count = %d, want 1", len(snapshotsResp.Snapshots))
	}
}

// TestIntegration_AnswerVersioning tests that answer versioning works correctly.
func TestIntegration_AnswerVersioning(t *testing.T) {
	handler, repo, _ := setupIntegrationTest(t, `{"spec": {}, "trace": {}}`)

	// Create project
	projectID := uuid.New()
	now := time.Now().UTC()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Versioning Test",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateProject(context.Background(), project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create question
	questionID := uuid.New()
	question := &domain.Question{
		ID:        questionID,
		ProjectID: projectID,
		Text:      "What database should we use?",
		Type:      domain.QuestionTypeSingle,
		Options:   []string{"PostgreSQL", "MySQL", "SQLite"},
		Status:    domain.QuestionStatusUnanswered,
		Priority:  100,
		CreatedAt: now,
	}
	if err := repo.CreateQuestion(context.Background(), question); err != nil {
		t.Fatalf("Failed to create question: %v", err)
	}

	// Submit first answer
	answerBody := []byte(`{"question_id": "` + questionID.String() + `", "value": "PostgreSQL"}`)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/answers", bytes.NewReader(answerBody))
	req.SetPathValue("projectId", projectID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.SubmitAnswer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("First answer status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var answer1 struct {
		AnswerID uuid.UUID `json:"answer_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&answer1); err != nil {
		t.Fatalf("Failed to decode first answer response: %v", err)
	}

	if answer1.AnswerID == uuid.Nil {
		t.Fatal("First answer ID should not be nil")
	}

	// Get the answer to verify version
	firstAnswer, err := repo.GetLatestAnswer(context.Background(), questionID)
	if err != nil {
		t.Fatalf("Failed to get latest answer: %v", err)
	}
	if firstAnswer == nil {
		t.Fatal("Expected an answer but got nil")
	}
	if firstAnswer.Version != 1 {
		t.Errorf("First answer version = %d, want 1", firstAnswer.Version)
	}

	// Submit second answer (revised)
	answerBody = []byte(`{"question_id": "` + questionID.String() + `", "value": "MySQL"}`)
	req = httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/answers", bytes.NewReader(answerBody))
	req.SetPathValue("projectId", projectID.String())
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	handler.SubmitAnswer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Second answer status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Get the answer to verify version
	secondAnswer, err := repo.GetLatestAnswer(context.Background(), questionID)
	if err != nil {
		t.Fatalf("Failed to get second answer: %v", err)
	}
	if secondAnswer == nil {
		t.Fatal("Expected second answer but got nil")
	}
	if secondAnswer.Version != 2 {
		t.Errorf("Second answer version = %d, want 2", secondAnswer.Version)
	}

	// Submit third answer (another revision)
	answerBody = []byte(`{"question_id": "` + questionID.String() + `", "value": "SQLite"}`)
	req = httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/answers", bytes.NewReader(answerBody))
	req.SetPathValue("projectId", projectID.String())
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	handler.SubmitAnswer(rec, req)

	// Get the answer to verify version
	thirdAnswer, err := repo.GetLatestAnswer(context.Background(), questionID)
	if err != nil {
		t.Fatalf("Failed to get third answer: %v", err)
	}
	if thirdAnswer == nil {
		t.Fatal("Expected third answer but got nil")
	}
	if thirdAnswer.Version != 3 {
		t.Errorf("Third answer version = %d, want 3", thirdAnswer.Version)
	}
}

// TestIntegration_SnapshotDiffing tests snapshot comparison.
func TestIntegration_SnapshotDiffing(t *testing.T) {
	handler, repo, _ := setupIntegrationTest(t, `{"spec": {}, "trace": {}}`)

	projectID := uuid.New()
	now := time.Now().UTC()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Diff Test",
		CreatedAt: now,
	}
	if err := repo.CreateProject(context.Background(), project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create two snapshots with different specs
	snapshot1 := &domain.SpecSnapshot{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Spec:        json.RawMessage(`{"product": {"name": "App v1"}}`),
		CreatedAt:   now,
		DerivedFrom: map[uuid.UUID]int{},
		Compiler:    domain.CompilerConfig{Model: "mock-model"},
	}
	if err := repo.CreateSnapshot(context.Background(), snapshot1); err != nil {
		t.Fatalf("Failed to create snapshot1: %v", err)
	}

	snapshot2 := &domain.SpecSnapshot{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Spec:        json.RawMessage(`{"product": {"name": "App v2", "version": "2.0"}}`),
		CreatedAt:   now.Add(time.Hour),
		DerivedFrom: map[uuid.UUID]int{},
		Compiler:    domain.CompilerConfig{Model: "mock-model"},
	}
	if err := repo.CreateSnapshot(context.Background(), snapshot2); err != nil {
		t.Fatalf("Failed to create snapshot2: %v", err)
	}

	// Get diff between snapshots - route is /projects/{projectId}/snapshots/{snapshotId}/diff?base={baseId}
	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/snapshots/"+snapshot2.ID.String()+"/diff?base="+snapshot1.ID.String(), nil)
	req.SetPathValue("projectId", projectID.String())
	req.SetPathValue("snapshotId", snapshot2.ID.String())
	rec := httptest.NewRecorder()

	handler.DiffSnapshots(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("DiffSnapshots status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var diffResp struct {
		Diff struct {
			Changes []map[string]interface{} `json:"changes"`
			Summary struct {
				Added    int `json:"added"`
				Removed  int `json:"removed"`
				Modified int `json:"modified"`
			} `json:"summary"`
		} `json:"diff"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&diffResp); err != nil {
		t.Fatalf("Failed to decode diff response: %v", err)
	}

	// Should have changes (name changed, version added)
	if len(diffResp.Diff.Changes) == 0 {
		t.Error("Expected changes between snapshots")
	}
}

// TestIntegration_ExportPack tests spec export functionality.
func TestIntegration_ExportPack(t *testing.T) {
	handler, repo, _ := setupIntegrationTest(t, `{}`)

	projectID := uuid.New()
	now := time.Now().UTC()
	project := &domain.Project{
		ID:        projectID,
		Name:      "Export Test",
		CreatedAt: now,
	}
	if err := repo.CreateProject(context.Background(), project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	snapshot := &domain.SpecSnapshot{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Spec:        json.RawMessage(`{"product": {"name": "Export App", "purpose": "Testing exports"}}`),
		CreatedAt:   now,
		DerivedFrom: map[uuid.UUID]int{},
		Compiler:    domain.CompilerConfig{Model: "mock-model"},
	}
	if err := repo.CreateSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Test export - returns a zip file
	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/export", nil)
	req.SetPathValue("projectId", projectID.String())
	rec := httptest.NewRecorder()

	handler.ExportPack(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ExportPack status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("Export content type = %s, want application/zip", contentType)
	}

	contentDisposition := rec.Header().Get("Content-Disposition")
	if contentDisposition == "" {
		t.Error("Export should have Content-Disposition header")
	}

	// Verify response body has content (zip file)
	if rec.Body.Len() == 0 {
		t.Error("Export response body should not be empty")
	}
}

// TestIntegration_ProjectNotFound tests error handling for missing projects.
func TestIntegration_ProjectNotFound(t *testing.T) {
	handler, _, _ := setupIntegrationTest(t, `{}`)

	nonExistentID := uuid.New()

	// GetProject
	req := httptest.NewRequest(http.MethodGet, "/projects/"+nonExistentID.String(), nil)
	req.SetPathValue("projectId", nonExistentID.String())
	rec := httptest.NewRecorder()

	handler.GetProject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GetProject for non-existent project = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// ListQuestions
	req = httptest.NewRequest(http.MethodGet, "/projects/"+nonExistentID.String()+"/questions", nil)
	req.SetPathValue("projectId", nonExistentID.String())
	rec = httptest.NewRecorder()

	handler.ListQuestions(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ListQuestions for non-existent project = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
