package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/google/uuid"
)

// PackContents holds all files for the AI Coder Pack.
type PackContents struct {
	SpecJSON     []byte
	SpecMD       []byte
	DecisionsMD  []byte
	AcceptanceMD []byte
	PlanMD       []byte
	PromptsMD    []byte
	TraceJSON    []byte
}

// Input holds input for generating an export.
type Input struct {
	Project   *domain.Project
	Snapshot  *domain.SpecSnapshot
	Trace     json.RawMessage
	QABundles []QABundle
}

// QABundle represents question-answer provenance info.
type QABundle struct {
	QuestionID   uuid.UUID `json:"question_id"`
	QuestionText string    `json:"question_text"`
	AnswerID     uuid.UUID `json:"answer_id"`
	AnswerValue  string    `json:"answer_value"`
	Version      int       `json:"version"`
}

// GeneratePack creates all files for the AI Coder Pack.
func GeneratePack(input Input) (*PackContents, error) {
	contents := &PackContents{}

	// SPEC.json - direct from snapshot
	specJSON, err := json.MarshalIndent(input.Snapshot.Spec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}
	contents.SpecJSON = specJSON

	// TRACE.json - provenance mapping
	traceJSON, err := json.MarshalIndent(input.Trace, "", "  ")
	if err != nil {
		traceJSON = []byte("{}")
	}
	contents.TraceJSON = traceJSON

	// SPEC.md - markdown render of spec
	contents.SpecMD = renderSpecMarkdown(input.Project, input.Snapshot.Spec)

	// DECISIONS.md - compiler metadata
	contents.DecisionsMD = renderDecisionsMarkdown(input.Snapshot, input.QABundles)

	// ACCEPTANCE.md - acceptance criteria from spec
	contents.AcceptanceMD = renderAcceptanceMarkdown(input.Snapshot.Spec)

	// PLAN.md - implementation plan from spec
	contents.PlanMD = renderPlanMarkdown(input.Snapshot.Spec)

	// PROMPTS.md - AI coder prompts for working with the spec
	contents.PromptsMD = renderPromptsMarkdown(input.Project)

	return contents, nil
}

// WriteZip writes the pack contents to a zip archive.
func WriteZip(contents *PackContents, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	files := map[string][]byte{
		"SPEC.json":     contents.SpecJSON,
		"SPEC.md":       contents.SpecMD,
		"DECISIONS.md":  contents.DecisionsMD,
		"ACCEPTANCE.md": contents.AcceptanceMD,
		"PLAN.md":       contents.PlanMD,
		"PROMPTS.md":    contents.PromptsMD,
		"TRACE.json":    contents.TraceJSON,
	}

	for name, data := range files {
		fw, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		if _, err := fw.Write(data); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}

func renderSpecMarkdown(project *domain.Project, spec json.RawMessage) []byte {
	var s map[string]interface{}
	if err := json.Unmarshal(spec, &s); err != nil {
		return []byte("# Specification\n\nError parsing spec.\n")
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("# %s - Specification\n\n", project.Name))
	buf.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	buf.WriteString("---\n\n")

	// Render each top-level section
	renderSection(&buf, s, 0)

	return buf.Bytes()
}

func renderSection(buf *bytes.Buffer, data interface{}, depth int) {
	prefix := strings.Repeat("#", depth+2)
	if depth > 4 {
		prefix = "######"
	}

	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			title := strings.ReplaceAll(key, "_", " ")
			title = strings.Title(title)
			buf.WriteString(fmt.Sprintf("%s %s\n\n", prefix, title))
			renderSection(buf, val, depth+1)
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				buf.WriteString(fmt.Sprintf("- %s\n", s))
			} else {
				renderSection(buf, item, depth)
			}
		}
		buf.WriteString("\n")
	case string:
		buf.WriteString(fmt.Sprintf("%s\n\n", v))
	case float64:
		buf.WriteString(fmt.Sprintf("%v\n\n", v))
	case bool:
		buf.WriteString(fmt.Sprintf("%v\n\n", v))
	default:
		buf.WriteString(fmt.Sprintf("%v\n\n", v))
	}
}

