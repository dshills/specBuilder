package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/google/uuid"
)

func TestGeneratePack(t *testing.T) {
	project := &domain.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	spec := json.RawMessage(`{
		"product": {"name": "Test", "purpose": "Testing"},
		"non_functionals": {"security": ["Encrypt at rest"]}
	}`)

	snapshot := &domain.SpecSnapshot{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Spec:        spec,
		CreatedAt:   time.Now().UTC(),
		DerivedFrom: map[uuid.UUID]int{},
		Compiler: domain.CompilerConfig{
			Model:         "gpt-4o",
			PromptVersion: "v1",
			Temperature:   0,
		},
	}

	input := Input{
		Project:  project,
		Snapshot: snapshot,
		Trace:    json.RawMessage(`{}`),
		QABundles: []QABundle{
			{
				QuestionID:   uuid.New(),
				QuestionText: "What is the product name?",
				AnswerID:     uuid.New(),
				AnswerValue:  "Test",
				Version:      1,
			},
		},
	}

	contents, err := GeneratePack(input)
	if err != nil {
		t.Fatalf("GeneratePack failed: %v", err)
	}

	if len(contents.SpecJSON) == 0 {
		t.Error("SpecJSON is empty")
	}
	if len(contents.SpecMD) == 0 {
		t.Error("SpecMD is empty")
	}
	if len(contents.DecisionsMD) == 0 {
		t.Error("DecisionsMD is empty")
	}
	if len(contents.PlanMD) == 0 {
		t.Error("PlanMD is empty")
	}
}

func TestWriteZip(t *testing.T) {
	contents := &PackContents{
		SpecJSON:     []byte(`{"test": true}`),
		SpecMD:       []byte("# Test"),
		DecisionsMD:  []byte("# Decisions"),
		AcceptanceMD: []byte("# Acceptance"),
		PlanMD:       []byte("# Plan"),
		TraceJSON:    []byte(`{}`),
	}

	var buf bytes.Buffer
	err := WriteZip(contents, &buf)
	if err != nil {
		t.Fatalf("WriteZip failed: %v", err)
	}

	// Verify zip contents
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	expectedFiles := map[string]bool{
		"SPEC.json":     false,
		"SPEC.md":       false,
		"DECISIONS.md":  false,
		"ACCEPTANCE.md": false,
		"PLAN.md":       false,
		"TRACE.json":    false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("Missing file in zip: %s", name)
		}
	}
}

func TestGenerateRalphPack(t *testing.T) {
	project := &domain.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	spec := json.RawMessage(`{
		"product": {"name": "Test App", "purpose": "A test application for validating exports", "success_criteria": ["Works correctly", "Has tests"]},
		"scope": {"in_scope": ["Feature A", "Feature B"], "assumptions": ["Node.js environment"]},
		"requirements": {
			"functional": [
				{"name": "User auth", "description": "Users can log in"},
				{"name": "Data storage", "description": "Persist user data"}
			]
		},
		"plan": {
			"phases": [
				{"name": "MVP", "tasks": ["Set up project", "Implement auth"]},
				{"name": "Beta", "tasks": ["Add features", "Write tests"]}
			]
		},
		"non_functionals": {"security": ["Encrypt passwords"], "performance": ["Load under 2s"]},
		"acceptance": {"criteria": ["All tests pass", "No security vulnerabilities"]}
	}`)

	snapshot := &domain.SpecSnapshot{
		ID:          uuid.New(),
		ProjectID:   project.ID,
		Spec:        spec,
		CreatedAt:   time.Now().UTC(),
		DerivedFrom: map[uuid.UUID]int{},
		Compiler: domain.CompilerConfig{
			Model:         "gpt-4o",
			PromptVersion: "v1",
			Temperature:   0,
		},
	}

	input := Input{
		Project:   project,
		Snapshot:  snapshot,
		Trace:     json.RawMessage(`{}`),
		QABundles: []QABundle{},
	}

	contents, err := GenerateRalphPack(input)
	if err != nil {
		t.Fatalf("GenerateRalphPack failed: %v", err)
	}

	// Verify all files are generated
	if len(contents.PromptMD) == 0 {
		t.Error("PromptMD is empty")
	}
	if len(contents.FixPlanMD) == 0 {
		t.Error("FixPlanMD is empty")
	}
	if len(contents.RequirementsMD) == 0 {
		t.Error("RequirementsMD is empty")
	}
	if len(contents.AgentMD) == 0 {
		t.Error("AgentMD is empty")
	}
	if len(contents.SpecJSON) == 0 {
		t.Error("SpecJSON is empty")
	}

	// Verify PROMPT.md contains key Ralph elements
	promptStr := string(contents.PromptMD)
	if !bytes.Contains(contents.PromptMD, []byte("Ralph Development Instructions")) {
		t.Error("PromptMD missing Ralph header")
	}
	if !bytes.Contains(contents.PromptMD, []byte("RALPH_STATUS")) {
		t.Error("PromptMD missing status reporting block")
	}
	if !bytes.Contains(contents.PromptMD, []byte("EXIT_SIGNAL")) {
		t.Error("PromptMD missing EXIT_SIGNAL instructions")
	}
	if !bytes.Contains(contents.PromptMD, []byte("Key Principles")) {
		t.Error("PromptMD missing Key Principles section")
	}
	_ = promptStr // prevent unused variable warning

	// Verify @fix_plan.md contains priority sections
	if !bytes.Contains(contents.FixPlanMD, []byte("High Priority")) {
		t.Error("FixPlanMD missing High Priority section")
	}
	if !bytes.Contains(contents.FixPlanMD, []byte("Medium Priority")) {
		t.Error("FixPlanMD missing Medium Priority section")
	}
	if !bytes.Contains(contents.FixPlanMD, []byte("Completed")) {
		t.Error("FixPlanMD missing Completed section")
	}
	if !bytes.Contains(contents.FixPlanMD, []byte("- [ ]")) {
		t.Error("FixPlanMD missing checkbox format")
	}

	// Verify specs/requirements.md contains technical content
	if !bytes.Contains(contents.RequirementsMD, []byte("Technical Specifications")) {
		t.Error("RequirementsMD missing header")
	}
	if !bytes.Contains(contents.RequirementsMD, []byte("Test Project")) {
		t.Error("RequirementsMD missing project name")
	}

	// Verify @AGENT.md contains quality gates
	if !bytes.Contains(contents.AgentMD, []byte("Agent Build Instructions")) {
		t.Error("AgentMD missing header")
	}
	if !bytes.Contains(contents.AgentMD, []byte("Feature Development Quality Standards")) {
		t.Error("AgentMD missing quality standards section")
	}
	if !bytes.Contains(contents.AgentMD, []byte("Feature Completion Checklist")) {
		t.Error("AgentMD missing completion checklist")
	}
}

