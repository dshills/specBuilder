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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// titleCaser is used to title-case strings (replacement for deprecated strings.Title).
var titleCaser = cases.Title(language.English)

// ExportFormat represents the type of export package.
type ExportFormat string

const (
	FormatDefault ExportFormat = "default"
	FormatRalph   ExportFormat = "ralph"
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
			title = titleCaser.String(title)
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
			title = titleCaser.String(title)
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

// =============================================================================
// Ralph Format Export
// =============================================================================

// RalphPackContents holds all files for the Ralph format export.
type RalphPackContents struct {
	PromptMD       []byte // PROMPT.md - Agent instructions
	FixPlanMD      []byte // @fix_plan.md - Task prioritization
	RequirementsMD []byte // specs/requirements.md - Technical specs
	AgentMD        []byte // @AGENT.md - Build instructions
	SpecJSON       []byte // specs/SPEC.json - Full spec for reference
}

// GenerateRalphPack creates all files for the Ralph format export.
func GenerateRalphPack(input Input) (*RalphPackContents, error) {
	contents := &RalphPackContents{}

	// SPEC.json - direct from snapshot (for reference)
	specJSON, err := json.MarshalIndent(input.Snapshot.Spec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}
	contents.SpecJSON = specJSON

	// PROMPT.md - Ralph agent instructions
	contents.PromptMD = renderRalphPromptMD(input.Project, input.Snapshot.Spec)

	// @fix_plan.md - Task prioritization from spec plan
	contents.FixPlanMD = renderRalphFixPlanMD(input.Snapshot.Spec)

	// specs/requirements.md - Technical specifications
	contents.RequirementsMD = renderRalphRequirementsMD(input.Project, input.Snapshot.Spec)

	// @AGENT.md - Build instructions and quality gates
	contents.AgentMD = renderRalphAgentMD(input.Project, input.Snapshot.Spec)

	return contents, nil
}

// WriteRalphZip writes the Ralph pack contents to a zip archive.
func WriteRalphZip(contents *RalphPackContents, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// Create directory structure in zip
	files := map[string][]byte{
		"PROMPT.md":             contents.PromptMD,
		"@fix_plan.md":          contents.FixPlanMD,
		"@AGENT.md":             contents.AgentMD,
		"specs/requirements.md": contents.RequirementsMD,
		"specs/SPEC.json":       contents.SpecJSON,
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

// renderRalphPromptMD generates the PROMPT.md file for Ralph format.
// This follows the Ralph best practices for agent instruction files.
func renderRalphPromptMD(project *domain.Project, spec json.RawMessage) []byte {
	var s map[string]interface{}
	if err := json.Unmarshal(spec, &s); err != nil {
		return []byte("# Ralph Development Instructions\n\nError parsing spec.\n")
	}

	var buf bytes.Buffer

	buf.WriteString("# Ralph Development Instructions\n\n")

	// Context section
	buf.WriteString("## Context\n")
	projectType := "software"
	if product, ok := s["product"].(map[string]interface{}); ok {
		if purpose, ok := product["purpose"].(string); ok && purpose != "" {
			projectType = purpose
		}
	}
	buf.WriteString(fmt.Sprintf("You are Ralph, an autonomous AI development agent working on %s - %s.\n\n", project.Name, projectType))

	// Current Objectives - extracted from spec
	buf.WriteString("## Current Objectives\n")
	objectives := extractObjectives(s)
	for i, obj := range objectives {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, obj))
	}
	buf.WriteString("\n")

	// Key Principles - Ralph best practices
	buf.WriteString("## Key Principles\n")
	buf.WriteString("- ONE task per loop - focus on the most important thing\n")
	buf.WriteString("- Search the codebase before assuming something isn't implemented\n")
	buf.WriteString("- Use subagents for expensive operations (file searching, analysis)\n")
	buf.WriteString("- Write comprehensive tests with clear documentation\n")
	buf.WriteString("- Update @fix_plan.md with your learnings\n")
	buf.WriteString("- Commit working changes with descriptive messages\n\n")

	// Testing Guidelines
	buf.WriteString("## Testing Guidelines (CRITICAL)\n")
	buf.WriteString("- LIMIT testing to ~20% of your total effort per loop\n")
	buf.WriteString("- PRIORITIZE: Implementation > Documentation > Tests\n")
	buf.WriteString("- Only write tests for NEW functionality you implement\n")
	buf.WriteString("- Do NOT refactor existing tests unless broken\n")
	buf.WriteString("- Do NOT add \"additional test coverage\" as busy work\n")
	buf.WriteString("- Focus on CORE functionality first, comprehensive testing later\n\n")

	// Execution Guidelines
	buf.WriteString("## Execution Guidelines\n")
	buf.WriteString("- Before making changes: search codebase using subagents\n")
	buf.WriteString("- After implementation: run ESSENTIAL tests for the modified code only\n")
	buf.WriteString("- If tests fail: fix them as part of your current work\n")
	buf.WriteString("- Keep @AGENT.md updated with build/run instructions\n")
	buf.WriteString("- Document the WHY behind tests and implementations\n")
	buf.WriteString("- No placeholder implementations - build it properly\n\n")

	// Status Reporting
	buf.WriteString("## Status Reporting (CRITICAL)\n\n")
	buf.WriteString("**IMPORTANT**: At the end of your response, ALWAYS include this status block:\n\n")
	buf.WriteString("```\n")
	buf.WriteString("---RALPH_STATUS---\n")
	buf.WriteString("STATUS: IN_PROGRESS | COMPLETE | BLOCKED\n")
	buf.WriteString("TASKS_COMPLETED_THIS_LOOP: <number>\n")
	buf.WriteString("FILES_MODIFIED: <number>\n")
	buf.WriteString("TESTS_STATUS: PASSING | FAILING | NOT_RUN\n")
	buf.WriteString("WORK_TYPE: IMPLEMENTATION | TESTING | DOCUMENTATION | REFACTORING\n")
	buf.WriteString("EXIT_SIGNAL: false | true\n")
	buf.WriteString("RECOMMENDATION: <one line summary of what to do next>\n")
	buf.WriteString("---END_RALPH_STATUS---\n")
	buf.WriteString("```\n\n")

	// When to set EXIT_SIGNAL
	buf.WriteString("### When to set EXIT_SIGNAL: true\n\n")
	buf.WriteString("Set EXIT_SIGNAL to **true** when ALL of these conditions are met:\n")
	buf.WriteString("1. All items in @fix_plan.md are marked [x]\n")
	buf.WriteString("2. All tests are passing (or no tests exist for valid reasons)\n")
	buf.WriteString("3. No errors or warnings in the last execution\n")
	buf.WriteString("4. All requirements from specs/ are implemented\n")
	buf.WriteString("5. You have nothing meaningful left to implement\n\n")

	// What NOT to do
	buf.WriteString("### What NOT to do:\n")
	buf.WriteString("- Do NOT continue with busy work when EXIT_SIGNAL should be true\n")
	buf.WriteString("- Do NOT run tests repeatedly without implementing new features\n")
	buf.WriteString("- Do NOT refactor code that is already working fine\n")
	buf.WriteString("- Do NOT add features not in the specifications\n")
	buf.WriteString("- Do NOT forget to include the status block (Ralph depends on it!)\n\n")

	// File Structure
	buf.WriteString("## File Structure\n")
	buf.WriteString("- specs/: Project specifications and requirements\n")
	buf.WriteString("- src/: Source code implementation\n")
	buf.WriteString("- examples/: Example usage and test cases\n")
	buf.WriteString("- @fix_plan.md: Prioritized TODO list\n")
	buf.WriteString("- @AGENT.md: Project build and run instructions\n\n")

	// Technical Constraints from spec
	if constraints := extractConstraints(s); len(constraints) > 0 {
		buf.WriteString("## Technical Constraints\n")
		for _, c := range constraints {
			buf.WriteString(fmt.Sprintf("- %s\n", c))
		}
		buf.WriteString("\n")
	}

	// Success Criteria from spec
	if criteria := extractSuccessCriteria(s); len(criteria) > 0 {
		buf.WriteString("## Success Criteria\n")
		for _, c := range criteria {
			buf.WriteString(fmt.Sprintf("- %s\n", c))
		}
		buf.WriteString("\n")
	}

	// Current Task
	buf.WriteString("## Current Task\n")
	buf.WriteString("Follow @fix_plan.md and choose the most important item to implement next.\n")
	buf.WriteString("Use your judgment to prioritize what will have the biggest impact on project progress.\n\n")

	buf.WriteString("Remember: Quality over speed. Build it right the first time. Know when you're done.\n")

	return buf.Bytes()
}

// renderRalphFixPlanMD generates the @fix_plan.md file for Ralph format.
func renderRalphFixPlanMD(spec json.RawMessage) []byte {
	var s map[string]interface{}
	if err := json.Unmarshal(spec, &s); err != nil {
		return []byte("# Ralph Fix Plan\n\n## High Priority\n- [ ] Review specs and begin implementation\n")
	}

	var buf bytes.Buffer
	buf.WriteString("# Ralph Fix Plan\n\n")

	highPriority := []string{}
	mediumPriority := []string{}
	lowPriority := []string{}

	// Extract tasks from plan phases
	if plan, ok := s["plan"].(map[string]interface{}); ok {
		if phases, ok := plan["phases"].([]interface{}); ok {
			for i, phase := range phases {
				if pm, ok := phase.(map[string]interface{}); ok {
					phaseName := ""
					if name, ok := pm["name"].(string); ok {
						phaseName = name
					}
					if tasks, ok := pm["tasks"].([]interface{}); ok {
						for _, task := range tasks {
							if ts, ok := task.(string); ok {
								taskStr := ts
								if phaseName != "" {
									taskStr = fmt.Sprintf("[%s] %s", phaseName, ts)
								}
								// First phase is high priority, second is medium, rest are low
								switch {
								case i == 0:
									highPriority = append(highPriority, taskStr)
								case i == 1:
									mediumPriority = append(mediumPriority, taskStr)
								default:
									lowPriority = append(lowPriority, taskStr)
								}
							}
						}
					}
				}
			}
		}

		// Also check for milestones
		if milestones, ok := plan["milestones"].([]interface{}); ok {
			for _, m := range milestones {
				if mm, ok := m.(map[string]interface{}); ok {
					if name, ok := mm["name"].(string); ok {
						mediumPriority = append(mediumPriority, fmt.Sprintf("Complete milestone: %s", name))
					}
				}
			}
		}
	}

	// Extract functional requirements as tasks if no plan found
	if len(highPriority) == 0 {
		if reqs, ok := s["requirements"].(map[string]interface{}); ok {
			if functional, ok := reqs["functional"].([]interface{}); ok {
				for i, req := range functional {
					var reqStr string
					if rs, ok := req.(string); ok {
						reqStr = rs
					} else if rm, ok := req.(map[string]interface{}); ok {
						if desc, ok := rm["description"].(string); ok {
							reqStr = desc
						} else if name, ok := rm["name"].(string); ok {
							reqStr = name
						}
					}
					if reqStr != "" {
						if i < 4 {
							highPriority = append(highPriority, fmt.Sprintf("Implement: %s", reqStr))
						} else if i < 8 {
							mediumPriority = append(mediumPriority, fmt.Sprintf("Implement: %s", reqStr))
						} else {
							lowPriority = append(lowPriority, fmt.Sprintf("Implement: %s", reqStr))
						}
					}
				}
			}
		}
	}

	// Add default tasks if still empty
	if len(highPriority) == 0 {
		highPriority = []string{
			"Set up basic project structure and build system",
			"Define core data structures and types",
			"Implement basic input/output handling",
			"Create test framework and initial tests",
		}
	}
	if len(mediumPriority) == 0 {
		mediumPriority = []string{
			"Add error handling and validation",
			"Implement core business logic",
			"Add configuration management",
			"Create user documentation",
		}
	}
	if len(lowPriority) == 0 {
		lowPriority = []string{
			"Performance optimization",
			"Extended feature set",
			"Integration with external services",
			"Advanced error recovery",
		}
	}

	// Write sections
	buf.WriteString("## High Priority\n")
	for _, task := range highPriority {
		buf.WriteString(fmt.Sprintf("- [ ] %s\n", task))
	}
	buf.WriteString("\n")

	buf.WriteString("## Medium Priority\n")
	for _, task := range mediumPriority {
		buf.WriteString(fmt.Sprintf("- [ ] %s\n", task))
	}
	buf.WriteString("\n")

	buf.WriteString("## Low Priority\n")
	for _, task := range lowPriority {
		buf.WriteString(fmt.Sprintf("- [ ] %s\n", task))
	}
	buf.WriteString("\n")

	buf.WriteString("## Completed\n")
	buf.WriteString("- [x] Project initialization\n")
	buf.WriteString("- [x] Specification generation\n\n")

	buf.WriteString("## Notes\n")
	buf.WriteString("- Focus on MVP functionality first\n")
	buf.WriteString("- Ensure each feature is properly tested\n")
	buf.WriteString("- Update this file after each major milestone\n")

	return buf.Bytes()
}

// renderRalphRequirementsMD generates the specs/requirements.md file for Ralph format.
func renderRalphRequirementsMD(project *domain.Project, spec json.RawMessage) []byte {
	var s map[string]interface{}
	if err := json.Unmarshal(spec, &s); err != nil {
		return []byte("# Technical Specifications\n\nError parsing spec.\n")
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("# Technical Specifications: %s\n\n", project.Name))
	buf.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	buf.WriteString("---\n\n")

	// Product Overview
	if product, ok := s["product"].(map[string]interface{}); ok {
		buf.WriteString("## Product Overview\n\n")
		if name, ok := product["name"].(string); ok {
			buf.WriteString(fmt.Sprintf("**Name**: %s\n\n", name))
		}
		if purpose, ok := product["purpose"].(string); ok {
			buf.WriteString(fmt.Sprintf("**Purpose**: %s\n\n", purpose))
		}
		if criteria, ok := product["success_criteria"].([]interface{}); ok {
			buf.WriteString("**Success Criteria**:\n")
			for _, c := range criteria {
				if cs, ok := c.(string); ok {
					buf.WriteString(fmt.Sprintf("- %s\n", cs))
				}
			}
			buf.WriteString("\n")
		}
		if nonGoals, ok := product["non_goals"].([]interface{}); ok && len(nonGoals) > 0 {
			buf.WriteString("**Non-Goals**:\n")
			for _, ng := range nonGoals {
				if ngs, ok := ng.(string); ok {
					buf.WriteString(fmt.Sprintf("- %s\n", ngs))
				}
			}
			buf.WriteString("\n")
		}
	}

	// Scope
	if scope, ok := s["scope"].(map[string]interface{}); ok {
		buf.WriteString("## Scope\n\n")
		if inScope, ok := scope["in_scope"].([]interface{}); ok {
			buf.WriteString("**In Scope**:\n")
			for _, item := range inScope {
				if is, ok := item.(string); ok {
					buf.WriteString(fmt.Sprintf("- %s\n", is))
				}
			}
			buf.WriteString("\n")
		}
		if outScope, ok := scope["out_of_scope"].([]interface{}); ok && len(outScope) > 0 {
			buf.WriteString("**Out of Scope**:\n")
			for _, item := range outScope {
				if os, ok := item.(string); ok {
					buf.WriteString(fmt.Sprintf("- %s\n", os))
				}
			}
			buf.WriteString("\n")
		}
		if assumptions, ok := scope["assumptions"].([]interface{}); ok && len(assumptions) > 0 {
			buf.WriteString("**Assumptions**:\n")
			for _, a := range assumptions {
				if as, ok := a.(string); ok {
					buf.WriteString(fmt.Sprintf("- %s\n", as))
				}
			}
			buf.WriteString("\n")
		}
	}

	// Personas
	if personas, ok := s["personas"].([]interface{}); ok && len(personas) > 0 {
		buf.WriteString("## User Personas\n\n")
		for _, p := range personas {
			if pm, ok := p.(map[string]interface{}); ok {
				if name, ok := pm["name"].(string); ok {
					buf.WriteString(fmt.Sprintf("### %s\n\n", name))
				}
				if desc, ok := pm["description"].(string); ok {
					buf.WriteString(fmt.Sprintf("%s\n\n", desc))
				}
				if goals, ok := pm["goals"].([]interface{}); ok {
					buf.WriteString("**Goals**:\n")
					for _, g := range goals {
						if gs, ok := g.(string); ok {
							buf.WriteString(fmt.Sprintf("- %s\n", gs))
						}
					}
					buf.WriteString("\n")
				}
			}
		}
	}

	// Requirements
	if reqs, ok := s["requirements"].(map[string]interface{}); ok {
		buf.WriteString("## Requirements\n\n")
		if functional, ok := reqs["functional"].([]interface{}); ok {
			buf.WriteString("### Functional Requirements\n\n")
			for i, req := range functional {
				writeRequirement(&buf, req, i+1)
			}
		}
		if nonFunctional, ok := reqs["non_functional"].([]interface{}); ok && len(nonFunctional) > 0 {
			buf.WriteString("### Non-Functional Requirements\n\n")
			for i, req := range nonFunctional {
				writeRequirement(&buf, req, i+1)
			}
		}
	}

	// Workflows
	if workflows, ok := s["workflows"].([]interface{}); ok && len(workflows) > 0 {
		buf.WriteString("## Workflows\n\n")
		for _, wf := range workflows {
			if wfm, ok := wf.(map[string]interface{}); ok {
				if name, ok := wfm["name"].(string); ok {
					buf.WriteString(fmt.Sprintf("### %s\n\n", name))
				}
				if desc, ok := wfm["description"].(string); ok {
					buf.WriteString(fmt.Sprintf("%s\n\n", desc))
				}
				if steps, ok := wfm["steps"].([]interface{}); ok {
					buf.WriteString("**Steps**:\n")
					for i, step := range steps {
						if ss, ok := step.(string); ok {
							buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, ss))
						} else if sm, ok := step.(map[string]interface{}); ok {
							if action, ok := sm["action"].(string); ok {
								buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, action))
							}
						}
					}
					buf.WriteString("\n")
				}
			}
		}
	}

	// Data Model
	if dataModel, ok := s["data_model"].(map[string]interface{}); ok {
		buf.WriteString("## Data Model\n\n")
		if entities, ok := dataModel["entities"].([]interface{}); ok {
			for _, entity := range entities {
				if em, ok := entity.(map[string]interface{}); ok {
					if name, ok := em["name"].(string); ok {
						buf.WriteString(fmt.Sprintf("### Entity: %s\n\n", name))
					}
					if desc, ok := em["description"].(string); ok {
						buf.WriteString(fmt.Sprintf("%s\n\n", desc))
					}
					if fields, ok := em["fields"].([]interface{}); ok {
						buf.WriteString("**Fields**:\n")
						buf.WriteString("| Name | Type | Required | Description |\n")
						buf.WriteString("|------|------|----------|-------------|\n")
						for _, f := range fields {
							if fm, ok := f.(map[string]interface{}); ok {
								name := fm["name"]
								ftype := fm["type"]
								required := fm["required"]
								desc := fm["description"]
								buf.WriteString(fmt.Sprintf("| %v | %v | %v | %v |\n", name, ftype, required, desc))
							}
						}
						buf.WriteString("\n")
					}
				}
			}
		}
	}

	// API
	if api, ok := s["api"].(map[string]interface{}); ok {
		buf.WriteString("## API Specification\n\n")
		if style, ok := api["style"].(string); ok {
			buf.WriteString(fmt.Sprintf("**Style**: %s\n\n", style))
		}
		if auth, ok := api["authentication"].(map[string]interface{}); ok {
			buf.WriteString("**Authentication**:\n")
			if method, ok := auth["method"].(string); ok {
				buf.WriteString(fmt.Sprintf("- Method: %s\n", method))
			}
			buf.WriteString("\n")
		}
		if endpoints, ok := api["endpoints"].([]interface{}); ok {
			buf.WriteString("### Endpoints\n\n")
			for _, ep := range endpoints {
				if epm, ok := ep.(map[string]interface{}); ok {
					method := epm["method"]
					path := epm["path"]
					desc := epm["description"]
					buf.WriteString(fmt.Sprintf("**%v %v**\n", method, path))
					if desc != nil {
						buf.WriteString(fmt.Sprintf("%v\n\n", desc))
					}
				}
			}
		}
	}

	// UI
	if ui, ok := s["ui"].(map[string]interface{}); ok {
		buf.WriteString("## User Interface\n\n")
		if screens, ok := ui["screens"].([]interface{}); ok {
			for _, screen := range screens {
				if sm, ok := screen.(map[string]interface{}); ok {
					if name, ok := sm["name"].(string); ok {
						buf.WriteString(fmt.Sprintf("### Screen: %s\n\n", name))
					}
					if desc, ok := sm["description"].(string); ok {
						buf.WriteString(fmt.Sprintf("%s\n\n", desc))
					}
					if components, ok := sm["components"].([]interface{}); ok {
						buf.WriteString("**Components**:\n")
						for _, c := range components {
							if cs, ok := c.(string); ok {
								buf.WriteString(fmt.Sprintf("- %s\n", cs))
							}
						}
						buf.WriteString("\n")
					}
				}
			}
		}
	}

	// Non-Functionals
	if nf, ok := s["non_functionals"].(map[string]interface{}); ok {
		buf.WriteString("## Non-Functional Requirements\n\n")
		// Sort keys for consistent output
		keys := make([]string, 0, len(nf))
		for k := range nf {
			keys = append(keys, k)
		}
		for _, key := range keys {
			val := nf[key]
			title := strings.ReplaceAll(key, "_", " ")
			title = titleCaser.String(title)
			buf.WriteString(fmt.Sprintf("### %s\n\n", title))
			if arr, ok := val.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						buf.WriteString(fmt.Sprintf("- %s\n", s))
					}
				}
			} else if s, ok := val.(string); ok {
				buf.WriteString(fmt.Sprintf("%s\n", s))
			}
			buf.WriteString("\n")
		}
	}

	// Acceptance Criteria
	if acceptance, ok := s["acceptance"].(map[string]interface{}); ok {
		buf.WriteString("## Acceptance Criteria\n\n")
		if criteria, ok := acceptance["criteria"].([]interface{}); ok {
			for i, c := range criteria {
				if cs, ok := c.(string); ok {
					buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, cs))
				}
			}
			buf.WriteString("\n")
		}
		if testCases, ok := acceptance["test_cases"].([]interface{}); ok && len(testCases) > 0 {
			buf.WriteString("### Test Cases\n\n")
			for _, tc := range testCases {
				if tcm, ok := tc.(map[string]interface{}); ok {
					if name, ok := tcm["name"].(string); ok {
						buf.WriteString(fmt.Sprintf("- **%s**", name))
					}
					if desc, ok := tcm["description"].(string); ok {
						buf.WriteString(fmt.Sprintf(": %s", desc))
					}
					buf.WriteString("\n")
				}
			}
			buf.WriteString("\n")
		}
	}

	return buf.Bytes()
}

