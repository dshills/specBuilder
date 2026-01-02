package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/dshills/specbuilder/backend/internal/compiler"
	"github.com/dshills/specbuilder/backend/internal/diff"
	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/export"
	"github.com/dshills/specbuilder/backend/internal/llm"
	"github.com/dshills/specbuilder/backend/internal/repository"
	"github.com/google/uuid"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	repo     repository.Repository
	compiler *compiler.Service
}

// NewHandler creates a new Handler.
func NewHandler(repo repository.Repository, comp *compiler.Service) *Handler {
	return &Handler{repo: repo, compiler: comp}
}

// RegisterRoutes registers all API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Models
	mux.HandleFunc("GET /models", h.ListModels)

	// Projects
	mux.HandleFunc("GET /projects", h.ListProjects)
	mux.HandleFunc("POST /projects", h.CreateProject)
	mux.HandleFunc("GET /projects/{projectId}", h.GetProject)
	mux.HandleFunc("DELETE /projects/{projectId}", h.DeleteProject)

	// Questions
	mux.HandleFunc("GET /projects/{projectId}/questions", h.ListQuestions)
	mux.HandleFunc("POST /projects/{projectId}/next-questions", h.GenerateNextQuestions)

	// Suggestions
	mux.HandleFunc("POST /projects/{projectId}/suggestions", h.GenerateSuggestions)

	// Answers
	mux.HandleFunc("POST /projects/{projectId}/answers", h.SubmitAnswer)

	// Compilation
	mux.HandleFunc("POST /projects/{projectId}/compile", h.Compile)

	// Snapshots
	mux.HandleFunc("GET /projects/{projectId}/snapshots", h.ListSnapshots)
	mux.HandleFunc("GET /projects/{projectId}/snapshots/{snapshotId}", h.GetSnapshot)
	mux.HandleFunc("GET /projects/{projectId}/snapshots/{snapshotId}/diff", h.DiffSnapshots)

	// Export
	mux.HandleFunc("GET /projects/{projectId}/export", h.ExportPack)
}

// Error response helpers

type errorResponse struct {
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err, message string) {
	writeJSON(w, status, errorResponse{Error: err, Message: message})
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// Models

type listModelsResponse struct {
	Providers       []llm.ProviderInfo `json:"providers"`
	DefaultProvider llm.Provider       `json:"default_provider"`
	DefaultModel    string             `json:"default_model"`
}

func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	if h.compiler == nil {
		writeJSON(w, http.StatusOK, listModelsResponse{
			Providers: []llm.ProviderInfo{},
		})
		return
	}

	factory := h.compiler.Factory()
	writeJSON(w, http.StatusOK, listModelsResponse{
		Providers:       factory.ListProviders(),
		DefaultProvider: factory.DefaultProvider(),
		DefaultModel:    factory.DefaultModel(),
	})
}

// Projects

type createProjectRequest struct {
	Name string `json:"name"`
	Mode string `json:"mode"` // "basic" or "advanced" (default: advanced)
}

// ListProjects

type listProjectsResponse struct {
	Projects []*domain.Project `json:"projects"`
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.repo.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list projects")
		return
	}
	if projects == nil {
		projects = []*domain.Project{}
	}
	writeJSON(w, http.StatusOK, listProjectsResponse{Projects: projects})
}

// CreateProject

type createProjectResponse struct {
	ProjectID uuid.UUID `json:"project_id"`
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	// Default to advanced mode
	mode := domain.ProjectModeAdvanced
	if req.Mode == "basic" {
		mode = domain.ProjectModeBasic
	}

	now := time.Now().UTC()
	project := &domain.Project{
		ID:        uuid.New(),
		Name:      req.Name,
		Mode:      mode,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.repo.CreateProject(r.Context(), project); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create project")
		return
	}

	// Seed minimum questions for new project
	if err := h.seedQuestions(r.Context(), project.ID, mode); err != nil {
		// Log but don't fail - project was created
	}

	writeJSON(w, http.StatusCreated, createProjectResponse{ProjectID: project.ID})
}

