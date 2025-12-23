# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Spec Builder is a requirements compiler that converts product ideas into formal, machine-usable specifications for AI coding agents. It uses LLMs as constrained compilers (not chatbots) with strict structured outputs.

**Key files to read first:**
- `specs/SPEC.json` — Canonical product requirements (source of truth)
- `specs/PLAN.md` — Implementation milestones in order
- `contracts/api/spec-builder.openapi.json` — Backend API contract
- `contracts/spec/ProjectImplementationSpec.schema.json` — Schema for compiled specs

## Architecture

```
Backend (Go 1.22+)              Frontend (React + TypeScript)
├── HTTP API                    ├── Question Ledger
├── SQLite persistence          ├── Spec Workspace (JSON/Markdown/Diff views)
├── LLM orchestration           ├── Issues Panel
├── Schema validation           └── Spec Map (tree)
└── Export generation
```

**Critical constraint:** All LLM calls happen in the backend. Frontend is a pure consumer—no LLM calls, no canonical state.

## Build Commands

```bash
# Backend (Go)
cd backend
go mod tidy
go build ./...
go test ./...
go test -run TestName ./path/to/package  # Single test

# Frontend (React)
cd frontend
npm install
npm run dev      # Development server
npm run build    # Production build
npm run lint     # ESLint + Prettier
npm test         # Run tests
```

## Domain Model Invariants

These are hard requirements—code must enforce them:

1. **Answer immutability**: Editing creates a new Answer with `version=prev+1` and `supersedes=prevAnswerId`. Never mutate existing answers.

2. **Snapshot append-only**: Snapshots are never updated in place. Each compilation creates a new snapshot.

3. **Trace coverage**: Every populated spec field must have provenance in `trace.spec_path_to_sources` or emit a `missing` issue.

4. **Deterministic compilation**: Same inputs (answers + prompt version + model config) must produce identical spec output. Use `temperature=0`.

5. **Schema validation**: Compiled specs must validate against `ProjectImplementationSpec.schema.json`. Reject invalid LLM outputs—no silent fallbacks.

## LLM Orchestration

Four prompt roles in `backend/prompts/v1/`:

| Role | Purpose | Output |
|------|---------|--------|
| planner | Identify spec gaps, suggest question priorities | `PlannerOutput` JSON |
| asker | Generate constrained questions | `AskOutput` JSON |
| compiler | Q&A → ProjectImplementationSpec + Trace | `CompilerOutput` JSON |
| validator | Detect schema/semantic issues | `ValidatorOutput` JSON |

All prompts use `{{TEMPLATE}}` substitution. Outputs must be valid JSON—no prose wrappers.

## API Endpoints (OpenAPI contract)

| Method | Path | Purpose |
|--------|------|---------|
| POST | /projects | Create project |
| GET | /projects/{id} | Get project + latest snapshot ID |
| GET | /projects/{id}/questions | List questions (filter by status/tag) |
| POST | /projects/{id}/next-questions | Generate questions via LLM |
| POST | /projects/{id}/answers | Submit/edit answer (triggers compile) |
| POST | /projects/{id}/compile | Explicit compile |
| GET | /projects/{id}/snapshots | List snapshots |
| GET | /projects/{id}/snapshots/{id} | Get snapshot + issues |
| GET | /projects/{id}/snapshots/{id}/diff/{other} | Structured diff |
| POST | /projects/{id}/export | Generate AI Coder Pack zip |

## Implementation Order

Follow `specs/PLAN.md` milestones strictly:

1. **M1**: Data model + persistence (enforce versioning/immutability)
2. **M2**: Spec schema validation + issue generation
3. **M3**: LLM orchestration pipeline
4. **M4**: React frontend MVP
5. **M5**: Snapshot diff
6. **M6**: Export (AI Coder Pack zip)

Backend invariants must be locked before building UI.

## Key Schemas

- **ProjectImplementationSpec**: 12 required sections (product, scope, personas, requirements, workflows, data_model, api, ui, non_functionals, acceptance, plan, trace)
- **Trace**: Maps spec paths (JSON pointers like `/requirements/functional/0`) to source answers
- **Issue**: Types are `conflict | missing | assumption`, severities are `info | warn | error`

## Testing Focus

Prioritize tests for:
- Answer version chain integrity (supersedes links)
- Snapshot immutability (no updates allowed)
- Schema validation rejects invalid specs
- Trace coverage for all spec sections
- LLM output parsing (reject malformed JSON)
