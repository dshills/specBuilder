# Ralph Format: Best Practices for AI Agent Instruction Files

This document details the file formats and best practices used in the Ralph for Claude Code project for creating effective AI agent instruction files. Ralph is an autonomous AI development loop system that uses structured markdown files to guide Claude Code through iterative development cycles.

## Table of Contents

1. [Overview](#overview)
2. [PROMPT.md - Agent Instructions](#promptmd---agent-instructions)
3. [@fix_plan.md - Task Prioritization](#fix_planmd---task-prioritization)
4. [specs/requirements.md - Technical Specifications](#specsrequirementsmd---technical-specifications)
5. [@AGENT.md - Build Instructions & Quality Gates](#agentmd---build-instructions--quality-gates)
6. [File Naming Conventions](#file-naming-conventions)
7. [Status Reporting Protocol](#status-reporting-protocol)
8. [Exit Detection Patterns](#exit-detection-patterns)
9. [Specification Workshop Format](#specification-workshop-format)

---

## Overview

Ralph uses four primary file types to guide autonomous AI development:

| File | Purpose | Prefix | Update Frequency |
|------|---------|--------|------------------|
| `PROMPT.md` | Agent instructions and context | None | Rarely (project setup) |
| `@fix_plan.md` | Prioritized task list | `@` | Every loop iteration |
| `specs/requirements.md` | Technical specifications | None | When requirements change |
| `@AGENT.md` | Build/run instructions | `@` | When patterns change |

The `@` prefix convention indicates Ralph-specific control files that the agent actively reads and updates.

---

## PROMPT.md - Agent Instructions

The PROMPT.md file is the primary instruction file that drives each iteration of the autonomous development loop. It establishes context, objectives, and behavioral guidelines.

### Structure

```markdown
# Ralph Development Instructions

## Context
[Identity and project description]

## Current Objectives
[Numbered list of 4-6 prioritized goals]

## Key Principles
[Behavioral guidelines - typically 5-8 bullet points]

## Testing Guidelines (CRITICAL)
[Testing constraints and priorities]

## Execution Guidelines
[Step-by-step workflow instructions]

## Status Reporting (CRITICAL)
[Machine-readable status block format]

## Exit Scenarios
[Specification by Example for completion detection]

## File Structure
[Project organization reference]

## Current Task
[Dynamic task selection instruction]
```

### Best Practices

#### 1. Context Section
- Define the agent's identity ("You are Ralph, an autonomous AI development agent...")
- Specify the project type clearly
- Keep it brief (2-3 sentences)

```markdown
## Context
You are Ralph, an autonomous AI development agent working on a [PROJECT_TYPE] project.
```

#### 2. Current Objectives
- Limit to 4-6 numbered objectives
- Order by priority (most important first)
- Make objectives actionable and specific
- Reference key files (`@fix_plan.md`, `specs/*`)

```markdown
## Current Objectives
1. Study specs/* to learn about the project specifications
2. Review @fix_plan.md for current priorities
3. Implement the highest priority item using best practices
4. Use parallel subagents for complex tasks (max 100 concurrent)
5. Run tests after each implementation
6. Update documentation and fix_plan.md
```

#### 3. Key Principles
- Use imperative statements
- Focus on ONE principle per bullet
- Include anti-patterns to avoid
- Reference control files with `@` prefix

```markdown
## Key Principles
- ONE task per loop - focus on the most important thing
- Search the codebase before assuming something isn't implemented
- Use subagents for expensive operations (file searching, analysis)
- Write comprehensive tests with clear documentation
- Update @fix_plan.md with your learnings
- Commit working changes with descriptive messages
```

#### 4. Testing Guidelines
- Mark as CRITICAL when important
- Set explicit effort limits (e.g., "~20% of total effort")
- Define priority order clearly
- Use negative constraints ("Do NOT...")

```markdown
## Testing Guidelines (CRITICAL)
- LIMIT testing to ~20% of your total effort per loop
- PRIORITIZE: Implementation > Documentation > Tests
- Only write tests for NEW functionality you implement
- Do NOT refactor existing tests unless broken
- Do NOT add "additional test coverage" as busy work
- Focus on CORE functionality first, comprehensive testing later
```

#### 5. Execution Guidelines
- Sequence matters - order steps chronologically
- Include both pre-conditions and post-conditions
- Reference specific files to update

```markdown
## Execution Guidelines
- Before making changes: search codebase using subagents
- After implementation: run ESSENTIAL tests for the modified code only
- If tests fail: fix them as part of your current work
- Keep @AGENT.md updated with build/run instructions
- Document the WHY behind tests and implementations
- No placeholder implementations - build it properly
```

#### 6. What NOT to Include
- Generic platitudes that don't affect behavior
- Overly verbose explanations
- Conflicting instructions
- Time-based deadlines (agents don't track time)

---

## @fix_plan.md - Task Prioritization

The @fix_plan.md file serves as the agent's task queue. It uses a simple markdown checklist format with priority tiers.

### Structure

```markdown
# Ralph Fix Plan

## High Priority
- [ ] [Critical task 1]
- [ ] [Critical task 2]

## Medium Priority
- [ ] [Secondary task 1]
- [ ] [Secondary task 2]

## Low Priority
- [ ] [Nice-to-have 1]
- [ ] [Nice-to-have 2]

## Completed
- [x] [Completed task 1]
- [x] [Completed task 2]

## Notes
[Context, blockers, learnings]
```

### Best Practices

#### 1. Task Formatting
- Use checkbox syntax: `- [ ]` (pending) and `- [x]` (complete)
- Keep tasks atomic and actionable
- Avoid vague tasks like "improve performance"
- Include enough context to be self-contained

**Good:**
```markdown
- [ ] Set up basic project structure and build system
- [ ] Define core data structures and types
- [ ] Implement basic input/output handling
```

**Bad:**
```markdown
- [ ] Make it work
- [ ] Fix bugs
- [ ] Improve things
```

#### 2. Priority Tier Guidelines

| Tier | Criteria | Examples |
|------|----------|----------|
| High | Blocking other work, core functionality, critical bugs | "Set up build system", "Define data models" |
| Medium | Important but not blocking, enhancements | "Add error handling", "Implement business logic" |
| Low | Nice-to-have, optimization, polish | "Performance tuning", "Extended features" |

#### 3. Completed Section
- Move completed tasks here rather than deleting
- Preserves history for context
- Helps measure progress

#### 4. Notes Section
- Document blockers and dependencies
- Record learnings for future reference
- Track external dependencies or decisions needed

```markdown
## Notes
- Focus on MVP functionality first
- Ensure each feature is properly tested
- Update this file after each major milestone
- Blocked on API design decision (see specs/api.md)
```

#### 5. Update Discipline
- Agent should update after completing each task
- Move items between tiers as priorities shift
- Add new discovered tasks during implementation

---

## specs/requirements.md - Technical Specifications

The specs/requirements.md file contains detailed technical requirements derived from the product requirements document (PRD).

### Structure

```markdown
# Technical Specifications

## System Architecture
[High-level architecture description]

## Data Models
[Core data structures and relationships]

## API Specifications
[Endpoints, contracts, authentication]

## User Interface Requirements
[UI components, interactions, states]

## Performance Requirements
[Latency, throughput, scalability targets]

## Security Considerations
[Authentication, authorization, data protection]

## Integration Requirements
[External systems, APIs, protocols]
```

### Best Practices

#### 1. Completeness Over Brevity
- Include all technical details from the PRD
- Be explicit about constraints and assumptions
- Document non-functional requirements

#### 2. Structured Sections
- Use consistent heading levels
- Group related requirements together
- Include success criteria for each section

#### 3. Concrete Examples
- Provide sample data structures
- Show example API requests/responses
- Include UI mockup descriptions or references

#### 4. Traceability
- Reference original PRD sections when applicable
- Note which requirements are MVP vs. future

---

## @AGENT.md - Build Instructions & Quality Gates

The @AGENT.md file contains build/run instructions and quality standards. It serves as the operational reference for the agent.

### Structure

```markdown
# Agent Build Instructions

## Project Setup
[Installation and setup commands]

## Running Tests
[Test execution commands by platform]

## Build Commands
[Build and compilation instructions]

## Development Server
[Local development instructions]

## Key Learnings
[Accumulated project-specific knowledge]

## Feature Development Quality Standards
[Mandatory requirements checklist]

## Feature Completion Checklist
[Pre-merge verification steps]
```

### Best Practices

#### 1. Command Blocks
- Provide commands for multiple platforms/languages
- Use code blocks with appropriate syntax highlighting
- Include both setup and ongoing commands

```markdown
## Running Tests
```bash
# Node.js
npm test

# Python
pytest

# Rust
cargo test
```

#### 2. Quality Standards Section
- Define explicit thresholds (e.g., "85% code coverage")
- Specify mandatory test types
- Include git workflow requirements

```markdown
## Testing Requirements
- **Minimum Coverage**: 85% code coverage ratio required for all new code
- **Test Pass Rate**: 100% - all tests must pass, no exceptions
- **Test Types Required**:
  - Unit tests for all business logic and services
  - Integration tests for API endpoints or main functionality
  - End-to-end tests for critical user workflows
```

#### 3. Git Workflow Requirements
- Specify commit message format (conventional commits recommended)
- Define branch naming conventions
- Document push/PR requirements

```markdown
## Git Workflow Requirements
1. **Committed with Clear Messages**:
   - Use conventional commit format: `feat:`, `fix:`, `docs:`, `test:`, etc.
   - Include scope when applicable: `feat(api):`, `fix(ui):`, `test(auth):`
   - Write descriptive messages that explain WHAT changed and WHY

2. **Branch Hygiene**:
   - Work on feature branches, never directly on `main`
   - Branch naming: `feature/<name>`, `fix/<issue>`, `docs/<update>`
```

#### 4. Feature Completion Checklist
Use checkbox format for clear verification:

```markdown
## Feature Completion Checklist
- [ ] All tests pass with appropriate framework command
- [ ] Code coverage meets 85% minimum threshold
- [ ] Code formatted according to project standards
- [ ] All changes committed with conventional commit messages
- [ ] All commits pushed to remote repository
- [ ] @fix_plan.md task marked as complete
- [ ] Implementation documentation updated
- [ ] CI/CD pipeline passes
```

---

## File Naming Conventions

### Control Files (@ Prefix)
Files prefixed with `@` are Ralph-specific control files that the agent actively reads and updates:

| File | Purpose |
|------|---------|
| `@fix_plan.md` | Task queue |
| `@AGENT.md` | Build instructions |

### State Files (Hidden)
Hidden files track loop state and should not be manually edited:

| File | Purpose |
|------|---------|
| `.call_count` | API call tracking |
| `.exit_signals` | Exit detection state |
| `.circuit_breaker_state` | Circuit breaker status |
| `.ralph_session` | Session continuity |

### Directory Structure

```
project/
├── PROMPT.md              # Main agent instructions
├── @fix_plan.md           # Task queue (Ralph control file)
├── @AGENT.md              # Build instructions (Ralph control file)
├── specs/                 # Specifications directory
│   ├── requirements.md    # Technical specifications
│   └── stdlib/           # Standard library specs
├── src/                   # Source code
├── examples/              # Usage examples
├── logs/                  # Execution logs
└── docs/generated/        # Auto-generated documentation
```

---

## Status Reporting Protocol

The PROMPT.md file should define a machine-readable status block format for exit detection. Ralph uses the `RALPH_STATUS` block:

### Format

```markdown
## Status Reporting (CRITICAL)

At the end of your response, ALWAYS include this status block:

```
---RALPH_STATUS---
STATUS: IN_PROGRESS | COMPLETE | BLOCKED
TASKS_COMPLETED_THIS_LOOP: <number>
FILES_MODIFIED: <number>
TESTS_STATUS: PASSING | FAILING | NOT_RUN
WORK_TYPE: IMPLEMENTATION | TESTING | DOCUMENTATION | REFACTORING
EXIT_SIGNAL: false | true
RECOMMENDATION: <one line summary of what to do next>
---END_RALPH_STATUS---
```
```

### Field Definitions

| Field | Values | Purpose |
|-------|--------|---------|
| `STATUS` | `IN_PROGRESS`, `COMPLETE`, `BLOCKED` | Current work state |
| `TASKS_COMPLETED_THIS_LOOP` | Integer | Progress metric |
| `FILES_MODIFIED` | Integer | Change metric |
| `TESTS_STATUS` | `PASSING`, `FAILING`, `NOT_RUN` | Quality gate |
| `WORK_TYPE` | `IMPLEMENTATION`, `TESTING`, `DOCUMENTATION`, `REFACTORING` | Work categorization |
| `EXIT_SIGNAL` | `true`, `false` | Explicit completion signal |
| `RECOMMENDATION` | String | Next action guidance |

### EXIT_SIGNAL Logic

The EXIT_SIGNAL is the **explicit completion signal** from the agent. It enables a dual-condition exit gate:

**Exit requires BOTH conditions:**
1. `completion_indicators >= 2` (heuristic detection from natural language)
2. Claude's explicit `EXIT_SIGNAL: true` in the RALPH_STATUS block

This prevents premature exits when the agent says things like "Phase complete, moving to next feature" which could trigger false positives.

| completion_indicators | EXIT_SIGNAL | Result |
|----------------------|-------------|--------|
| >= 2 | `true` | **Exit** |
| >= 2 | `false` | **Continue** |
| < 2 | `true` | **Continue** |

---

## Exit Detection Patterns

### When to Set EXIT_SIGNAL: true

Include explicit conditions in PROMPT.md:

```markdown
### When to set EXIT_SIGNAL: true

Set EXIT_SIGNAL to **true** when ALL of these conditions are met:
1. All items in @fix_plan.md are marked [x]
2. All tests are passing (or no tests exist for valid reasons)
3. No errors or warnings in the last execution
4. All requirements from specs/ are implemented
5. You have nothing meaningful left to implement
```

### What NOT to Do

Include anti-patterns:

```markdown
### What NOT to do:
- Do NOT continue with busy work when EXIT_SIGNAL should be true
- Do NOT run tests repeatedly without implementing new features
- Do NOT refactor code that is already working fine
- Do NOT add features not in the specifications
- Do NOT forget to include the status block
```

### Specification by Example

Include concrete scenarios in Given/When/Then format:

```markdown
## Exit Scenarios (Specification by Example)

### Scenario 1: Successful Project Completion
**Given**:
- All items in @fix_plan.md are marked [x]
- Last test run shows all tests passing
- No errors in recent logs/

**When**: You evaluate project status at end of loop

**Then**: You must output:
```
---RALPH_STATUS---
STATUS: COMPLETE
EXIT_SIGNAL: true
RECOMMENDATION: All requirements met, project ready for review
---END_RALPH_STATUS---
```

### Scenario 2: Work in Progress
**Given**:
- Tasks remain in @fix_plan.md
- Implementation is underway
- Tests are passing

**When**: You complete a task successfully

**Then**: You must output:
```
---RALPH_STATUS---
STATUS: IN_PROGRESS
EXIT_SIGNAL: false
RECOMMENDATION: Continue with next task from @fix_plan.md
---END_RALPH_STATUS---
```
```

---

## Specification Workshop Format

For complex features, use the Three Amigos specification workshop format (from Janet Gregory's collaborative testing approach).

### Quick Template

```markdown
# Feature: [Name]

**User Story**: As [role], I want [capability] so that [benefit]

**Key Scenarios**:
1. Given [state], When [action], Then [outcome]
2. Given [state], When [action], Then [outcome]

**Edge Cases**:
- [Case 1] → [Behavior]
- [Case 2] → [Behavior]

**Tests**:
- [ ] [Test 1]
- [ ] [Test 2]

**Done When**:
- [ ] Implemented
- [ ] Tested
- [ ] Documented
```

### Full Template Sections

1. **User Story** - As/I want/So that format
2. **Acceptance Criteria** - Measurable success criteria
3. **Questions from Tester** - Edge cases and verification
4. **Implementation Approach** - Technical strategy
5. **Specification by Example** - Given/When/Then scenarios
6. **Edge Cases and Error Conditions** - Unusual situations
7. **Test Strategy** - Unit/Integration/Manual tests
8. **Non-Functional Requirements** - Performance/Security/Usability
9. **Definition of Done** - Completion checklist
10. **Follow-Up Actions** - Next steps with owners

---

## Summary: Key Takeaways

1. **PROMPT.md** establishes identity, objectives, and behavioral guidelines - keep it focused and actionable
2. **@fix_plan.md** is the living task queue - update it every iteration
3. **specs/requirements.md** contains technical depth - be comprehensive
4. **@AGENT.md** defines quality gates - enforce standards
5. **Status blocks** enable machine-readable exit detection - always include them
6. **EXIT_SIGNAL** is the explicit completion signal - prevents premature exits
7. **@ prefix** marks control files the agent actively manages
8. **Specification by Example** provides concrete, testable scenarios

---

## References

- [Ralph for Claude Code](https://github.com/frankbria/ralph-claude-code) - Source project
- [Ralph Technique](https://ghuntley.com/ralph/) - Geoffrey Huntley's original technique
- [Three Amigos](https://www.agilealliance.org/glossary/three-amigos/) - Collaborative testing approach
- [Specification by Example](https://gojko.net/books/specification-by-example/) - Gojko Adzic