func renderDecisionsMarkdown(snapshot *domain.SpecSnapshot, qaBundles []QABundle) []byte {
	var buf bytes.Buffer

	buf.WriteString("# Decisions Log\n\n")
	buf.WriteString("This document records the key decisions made during specification development.\n\n")
	buf.WriteString("---\n\n")

	buf.WriteString("## Compiler Configuration\n\n")
	buf.WriteString(fmt.Sprintf("- **Model**: %s\n", snapshot.Compiler.Model))
	buf.WriteString(fmt.Sprintf("- **Prompt Version**: %s\n", snapshot.Compiler.PromptVersion))
	buf.WriteString(fmt.Sprintf("- **Temperature**: %.2f\n", snapshot.Compiler.Temperature))
	buf.WriteString(fmt.Sprintf("- **Snapshot ID**: %s\n", snapshot.ID))
	buf.WriteString(fmt.Sprintf("- **Created**: %s\n\n", snapshot.CreatedAt.Format(time.RFC3339)))

	buf.WriteString("## Question-Answer Provenance\n\n")
	buf.WriteString("The following questions and answers were used to derive this specification:\n\n")

	for i, qa := range qaBundles {
		buf.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, qa.QuestionText))
		buf.WriteString(fmt.Sprintf("**Answer (v%d)**: %s\n\n", qa.Version, qa.AnswerValue))
	}

	return buf.Bytes()
}

func renderAcceptanceMarkdown(spec json.RawMessage) []byte {
	var s map[string]interface{}
	if err := json.Unmarshal(spec, &s); err != nil {
		return []byte("# Acceptance Criteria\n\nNo acceptance criteria found.\n")
	}

	var buf bytes.Buffer
	buf.WriteString("# Acceptance Criteria\n\n")
	buf.WriteString("This document defines the acceptance criteria for validating the implementation.\n\n")
	buf.WriteString("---\n\n")

	// Try to extract acceptance from spec
	if acceptance, ok := s["acceptance"].(map[string]interface{}); ok {
		if criteria, ok := acceptance["criteria"].([]interface{}); ok {
			buf.WriteString("## Criteria\n\n")
			for i, c := range criteria {
				if cs, ok := c.(string); ok {
					buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, cs))
				}
			}
			buf.WriteString("\n")
		}
	}

	// Extract from non_functionals if present
	if nf, ok := s["non_functionals"].(map[string]interface{}); ok {
		buf.WriteString("## Non-Functional Requirements\n\n")
		for key, val := range nf {
			title := strings.ReplaceAll(key, "_", " ")
			title = strings.Title(title)
			buf.WriteString(fmt.Sprintf("### %s\n\n", title))
			if arr, ok := val.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						buf.WriteString(fmt.Sprintf("- %s\n", s))
					}
				}
			}
			buf.WriteString("\n")
		}
	}

	return buf.Bytes()
}

func renderPlanMarkdown(spec json.RawMessage) []byte {
	var s map[string]interface{}
	if err := json.Unmarshal(spec, &s); err != nil {
		return []byte("# Implementation Plan\n\nNo plan found.\n")
	}

	var buf bytes.Buffer
	buf.WriteString("# Implementation Plan\n\n")
	buf.WriteString("This document outlines the implementation strategy for the specified product.\n\n")
	buf.WriteString("---\n\n")

	// Try to extract plan from spec
	if plan, ok := s["plan"].(map[string]interface{}); ok {
		if phases, ok := plan["phases"].([]interface{}); ok {
			buf.WriteString("## Phases\n\n")
			for i, phase := range phases {
				if pm, ok := phase.(map[string]interface{}); ok {
					name := pm["name"]
					buf.WriteString(fmt.Sprintf("### Phase %d: %v\n\n", i+1, name))
					if tasks, ok := pm["tasks"].([]interface{}); ok {
						for _, task := range tasks {
							if ts, ok := task.(string); ok {
								buf.WriteString(fmt.Sprintf("- [ ] %s\n", ts))
							}
						}
					}
					buf.WriteString("\n")
				}
			}
		}
	}

	// Extract milestones if present
	if milestones, ok := s["milestones"].([]interface{}); ok {
		buf.WriteString("## Milestones\n\n")
		for i, m := range milestones {
			if mm, ok := m.(map[string]interface{}); ok {
				name := mm["name"]
				buf.WriteString(fmt.Sprintf("%d. **%v**\n", i+1, name))
				if desc, ok := mm["description"].(string); ok {
					buf.WriteString(fmt.Sprintf("   %s\n", desc))
				}
			} else if ms, ok := m.(string); ok {
				buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, ms))
			}
		}
		buf.WriteString("\n")
	}

	// Provide basic structure if no plan found
	if _, ok := s["plan"]; !ok {
		buf.WriteString("## Suggested Implementation Order\n\n")
		buf.WriteString("Based on the specification, consider implementing in this order:\n\n")
		buf.WriteString("1. **Data Model** - Set up database schema and entities\n")
		buf.WriteString("2. **API Layer** - Implement backend endpoints\n")
		buf.WriteString("3. **Core Logic** - Implement business workflows\n")
		buf.WriteString("4. **UI Components** - Build frontend interfaces\n")
		buf.WriteString("5. **Integration** - Connect all layers\n")
		buf.WriteString("6. **Testing** - Comprehensive test coverage\n\n")
	}

	return buf.Bytes()
}