// renderRalphAgentMD generates the @AGENT.md file for Ralph format.
func renderRalphAgentMD(project *domain.Project, spec json.RawMessage) []byte {
	var s map[string]interface{}
	_ = json.Unmarshal(spec, &s)

	var buf bytes.Buffer
	buf.WriteString("# Agent Build Instructions\n\n")

	// Try to detect tech stack from spec
	techStack := detectTechStack(s)

	buf.WriteString("## Project Setup\n")
	buf.WriteString("```bash\n")
	switch techStack {
	case "node", "typescript", "javascript":
		buf.WriteString("# Install dependencies\n")
		buf.WriteString("npm install\n")
	case "go", "golang":
		buf.WriteString("# Install dependencies\n")
		buf.WriteString("go mod tidy\n")
	case "python":
		buf.WriteString("# Create virtual environment and install dependencies\n")
		buf.WriteString("python -m venv venv\n")
		buf.WriteString("source venv/bin/activate\n")
		buf.WriteString("pip install -r requirements.txt\n")
	case "rust":
		buf.WriteString("# Build the project\n")
		buf.WriteString("cargo build\n")
	default:
		buf.WriteString("# Install dependencies (example for Node.js project)\n")
		buf.WriteString("npm install\n\n")
		buf.WriteString("# Or for Python project\n")
		buf.WriteString("pip install -r requirements.txt\n\n")
		buf.WriteString("# Or for Go project\n")
		buf.WriteString("go mod tidy\n")
	}
	buf.WriteString("```\n\n")

	buf.WriteString("## Running Tests\n")
	buf.WriteString("```bash\n")
	switch techStack {
	case "node", "typescript", "javascript":
		buf.WriteString("npm test\n")
	case "go", "golang":
		buf.WriteString("go test ./...\n")
	case "python":
		buf.WriteString("pytest\n")
	case "rust":
		buf.WriteString("cargo test\n")
	default:
		buf.WriteString("# Node.js\n")
		buf.WriteString("npm test\n\n")
		buf.WriteString("# Python\n")
		buf.WriteString("pytest\n\n")
		buf.WriteString("# Go\n")
		buf.WriteString("go test ./...\n")
	}
	buf.WriteString("```\n\n")

	buf.WriteString("## Build Commands\n")
	buf.WriteString("```bash\n")
	switch techStack {
	case "node", "typescript", "javascript":
		buf.WriteString("npm run build\n")
	case "go", "golang":
		buf.WriteString("go build ./...\n")
	case "python":
		buf.WriteString("python setup.py build\n")
	case "rust":
		buf.WriteString("cargo build --release\n")
	default:
		buf.WriteString("# Production build\n")
		buf.WriteString("npm run build\n")
		buf.WriteString("# or\n")
		buf.WriteString("go build ./...\n")
	}
	buf.WriteString("```\n\n")

	buf.WriteString("## Development Server\n")
	buf.WriteString("```bash\n")
	switch techStack {
	case "node", "typescript", "javascript":
		buf.WriteString("npm run dev\n")
	case "go", "golang":
		buf.WriteString("go run ./cmd/server\n")
	case "python":
		buf.WriteString("python main.py\n")
	case "rust":
		buf.WriteString("cargo run\n")
	default:
		buf.WriteString("# Start development server\n")
		buf.WriteString("npm run dev\n")
		buf.WriteString("# or\n")
		buf.WriteString("go run ./cmd/server\n")
	}
	buf.WriteString("```\n\n")

	buf.WriteString("## Key Learnings\n")
	buf.WriteString("- Update this section when you learn new build optimizations\n")
	buf.WriteString("- Document any gotchas or special setup requirements\n")
	buf.WriteString("- Keep track of the fastest test/build cycle\n\n")

	// Feature Development Quality Standards
	buf.WriteString("## Feature Development Quality Standards\n\n")
	buf.WriteString("**CRITICAL**: All new features MUST meet the following mandatory requirements before being considered complete.\n\n")

	buf.WriteString("### Testing Requirements\n\n")
	buf.WriteString("- **Minimum Coverage**: 85% code coverage ratio required for all new code\n")
	buf.WriteString("- **Test Pass Rate**: 100% - all tests must pass, no exceptions\n")
	buf.WriteString("- **Test Types Required**:\n")
	buf.WriteString("  - Unit tests for all business logic and services\n")
	buf.WriteString("  - Integration tests for API endpoints or main functionality\n")
	buf.WriteString("  - End-to-end tests for critical user workflows\n")
	buf.WriteString("- **Test Quality**: Tests must validate behavior, not just achieve coverage metrics\n")
	buf.WriteString("- **Test Documentation**: Complex test scenarios must include comments explaining the test strategy\n\n")

	buf.WriteString("### Git Workflow Requirements\n\n")
	buf.WriteString("Before moving to the next feature, ALL changes must be:\n\n")
	buf.WriteString("1. **Committed with Clear Messages**:\n")
	buf.WriteString("   ```bash\n")
	buf.WriteString("   git add .\n")
	buf.WriteString("   git commit -m \"feat(module): descriptive message following conventional commits\"\n")
	buf.WriteString("   ```\n")
	buf.WriteString("   - Use conventional commit format: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, etc.\n")
	buf.WriteString("   - Include scope when applicable: `feat(api):`, `fix(ui):`, `test(auth):`\n")
	buf.WriteString("   - Write descriptive messages that explain WHAT changed and WHY\n\n")
	buf.WriteString("2. **Pushed to Remote Repository**:\n")
	buf.WriteString("   ```bash\n")
	buf.WriteString("   git push origin <branch-name>\n")
	buf.WriteString("   ```\n")
	buf.WriteString("   - Never leave completed features uncommitted\n")
	buf.WriteString("   - Push regularly to maintain backup and enable collaboration\n")
	buf.WriteString("   - Ensure CI/CD pipelines pass before considering feature complete\n\n")
	buf.WriteString("3. **Branch Hygiene**:\n")
	buf.WriteString("   - Work on feature branches, never directly on `main`\n")
	buf.WriteString("   - Branch naming convention: `feature/<feature-name>`, `fix/<issue-name>`, `docs/<doc-update>`\n")
	buf.WriteString("   - Create pull requests for all significant changes\n\n")
	buf.WriteString("4. **Ralph Integration**:\n")
	buf.WriteString("   - Update @fix_plan.md with new tasks before starting work\n")
	buf.WriteString("   - Mark items complete in @fix_plan.md upon completion\n")
	buf.WriteString("   - Update PROMPT.md if development patterns change\n")
	buf.WriteString("   - Test features work within Ralph's autonomous loop\n\n")

	buf.WriteString("### Feature Completion Checklist\n\n")
	buf.WriteString("Before marking ANY feature as complete, verify:\n\n")
	buf.WriteString("- [ ] All tests pass with appropriate framework command\n")
	buf.WriteString("- [ ] Code coverage meets 85% minimum threshold\n")
	buf.WriteString("- [ ] Coverage report reviewed for meaningful test quality\n")
	buf.WriteString("- [ ] Code formatted according to project standards\n")
	buf.WriteString("- [ ] Type checking passes (if applicable)\n")
	buf.WriteString("- [ ] All changes committed with conventional commit messages\n")
	buf.WriteString("- [ ] All commits pushed to remote repository\n")
	buf.WriteString("- [ ] @fix_plan.md task marked as complete\n")
	buf.WriteString("- [ ] Implementation documentation updated\n")
	buf.WriteString("- [ ] Inline code comments updated or added\n")
	buf.WriteString("- [ ] @AGENT.md updated (if new patterns introduced)\n")
	buf.WriteString("- [ ] Breaking changes documented\n")
	buf.WriteString("- [ ] Features tested within Ralph loop (if applicable)\n")
	buf.WriteString("- [ ] CI/CD pipeline passes\n\n")

	buf.WriteString("---\n\n")
	buf.WriteString(fmt.Sprintf("*Generated for: %s*\n", project.Name))

	return buf.Bytes()
}