func (h *Handler) seedQuestions(ctx context.Context, projectID uuid.UUID, mode domain.ProjectMode) error {
	var seeds []struct {
		text     string
		specPath string
	}

	if mode == domain.ProjectModeBasic {
		// Simple, non-technical questions for non-programmers
		seeds = []struct {
			text     string
			specPath string
		}{
			{"What do you want to call your product or app?", "/product"},
			{"In one sentence, what problem does it solve?", "/product"},
			{"Who will use this? Describe your typical user.", "/personas"},
			{"What's the main thing a user should be able to do?", "/workflows"},
			{"What are 2-3 other important features?", "/requirements"},
			{"Is there anything you definitely don't want to include?", "/scope/out_of_scope"},
			{"Do you have any examples of similar products you like?", "/product"},
		}
	} else {
		// Technical questions for developers (advanced mode)
		seeds = []struct {
			text     string
			specPath string
		}{
			{"What is the product name and one-sentence purpose?", "/product"},
			{"Who are the primary users/personas?", "/personas"},
			{"What is explicitly out of scope?", "/scope/out_of_scope"},
			{"Describe the primary workflow (happy path) in 5-10 steps.", "/workflows"},
			{"What data entities exist (roughly)?", "/data_model"},
			{"What interfaces are required (API/UI/integrations)?", "/api"},
			{"What non-functional constraints matter most (security, performance, cost)?", "/non_functionals"},
		}
	}

	now := time.Now().UTC()
	for i, seed := range seeds {
		q := &domain.Question{
			ID:        uuid.New(),
			ProjectID: projectID,
			Text:      seed.text,
			Type:      domain.QuestionTypeFreeform,
			Options:   nil,
			Tags:      []string{"seed"},
			Priority:  100 - i, // Higher priority for earlier questions
			SpecPaths: []string{seed.specPath},
			Status:    domain.QuestionStatusUnanswered,
			CreatedAt: now,
		}
		if err := h.repo.CreateQuestion(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

type getProjectResponse struct {
	Project          *domain.Project `json:"project"`
	LatestSnapshotID *uuid.UUID      `json:"latest_snapshot_id"`
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("projectId")
	id, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	project, err := h.repo.GetProject(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get project")
		return
	}

	latestID, err := h.repo.GetLatestSnapshotID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get latest snapshot")
		return
	}

	writeJSON(w, http.StatusOK, getProjectResponse{Project: project, LatestSnapshotID: latestID})
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("projectId")
	id, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	if err := h.repo.DeleteProject(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete project")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Questions

type listQuestionsResponse struct {
	Questions []*domain.Question `json:"questions"`
}

func (h *Handler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("projectId")
	projectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	// Check project exists
	if _, err := h.repo.GetProject(r.Context(), projectID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get project")
		return
	}

	// Parse query params
	var status *domain.QuestionStatus
	if s := r.URL.Query().Get("status"); s != "" {
		qs := domain.QuestionStatus(s)
		status = &qs
	}
	var tag *string
	if t := r.URL.Query().Get("tag"); t != "" {
		tag = &t
	}

	questions, err := h.repo.ListQuestions(r.Context(), projectID, status, tag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list questions")
		return
	}

	if questions == nil {
		questions = []*domain.Question{}
	}

	writeJSON(w, http.StatusOK, listQuestionsResponse{Questions: questions})
}

// Answers

type submitAnswerRequest struct {
	QuestionID uuid.UUID       `json:"question_id"`
	Value      json.RawMessage `json:"value"`
	Compile    *bool           `json:"compile,omitempty"`
}

type submitAnswerResponse struct {
	AnswerID   uuid.UUID       `json:"answer_id"`
	SnapshotID *uuid.UUID      `json:"snapshot_id"`
	Issues     []*domain.Issue `json:"issues"`
}

func (h *Handler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("projectId")
	projectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	var req submitAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	if req.Value == nil || len(req.Value) == 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "value is required")
		return
	}

	// Check project exists
	if _, err := h.repo.GetProject(r.Context(), projectID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get project")
		return
	}

	// Check question exists and belongs to project
	question, err := h.repo.GetQuestion(r.Context(), req.QuestionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Question not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get question")
		return
	}
	if question.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "not_found", "Question not found in this project")
		return
	}

	// Create new answer version
	now := time.Now().UTC()
	answer := &domain.Answer{
		ID:         uuid.New(),
		ProjectID:  projectID,
		QuestionID: req.QuestionID,
		Value:      req.Value,
		Version:    1,
		Supersedes: nil,
		CreatedAt:  now,
	}

	// Check for existing answer to supersede
	existing, err := h.repo.GetLatestAnswer(r.Context(), req.QuestionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check existing answer")
		return
	}
	if existing != nil {
		answer.Version = existing.Version + 1
		answer.Supersedes = &existing.ID
	}

	if err := h.repo.CreateAnswer(r.Context(), answer); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create answer")
		return
	}

	// Update question status
	if err := h.repo.UpdateQuestionStatus(r.Context(), req.QuestionID, domain.QuestionStatusAnswered); err != nil {
		// Log but don't fail
	}

	// Compilation is triggered separately via POST /projects/{id}/compile
	// or POST /projects/{id}/next-questions which includes compilation

	writeJSON(w, http.StatusOK, submitAnswerResponse{
		AnswerID:   answer.ID,
		SnapshotID: nil,
		Issues:     []*domain.Issue{},
	})
}