func TestWriteRalphZip(t *testing.T) {
	contents := &RalphPackContents{
		PromptMD:       []byte("# Ralph Development Instructions"),
		FixPlanMD:      []byte("# Ralph Fix Plan"),
		RequirementsMD: []byte("# Technical Specifications"),
		AgentMD:        []byte("# Agent Build Instructions"),
		SpecJSON:       []byte(`{"test": true}`),
	}

	var buf bytes.Buffer
	err := WriteRalphZip(contents, &buf)
	if err != nil {
		t.Fatalf("WriteRalphZip failed: %v", err)
	}

	// Verify zip contents
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Failed to read zip: %v", err)
	}

	expectedFiles := map[string]bool{
		"PROMPT.md":             false,
		"@fix_plan.md":          false,
		"@AGENT.md":             false,
		"specs/requirements.md": false,
		"specs/SPEC.json":       false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("Missing file in Ralph zip: %s", name)
		}
	}
}

func TestRalphFixPlanExtractsTasks(t *testing.T) {
	// Test that tasks are properly extracted from spec plan
	spec := json.RawMessage(`{
		"plan": {
			"phases": [
				{"name": "Phase 1", "tasks": ["Task A", "Task B"]},
				{"name": "Phase 2", "tasks": ["Task C"]}
			]
		}
	}`)

	contents := renderRalphFixPlanMD(spec)

	// Should contain tasks from phases
	if !bytes.Contains(contents, []byte("[Phase 1] Task A")) {
		t.Error("FixPlan should contain Phase 1 tasks")
	}
	if !bytes.Contains(contents, []byte("[Phase 2] Task C")) {
		t.Error("FixPlan should contain Phase 2 tasks")
	}
}

func TestRalphPromptExtractsObjectives(t *testing.T) {
	project := &domain.Project{
		ID:   uuid.New(),
		Name: "My App",
	}

	spec := json.RawMessage(`{
		"product": {"purpose": "Build an amazing app"},
		"requirements": {
			"functional": [
				{"description": "User authentication"},
				{"description": "Data storage"}
			]
		}
	}`)

	contents := renderRalphPromptMD(project, spec)

	// Should contain product purpose in objectives
	if !bytes.Contains(contents, []byte("Implement: Build an amazing app")) {
		t.Error("Prompt should contain product purpose as objective")
	}
	// Should contain requirements as objectives
	if !bytes.Contains(contents, []byte("User authentication")) {
		t.Error("Prompt should contain functional requirements")
	}
}

func TestTechStackDetection(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]interface{}
		expected string
	}{
		{
			name: "Node from assumptions",
			spec: map[string]interface{}{
				"scope": map[string]interface{}{
					"assumptions": []interface{}{"Uses Node.js and TypeScript"},
				},
			},
			expected: "node",
		},
		{
			name: "Go from assumptions",
			spec: map[string]interface{}{
				"scope": map[string]interface{}{
					"assumptions": []interface{}{"Built with golang"},
				},
			},
			expected: "go",
		},
		{
			name: "Python from assumptions",
			spec: map[string]interface{}{
				"scope": map[string]interface{}{
					"assumptions": []interface{}{"Django web framework"},
				},
			},
			expected: "python",
		},
		{
			name:     "Generic fallback",
			spec:     map[string]interface{}{},
			expected: "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectTechStack(tt.spec)
			if result != tt.expected {
				t.Errorf("detectTechStack() = %s, want %s", result, tt.expected)
			}
		})
	}
}