// Helper functions for Ralph format

func extractObjectives(spec map[string]interface{}) []string {
	objectives := []string{
		"Study specs/* to learn about the project specifications",
		"Review @fix_plan.md for current priorities",
	}

	// Extract from product purpose
	if product, ok := spec["product"].(map[string]interface{}); ok {
		if purpose, ok := product["purpose"].(string); ok && purpose != "" {
			objectives = append(objectives, fmt.Sprintf("Implement: %s", truncateString(purpose, 80)))
		}
	}

	// Extract key requirements
	if reqs, ok := spec["requirements"].(map[string]interface{}); ok {
		if functional, ok := reqs["functional"].([]interface{}); ok {
			for i, req := range functional {
				if i >= 2 { // Only add first 2 requirements
					break
				}
				var reqStr string
				if rs, ok := req.(string); ok {
					reqStr = rs
				} else if rm, ok := req.(map[string]interface{}); ok {
					if desc, ok := rm["description"].(string); ok {
						reqStr = desc
					}
				}
				if reqStr != "" {
					objectives = append(objectives, fmt.Sprintf("Implement: %s", truncateString(reqStr, 60)))
				}
			}
		}
	}

	// Always add these standard objectives
	objectives = append(objectives,
		"Run tests after each implementation",
		"Update documentation and fix_plan.md",
	)

	return objectives
}