// Snapshots

type listSnapshotsResponse struct {
	Snapshots []*domain.SpecSnapshot `json:"snapshots"`
}

func (h *Handler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("projectId")
	projectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	// Check project exists
	if _, err := h.repo.GetProject(r.Context(), projectID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get project")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	snapshots, err := h.repo.ListSnapshots(r.Context(), projectID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list snapshots")
		return
	}

	if snapshots == nil {
		snapshots = []*domain.SpecSnapshot{}
	}

	writeJSON(w, http.StatusOK, listSnapshotsResponse{Snapshots: snapshots})
}

type getSnapshotResponse struct {
	Snapshot *domain.SpecSnapshot `json:"snapshot"`
	Issues   []*domain.Issue      `json:"issues"`
}

func (h *Handler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.PathValue("projectId")
	projectID, err := parseUUID(projectIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	snapshotIDStr := r.PathValue("snapshotId")
	snapshotID, err := parseUUID(snapshotIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid snapshot ID format")
		return
	}

	snapshot, err := h.repo.GetSnapshot(r.Context(), snapshotID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Snapshot not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get snapshot")
		return
	}

	if snapshot.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "not_found", "Snapshot not found in this project")
		return
	}

	issues, err := h.repo.ListIssuesForSnapshot(r.Context(), snapshotID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get issues")
		return
	}

	if issues == nil {
		issues = []*domain.Issue{}
	}

	writeJSON(w, http.StatusOK, getSnapshotResponse{Snapshot: snapshot, Issues: issues})
}

// Compilation

type compileRequest struct {
	Mode           string         `json:"mode"` // latest_answers or specific_answer_versions
	AnswerVersions map[string]int `json:"answer_versions,omitempty"`
	Provider       llm.Provider   `json:"provider,omitempty"` // Optional: override default provider
	Model          string         `json:"model,omitempty"`    // Optional: override default model
}

type compileResponse struct {
	SnapshotID uuid.UUID       `json:"snapshot_id"`
	Issues     []*domain.Issue `json:"issues"`
}

