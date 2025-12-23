# PLAN.md — Spec Builder (v0.2)

This plan is ordered to minimize ambiguity for AI coding agents. Backend establishes invariants first; frontend is a consumer.

## Milestone 0 — Repo & Tooling (Day 0)
- Create mono-repo layout:
  - /backend (Go)
  - /frontend (React + TS)
  - /contracts (JSON schemas, optional OpenAPI later)
  - /exports (generated artifacts ignored by git)
- Establish formatting/lint:
  - Go: gofmt, golangci-lint (optional), unit test harness
  - Frontend: eslint + prettier
- Define “prompt versioning” convention (e.g., /backend/prompts/v1/*.txt)

Deliverable:
- repo boots, CI runs tests for both packages (even if empty)

## Milestone 1 — Canonical Data Model + Persistence (Backend) (MVP-critical)
Goal: Lock invariants (versioning, immutability, snapshots) before UI.

Backend tasks:
1. Implement DB schema (SQLite):
   - projects
   - questions
   - answers
   - snapshots
   - issues
2. Implement Go domain structs + repository layer
3. Enforce invariants:
   - answer immutability
   - supersedes chain
   - “latest answer per question” query
   - snapshots append-only
4. Implement minimal HTTP API:
   - POST /projects
   - GET /projects/{id}
   - GET /projects/{id}/questions
   - POST /projects/{id}/answers  (store only; compilation stub ok for now)
   - GET /projects/{id}/snapshots/{snapshot_id} (stub ok)

Deliverable:
- Integration tests proving versioning + snapshot immutability semantics

## Milestone 2 — Spec Schema + Validation (Backend) (MVP-critical)
Goal: SPEC.json validation is not optional.

Tasks:
1. Define SpecSchema (JSON Schema file) for ProjectImplementationSpec
2. Implement validator:
   - Validate compiled spec against JSON Schema
   - Emit Issue(type=missing/conflict/assumption, severity)
3. Store issues by snapshot_id

Deliverable:
- Unit tests: invalid spec rejected + issues emitted as expected

## Milestone 3 — LLM Orchestration (Backend) (MVP-critical)
Goal: Build the compiler pipeline with strict structured outputs.

Tasks:
1. Implement prompt/version system:
   - planner prompt v1
   - asker prompt v1
   - compiler prompt v1 (must output SPEC JSON + TRACE JSON)
   - validator prompt v1 (optional if rules-based validator already sufficient)
2. Implement LLM client abstraction:
   - Support at least one provider first (OpenAI or Anthropic)
   - temperature=0, prompt_version recorded, model recorded
3. Implement /projects/{id}/next-questions:
   - Use planner+asker
   - Persist generated questions with tags/spec_paths
4. Implement compilation endpoint:
   - POST /projects/{id}/compile
   - Build “latest answers” bundle
   - Call compiler -> parse JSON -> validate -> store snapshot + issues
5. Wire answer submission to compilation (sync for MVP):
   - POST /projects/{id}/answers triggers compile and returns snapshot_id + issues

Deliverable:
- End-to-end: answer -> compile -> validated snapshot + issues saved

## Milestone 4 — Frontend MVP UI (React) (MVP-critical)
Goal: Visual spec builder UX: ledger + spec view + issues.

Tasks:
1. App shell with 3-panel layout:
   - Left: Question Ledger
   - Center: Spec Workspace (Readable + Formal tabs)
   - Right: Spec Map tree (static nodes initially)
2. Project screen:
   - Load project
   - Fetch questions, latest snapshot
3. Question answering:
   - Render question cards
   - Answer editor supports type-based UI
   - Submit answer -> refresh snapshot + issues
4. Answer edit/version history:
   - Show latest + prior versions
   - Editing creates new version (backend enforced)
5. Spec views:
   - Formal: render JSON
   - Readable: render Markdown derived by backend (or simple client render from JSON for MVP)
6. Issues panel:
   - Severity filter
   - Click issue to jump to spec section (best-effort)

Deliverable:
- A user can complete a project’s Q/A loop and see compiled spec and issues update live

## Milestone 5 — Snapshot Diff + “Impact” (Strongly recommended)
Goal: Make edits safe by showing what changed.

Backend:
- Implement /diff endpoint returning JSON diff between two snapshots (structured, not prose)

Frontend:
- Snapshot selector dropdown
- Diff tab rendering (tree-based or line-based JSON diff)

Deliverable:
- User can compare latest snapshot to prior and understand changes after edits

## Milestone 6 — Export (AI Coder Pack) (MVP-critical)
Goal: Output the pack your coding agent consumes.

Backend:
1. Implement export generator:
   - Input: snapshot_id
   - Output zip containing:
     - SPEC.json (snapshot.spec)
     - TRACE.json (snapshot trace)
     - SPEC.md (derived)
     - PLAN.md (generated from spec.plan or derived)
     - DECISIONS.md (static template + filled)
     - ACCEPTANCE.md (derived)
2. Implement /projects/{id}/export returning download URL

Frontend:
- Export button on snapshot view
- Download flow

Deliverable:
- Zip is self-contained and agent-ready; rebuildable from snapshot id

## Milestone 7 — Hardening (Post-MVP)
- Auth (optional)
- Postgres support
- Async compilation (queue) for long projects
- Spec map upgraded from tree to dependency graph (Cytoscape)
- Semantic consistency rules:
  - e.g., auth model conflicts, API vs workflow mismatch, data fields referenced but undefined

## Notes for AI Coding Agents
- Do not invent missing constraints: emit Issue(type=missing) instead.
- Treat SPEC.json as canonical.
- Never store canonical state in frontend.
- Reject invalid LLM outputs; do not “patch” them in code silently.