func extractConstraints(spec map[string]interface{}) []string {
	constraints := []string{}

	// From scope assumptions
	if scope, ok := spec["scope"].(map[string]interface{}); ok {
		if assumptions, ok := scope["assumptions"].([]interface{}); ok {
			for _, a := range assumptions {
				if as, ok := a.(string); ok {
					constraints = append(constraints, as)
				}
			}
		}
	}

	// From non-functionals
	if nf, ok := spec["non_functionals"].(map[string]interface{}); ok {
		for category, val := range nf {
			if arr, ok := val.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						constraints = append(constraints, fmt.Sprintf("[%s] %s", category, truncateString(s, 60)))
						if len(constraints) >= 6 {
							return constraints
						}
					}
				}
			}
		}
	}

	return constraints
}

func extractSuccessCriteria(spec map[string]interface{}) []string {
	criteria := []string{}

	// From product success_criteria
	if product, ok := spec["product"].(map[string]interface{}); ok {
		if sc, ok := product["success_criteria"].([]interface{}); ok {
			for _, c := range sc {
				if cs, ok := c.(string); ok {
					criteria = append(criteria, cs)
				}
			}
		}
	}

	// From acceptance criteria
	if acceptance, ok := spec["acceptance"].(map[string]interface{}); ok {
		if ac, ok := acceptance["criteria"].([]interface{}); ok {
			for _, c := range ac {
				if cs, ok := c.(string); ok {
					criteria = append(criteria, cs)
					if len(criteria) >= 6 {
						return criteria
					}
				}
			}
		}
	}

	return criteria
}