func (h *Handler) Compile(w http.ResponseWriter, r *http.Request) {
	log.Printf("Compile: starting request")
	if h.compiler == nil {
		log.Printf("Compile: compiler not configured")
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "Compilation service not configured")
		return
	}

	idStr := r.PathValue("projectId")
	projectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	var req compileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	// Get project
	project, err := h.repo.GetProject(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get project")
		return
	}

	// Get latest answers
	answers, err := h.repo.GetLatestAnswersForProject(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get answers")
		return
	}

	if len(answers) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "no_answers", "No answers to compile")
		return
	}

	// Build Q&A bundles
	qaBundles := make([]compiler.QABundle, 0, len(answers))
	for _, a := range answers {
		q, err := h.repo.GetQuestion(r.Context(), a.QuestionID)
		if err != nil {
			continue // Skip if question not found
		}
		qaBundles = append(qaBundles, compiler.QABundle{
			QuestionID:    q.ID,
			QuestionText:  q.Text,
			QuestionType:  string(q.Type),
			QuestionTags:  q.Tags,
			QuestionPaths: q.SpecPaths,
			AnswerID:      a.ID,
			AnswerValue:   a.Value,
			AnswerVersion: a.Version,
		})
	}

	// Get current spec if exists
	var currentSpec json.RawMessage
	if latestID, _ := h.repo.GetLatestSnapshotID(r.Context(), projectID); latestID != nil {
		if snap, err := h.repo.GetSnapshot(r.Context(), *latestID); err == nil {
			currentSpec = snap.Spec
		}
	}

	// Compile
	log.Printf("Compile: calling LLM with %d Q&A bundles (provider: %s, model: %s)", len(qaBundles), req.Provider, req.Model)
	output, err := h.compiler.Compile(r.Context(), compiler.CompileInput{
		Project:     project,
		QABundles:   qaBundles,
		CurrentSpec: currentSpec,
		Provider:    req.Provider,
		Model:       req.Model,
	})
	if err != nil {
		log.Printf("Compile: LLM error: %v", err)
		writeError(w, http.StatusUnprocessableEntity, "compilation_failed", err.Error())
		return
	}
	log.Printf("Compile: LLM returned successfully")

	// Create snapshot
	now := time.Now().UTC()
	snapshot := &domain.SpecSnapshot{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Spec:        output.Spec,
		CreatedAt:   now,
		DerivedFrom: output.DerivedFrom,
		Compiler:    output.Compiler,
	}

	if err := h.repo.CreateSnapshot(r.Context(), snapshot); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save snapshot")
		return
	}

	// Run validation and create issues
	issueDrafts, err := h.compiler.Validate(r.Context(), project, output.Spec, output.Trace, qaBundles)
	if err != nil {
		// Log but don't fail - validation is optional
		issueDrafts = nil
	}

	issues := compiler.HydrateIssues(issueDrafts, projectID, snapshot.ID)
	for _, issue := range issues {
		if err := h.repo.CreateIssue(r.Context(), issue); err != nil {
			// Log but don't fail
		}
	}

	// Update project timestamp
	project.UpdatedAt = now
	h.repo.UpdateProject(r.Context(), project)

	writeJSON(w, http.StatusOK, compileResponse{
		SnapshotID: snapshot.ID,
		Issues:     issues,
	})
}

// Next Questions

type nextQuestionsRequest struct {
	Count int `json:"count"`
}

type nextQuestionsResponse struct {
	Questions []*domain.Question `json:"questions"`
}

func (h *Handler) GenerateNextQuestions(w http.ResponseWriter, r *http.Request) {
	if h.compiler == nil {
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "LLM service not configured")
		return
	}

	idStr := r.PathValue("projectId")
	projectID, err := parseUUID(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	var req nextQuestionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	if req.Count < 1 || req.Count > 50 {
		req.Count = 5 // Default
	}

	// Get project
	project, err := h.repo.GetProject(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get project")
		return
	}

	// Convert project mode to compiler mode
	mode := compiler.ModeAdvanced
	if project.Mode == domain.ProjectModeBasic {
		mode = compiler.ModeBasic
	}
	log.Printf("GenerateNextQuestions: mode=%s (project.Mode=%s)", mode, project.Mode)

	// Get existing questions and answers
	questions, _ := h.repo.ListQuestions(r.Context(), projectID, nil, nil)
	answers, _ := h.repo.GetLatestAnswersForProject(r.Context(), projectID)

	// Get current spec and issues
	var currentSpec json.RawMessage
	var currentIssues []*domain.Issue
	if latestID, _ := h.repo.GetLatestSnapshotID(r.Context(), projectID); latestID != nil {
		if snap, err := h.repo.GetSnapshot(r.Context(), *latestID); err == nil {
			currentSpec = snap.Spec
		}
		currentIssues, _ = h.repo.ListIssuesForSnapshot(r.Context(), *latestID)
	}

	// Run planner
	planOutput, err := h.compiler.Plan(r.Context(), compiler.PlanInput{
		Project:           project,
		CurrentSpec:       currentSpec,
		CurrentIssues:     currentIssues,
		ExistingQuestions: questions,
		LatestAnswers:     answers,
		Mode:              mode,
	})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "planner_failed", err.Error())
		return
	}

	// Run asker with planner suggestions
	askOutput, err := h.compiler.Ask(r.Context(), compiler.AskInput{
		Project:            project,
		PlannerSuggestions: planOutput.Suggestions,
		CurrentSpec:        currentSpec,
		ExistingQuestions:  questions,
		LatestAnswers:      answers,
		Mode:               mode,
	})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "asker_failed", err.Error())
		return
	}

	// Persist generated questions (up to count)
	now := time.Now().UTC()
	newQuestions := make([]*domain.Question, 0, req.Count)
	for i, aq := range askOutput.Questions {
		if i >= req.Count {
			break
		}

		q := &domain.Question{
			ID:        uuid.New(),
			ProjectID: projectID,
			Text:      aq.Text,
			Type:      domain.QuestionType(aq.Type),
			Options:   aq.Options,
			Tags:      aq.Tags,
			Priority:  aq.Priority,
			SpecPaths: aq.SpecPaths,
			Status:    domain.QuestionStatusUnanswered,
			CreatedAt: now,
		}

		if err := h.repo.CreateQuestion(r.Context(), q); err != nil {
			continue // Skip on error
		}
		newQuestions = append(newQuestions, q)
	}

	writeJSON(w, http.StatusOK, nextQuestionsResponse{Questions: newQuestions})
}

