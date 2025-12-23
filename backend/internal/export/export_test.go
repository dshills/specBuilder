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