func detectTechStack(spec map[string]interface{}) string {
	// Check API style
	if api, ok := spec["api"].(map[string]interface{}); ok {
		if style, ok := api["style"].(string); ok {
			style = strings.ToLower(style)
			if strings.Contains(style, "graphql") {
				return "node" // GraphQL commonly with Node
			}
		}
	}

	// Check scope/assumptions for tech mentions
	if scope, ok := spec["scope"].(map[string]interface{}); ok {
		if assumptions, ok := scope["assumptions"].([]interface{}); ok {
			for _, a := range assumptions {
				if as, ok := a.(string); ok {
					as = strings.ToLower(as)
					if strings.Contains(as, "node") || strings.Contains(as, "typescript") || strings.Contains(as, "react") {
						return "node"
					}
					// Check Python-specific frameworks before Go to avoid false matches
					// (e.g., "django " contains "go " as substring)
					if strings.Contains(as, "python") || strings.Contains(as, "django") || strings.Contains(as, "flask") {
						return "python"
					}
					if strings.Contains(as, "golang") || strings.Contains(as, "go ") {
						return "go"
					}
					if strings.Contains(as, "rust") {
						return "rust"
					}
				}
			}
		}
	}

	// Check non-functionals for hints
	if nf, ok := spec["non_functionals"].(map[string]interface{}); ok {
		for _, val := range nf {
			if arr, ok := val.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						s = strings.ToLower(s)
						if strings.Contains(s, "node") || strings.Contains(s, "npm") {
							return "node"
						}
						if strings.Contains(s, "go ") || strings.Contains(s, "golang") {
							return "go"
						}
					}
				}
			}
		}
	}

	return "generic"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func writeRequirement(buf *bytes.Buffer, req interface{}, num int) {
	if rs, ok := req.(string); ok {
		buf.WriteString(fmt.Sprintf("%d. %s\n\n", num, rs))
	} else if rm, ok := req.(map[string]interface{}); ok {
		if name, ok := rm["name"].(string); ok {
			buf.WriteString(fmt.Sprintf("%d. **%s**\n", num, name))
		} else {
			buf.WriteString(fmt.Sprintf("%d. ", num))
		}
		if desc, ok := rm["description"].(string); ok {
			buf.WriteString(fmt.Sprintf("   %s\n", desc))
		}
		if priority, ok := rm["priority"].(string); ok {
			buf.WriteString(fmt.Sprintf("   - Priority: %s\n", priority))
		}
		buf.WriteString("\n")
	}
}