// Suggestions

type suggestionsResponse struct {
	Suggestions []suggestionItem `json:"suggestions"`
}

type suggestionItem struct {
	QuestionID     string          `json:"question_id"`
	SuggestedValue json.RawMessage `json:"suggested_value"`
	Confidence     string          `json:"confidence"`
	Reasoning      string          `json:"reasoning"`
}

func (h *Handler) GenerateSuggestions(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.PathValue("projectId")
	projectID, err := parseUUID(projectIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	// Get project
	project, err := h.repo.GetProject(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	// Convert project mode to compiler mode
	mode := compiler.ModeAdvanced
	if project.Mode == domain.ProjectModeBasic {
		mode = compiler.ModeBasic
	}

	// Get all questions and filter for unanswered
	questions, err := h.repo.ListQuestions(r.Context(), projectID, nil, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	unanswered := make([]*domain.Question, 0)
	for _, q := range questions {
		if q.Status == domain.QuestionStatusUnanswered {
			unanswered = append(unanswered, q)
		}
	}

	if len(unanswered) == 0 {
		writeJSON(w, http.StatusOK, suggestionsResponse{Suggestions: []suggestionItem{}})
		return
	}

	// Get latest answers for context
	answers, _ := h.repo.GetLatestAnswersForProject(r.Context(), projectID)

	// Get current spec if available
	var currentSpec json.RawMessage
	if snapshotID, err := h.repo.GetLatestSnapshotID(r.Context(), projectID); err == nil && snapshotID != nil {
		if snapshot, err := h.repo.GetSnapshot(r.Context(), *snapshotID); err == nil {
			currentSpec = snapshot.Spec
		}
	}

	// Call suggester
	suggestOutput, err := h.compiler.Suggest(r.Context(), compiler.SuggestInput{
		Project:             project,
		UnansweredQuestions: unanswered,
		LatestAnswers:       answers,
		CurrentSpec:         currentSpec,
		Mode:                mode,
	})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "suggester_failed", err.Error())
		return
	}

	// Convert to response format
	suggestions := make([]suggestionItem, len(suggestOutput.Suggestions))
	for i, s := range suggestOutput.Suggestions {
		suggestions[i] = suggestionItem{
			QuestionID:     s.QuestionID,
			SuggestedValue: s.SuggestedValue,
			Confidence:     s.Confidence,
			Reasoning:      s.Reasoning,
		}
	}

	writeJSON(w, http.StatusOK, suggestionsResponse{Suggestions: suggestions})
}

// Diff

type diffResponse struct {
	Diff   *diff.Result         `json:"diff"`
	Impact *diff.ImpactAnalysis `json:"impact"`
}

func (h *Handler) DiffSnapshots(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.PathValue("projectId")
	projectID, err := parseUUID(projectIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	snapshotIDStr := r.PathValue("snapshotId")
	targetID, err := parseUUID(snapshotIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid snapshot ID format")
		return
	}

	// Get base snapshot ID from query params
	baseIDStr := r.URL.Query().Get("base")
	if baseIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "base snapshot ID is required")
		return
	}
	baseID, err := parseUUID(baseIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid base snapshot ID format")
		return
	}

	// Get target snapshot
	targetSnap, err := h.repo.GetSnapshot(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Target snapshot not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get target snapshot")
		return
	}
	if targetSnap.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "not_found", "Target snapshot not found in this project")
		return
	}

	// Get base snapshot
	baseSnap, err := h.repo.GetSnapshot(r.Context(), baseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Base snapshot not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get base snapshot")
		return
	}
	if baseSnap.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "not_found", "Base snapshot not found in this project")
		return
	}

	// Compute diff
	result, err := diff.Specs(baseSnap.Spec, targetSnap.Spec, baseID.String(), targetID.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "diff_error", "Failed to compute diff")
		return
	}

	// Analyze impact
	impact := diff.AnalyzeImpact(result)

	writeJSON(w, http.StatusOK, diffResponse{
		Diff:   result,
		Impact: impact,
	})
}