func renderPromptsMarkdown(project *domain.Project) []byte {
	var buf bytes.Buffer

	buf.WriteString("# AI Coder Prompts\n\n")
	buf.WriteString("Use these prompts with your AI coding assistant to implement this specification.\n\n")
	buf.WriteString("---\n\n")

	// Getting Started
	buf.WriteString("## Getting Started\n\n")
	buf.WriteString("### Understand the Spec\n\n")
	buf.WriteString("```\n")
	buf.WriteString("I'm starting a new project. Please read SPEC.json and SPEC.md to understand the full requirements.\n")
	buf.WriteString("Summarize:\n")
	buf.WriteString("1. The core product purpose and target users\n")
	buf.WriteString("2. The main features and workflows\n")
	buf.WriteString("3. The data model and API structure\n")
	buf.WriteString("4. Any technical constraints or non-functional requirements\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Create Project Structure\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Based on SPEC.json, create the initial project structure including:\n")
	buf.WriteString("- Directory layout following best practices for the chosen stack\n")
	buf.WriteString("- Package/module organization matching the spec's domains\n")
	buf.WriteString("- Configuration files (package.json, go.mod, etc.)\n")
	buf.WriteString("- README with setup instructions\n")
	buf.WriteString("```\n\n")

	// Implementation Prompts
	buf.WriteString("## Implementation\n\n")

	buf.WriteString("### Implement Data Model\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Using the data_model section of SPEC.json, implement:\n")
	buf.WriteString("1. Entity/model definitions with all fields and types\n")
	buf.WriteString("2. Database schema or migrations\n")
	buf.WriteString("3. Validation rules as specified\n")
	buf.WriteString("4. Relationships between entities\n")
	buf.WriteString("Follow the exact field names and types from the spec.\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Implement API Endpoints\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Using the api section of SPEC.json, implement the REST/GraphQL endpoints:\n")
	buf.WriteString("1. Create route handlers for each endpoint\n")
	buf.WriteString("2. Implement request validation matching the spec\n")
	buf.WriteString("3. Return responses in the exact format specified\n")
	buf.WriteString("4. Handle errors with appropriate status codes\n")
	buf.WriteString("Reference: See api.endpoints in SPEC.json for the complete API contract.\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Implement Workflow: [Name]\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Implement the [workflow_name] workflow from SPEC.json:\n")
	buf.WriteString("1. Follow the exact steps defined in workflows.[name].steps\n")
	buf.WriteString("2. Handle all specified error conditions\n")
	buf.WriteString("3. Implement any business rules mentioned\n")
	buf.WriteString("4. Add appropriate logging and error handling\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Implement UI Component\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Using the ui section of SPEC.json, implement the [component_name] component:\n")
	buf.WriteString("1. Match the layout and structure described\n")
	buf.WriteString("2. Include all specified interactive elements\n")
	buf.WriteString("3. Connect to the API endpoints as specified\n")
	buf.WriteString("4. Handle loading, error, and empty states\n")
	buf.WriteString("```\n\n")

	// Quality Prompts
	buf.WriteString("## Quality & Review\n\n")

	buf.WriteString("### Review Against Spec\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Review this implementation against SPEC.json:\n")
	buf.WriteString("1. Does it implement all required features?\n")
	buf.WriteString("2. Does the data model match the spec exactly?\n")
	buf.WriteString("3. Do API responses match the specified format?\n")
	buf.WriteString("4. Are all validation rules implemented?\n")
	buf.WriteString("5. Are there any deviations that need justification?\n")
	buf.WriteString("[paste code here]\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Generate Tests\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Using ACCEPTANCE.md and SPEC.json, generate tests that verify:\n")
	buf.WriteString("1. Each acceptance criterion is testable and tested\n")
	buf.WriteString("2. All API endpoints return correct responses\n")
	buf.WriteString("3. Data validation rules are enforced\n")
	buf.WriteString("4. Workflow steps execute in correct order\n")
	buf.WriteString("5. Error conditions are handled properly\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Check Non-Functional Requirements\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Review the implementation against non_functionals in SPEC.json:\n")
	buf.WriteString("1. Performance: Are there any obvious bottlenecks?\n")
	buf.WriteString("2. Security: Are inputs validated? Auth implemented?\n")
	buf.WriteString("3. Scalability: Can this handle the expected load?\n")
	buf.WriteString("4. Accessibility: Does the UI meet accessibility standards?\n")
	buf.WriteString("```\n\n")

	// Debugging Prompts
	buf.WriteString("## Debugging & Issues\n\n")

	buf.WriteString("### Debug Against Spec\n\n")
	buf.WriteString("```\n")
	buf.WriteString("I'm seeing [describe issue]. Check against SPEC.json:\n")
	buf.WriteString("1. What does the spec say this behavior should be?\n")
	buf.WriteString("2. Is my implementation following the spec correctly?\n")
	buf.WriteString("3. Is this a spec ambiguity or implementation bug?\n")
	buf.WriteString("[paste relevant code and error]\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Trace Decision\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Using TRACE.json and DECISIONS.md, explain why [feature/behavior] was designed this way.\n")
	buf.WriteString("What were the original questions and answers that led to this decision?\n")
	buf.WriteString("```\n\n")

	// Continuation Prompts
	buf.WriteString("## Continuation & Handoff\n\n")

	buf.WriteString("### Continue Implementation\n\n")
	buf.WriteString("```\n")
	buf.WriteString("I'm continuing work on this project. Read SPEC.json, PLAN.md, and the current codebase.\n")
	buf.WriteString("1. What has been implemented so far?\n")
	buf.WriteString("2. What remains to be done according to the spec?\n")
	buf.WriteString("3. What should I work on next?\n")
	buf.WriteString("```\n\n")

	buf.WriteString("### Handoff Summary\n\n")
	buf.WriteString("```\n")
	buf.WriteString("Generate a handoff document that includes:\n")
	buf.WriteString("1. Implementation status vs SPEC.json (what's done, what's remaining)\n")
	buf.WriteString("2. Any deviations from the spec and why\n")
	buf.WriteString("3. Known issues or technical debt\n")
	buf.WriteString("4. Recommended next steps\n")
	buf.WriteString("```\n\n")

	// Quick Reference
	buf.WriteString("## Quick Reference\n\n")
	buf.WriteString("| File | Purpose |\n")
	buf.WriteString("|------|--------|\n")
	buf.WriteString("| SPEC.json | Machine-readable full specification |\n")
	buf.WriteString("| SPEC.md | Human-readable specification |\n")
	buf.WriteString("| PLAN.md | Implementation phases and tasks |\n")
	buf.WriteString("| ACCEPTANCE.md | Success criteria and test cases |\n")
	buf.WriteString("| DECISIONS.md | Q&A history and rationale |\n")
	buf.WriteString("| TRACE.json | Maps spec fields to source answers |\n")
	buf.WriteString("\n")

	buf.WriteString("---\n\n")
	buf.WriteString(fmt.Sprintf("*Generated for: %s*\n", project.Name))

	return buf.Bytes()
}