// Export

func (h *Handler) ExportPack(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.PathValue("projectId")
	projectID, err := parseUUID(projectIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid project ID format")
		return
	}

	// Get project
	project, err := h.repo.GetProject(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get project")
		return
	}

	// Get snapshot ID from query or use latest
	var snapshotID uuid.UUID
	if sidStr := r.URL.Query().Get("snapshot_id"); sidStr != "" {
		snapshotID, err = parseUUID(sidStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_uuid", "Invalid snapshot ID format")
			return
		}
	} else {
		latestID, err := h.repo.GetLatestSnapshotID(r.Context(), projectID)
		if err != nil || latestID == nil {
			writeError(w, http.StatusUnprocessableEntity, "no_snapshot", "No snapshot to export")
			return
		}
		snapshotID = *latestID
	}

	// Get snapshot
	snapshot, err := h.repo.GetSnapshot(r.Context(), snapshotID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Snapshot not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get snapshot")
		return
	}

	if snapshot.ProjectID != projectID {
		writeError(w, http.StatusNotFound, "not_found", "Snapshot not found in this project")
		return
	}

	// Build Q&A bundles for provenance
	qaBundles := make([]export.QABundle, 0)
	for qid, version := range snapshot.DerivedFrom {
		q, err := h.repo.GetQuestion(r.Context(), qid)
		if err != nil {
			continue
		}
		a, err := h.repo.GetAnswerByVersion(r.Context(), qid, version)
		if err != nil {
			continue
		}
		answerStr := string(a.Value)
		if len(answerStr) > 500 {
			answerStr = answerStr[:500] + "..."
		}
		qaBundles = append(qaBundles, export.QABundle{
			QuestionID:   q.ID,
			QuestionText: q.Text,
			AnswerID:     a.ID,
			AnswerValue:  answerStr,
			Version:      a.Version,
		})
	}

	// Generate pack
	input := export.Input{
		Project:   project,
		Snapshot:  snapshot,
		Trace:     snapshot.Trace,
		QABundles: qaBundles,
	}

	contents, err := export.GeneratePack(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_error", "Failed to generate export")
		return
	}

	// Write zip response
	var buf bytes.Buffer
	if err := export.WriteZip(contents, &buf); err != nil {
		writeError(w, http.StatusInternalServerError, "zip_error", "Failed to create zip")
		return
	}

	filename := fmt.Sprintf("%s-ai-coder-pack.zip", sanitizeFilename(project.Name))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.Write(buf.Bytes())
}

func sanitizeFilename(name string) string {
	// Replace unsafe characters
	safe := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			safe += string(r)
		} else if r == ' ' {
			safe += "-"
		}
	}
	if safe == "" {
		safe = "project"
	}
	return safe
}
