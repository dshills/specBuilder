# Repo Contract Index

## 1. `specs/SPEC.json`

| Aspect | Details |
|--------|---------|
| **Governs** | Canonical product specification for Spec Builder itself. Defines product purpose, architecture, domain model (5 entities), API endpoints, UI layout, LLM orchestration rules, export format, acceptance criteria, and question seeding strategy. |
| **Constrains** | Backend must be Go 1.22+, frontend must be React+TypeScript. All LLM calls backend-only. Answers are immutable (versioned). Snapshots are append-only. Deterministic compilation required (temperature=0). |
| **Must stay consistent with** | OpenAPI contract (endpoint shapes), ProjectImplementationSpec.schema.json (spec_schema.required_sections list), TRACE.schema.json (trace structure), ACCEPTANCE.md (acceptance criteria), PLAN.md (milestones match spec.acceptance.definition_of_done_mvp). |

---

## 2. `specs/PLAN.md`

| Aspect | Details |
|--------|---------|
| **Governs** | Implementation order across 8 milestones (0–7). Defines what to build when, with explicit deliverables and test requirements per milestone. Backend-first philosophy: invariants locked before UI. |
| **Constrains** | Milestone order is mandatory. MVP = milestones 0–4 + 6. Milestone 5 (diff) is "strongly recommended" but not blocking. No early implementation of future milestones. |
| **Must stay consistent with** | SPEC.json (endpoints, domain model, API behavior), OpenAPI (M1 implements subset of endpoints), ACCEPTANCE.md (deliverables prove acceptance criteria). |

---

## 3. `specs/ACCEPTANCE.md`

| Aspect | Details |
|--------|---------|
| **Governs** | Pass/fail criteria for the MVP. Six core acceptance criteria plus four explicit failure conditions. |
| **Constrains** | System must: create new answer versions on edit, produce new snapshots on recompile, trace all spec fields, validate specs, export self-contained packs. System must NOT: overwrite answers, emit prose from compiler, allow untraceable fields. |
| **Must stay consistent with** | SPEC.json (acceptance.definition_of_done_mvp, acceptance.failure_conditions), domain model invariants. |

---

## 4. `contracts/api/spec-builder.openapi.json`

| Aspect | Details |
|--------|---------|
| **Governs** | HTTP API contract. 11 endpoints across 6 tags (Projects, Questions, Answers, Compilation, Snapshots, Exports). Defines all request/response schemas with JSON Schema validation. |
| **Constrains** | All API implementations must match these schemas exactly. Answer.value is `JSONValue` (any JSON type). Diff uses RFC 6902 JSON Patch format. Error responses use `ErrorResponse` schema. Status codes: 200/201 success, 400/404/409/422 errors. |
| **Must stay consistent with** | SPEC.json (api.endpoints), domain model (entity schemas must match), TRACE.schema.json (trace in snapshot matches), ProjectImplementationSpec.schema.json (snapshot.spec validates against it). |

---

## 5. `contracts/spec/ProjectImplementationSpec.schema.json`

| Aspect | Details |
|--------|---------|
| **Governs** | Structure of compiled specifications (the output of the compiler LLM). JSON Schema Draft 2020-12 with 12 required top-level sections plus 3 optional (integrations, observability, security_privacy). |
| **Constrains** | All compiled specs must validate against this schema. minItems=1 on many arrays (personas, workflows, functional requirements, etc.). Trace section is required and must have spec_path_to_sources. |
| **Must stay consistent with** | SPEC.json (spec_schema.required_sections), TRACE.schema.json (Trace definition must match), compiler.txt prompt (instructs LLM to output conforming spec). |

---

## 6. `contracts/spec/TRACE.schema.json`

| Aspect | Details |
|--------|---------|
| **Governs** | Structure of provenance tracking. Maps JSON-pointer-like paths (e.g., `/requirements/functional/0`) to arrays of TraceSource objects (question_id, answer_id, answer_version). |
| **Constrains** | All paths must start with `/`. Each path must have at least one source (minItems=1). Sources require question_id, answer_id, answer_version. Optional `notes` field allowed. |
| **Must stay consistent with** | ProjectImplementationSpec.schema.json (Trace definition embedded there), compiler.txt prompt (instructs trace output), SPEC.json (exports.ai_coder_pack.trace_rules). |

---

## 7. `backend/prompts/v1/planner.txt`

| Aspect | Details |
|--------|---------|
| **Governs** | LLM prompt for planning which questions to ask next. Outputs `PlannerOutput` JSON with rationale, targets (gaps), and suggestions. |
| **Constrains** | Must output valid JSON only (no prose). Must prioritize by dependency order: product/scope → personas → workflows → data → API → UI → NFRs → acceptance → plan. Must not re-ask answered questions unless conflict exists. |
| **Must stay consistent with** | SPEC.json (llm.roles.planner, determinism_controls), asker.txt (suggestions feed into asker), ProjectImplementationSpec.schema.json (dependency order matches required_sections). |

---

## 8. `backend/prompts/v1/asker.txt`

| Aspect | Details |
|--------|---------|
| **Governs** | LLM prompt for generating constrained clarifying questions. Outputs `AskOutput` JSON with questions array containing text, type, options, tags, priority, spec_paths. |
| **Constrains** | Must output valid JSON only. Must enforce constrained answer shapes (single/multi preferred over freeform). Must avoid duplicate questions. Must include spec_paths and tags. |
| **Must stay consistent with** | SPEC.json (domain_model.Question.fields, question_rules), OpenAPI Question schema, planner.txt (consumes planner suggestions). |

---

## 9. `backend/prompts/v1/compiler.txt`

| Aspect | Details |
|--------|---------|
| **Governs** | LLM prompt for compiling Q&A bundle into formal spec + trace. Outputs `CompilerOutput` JSON with `spec` (ProjectImplementationSpec) and `trace` objects. |
| **Constrains** | Must output valid JSON only. spec must conform to provided schema. Must preserve stable IDs (FR-001, WF-001, etc.). Must ensure trace coverage for all populated sections. No "TBD" unless absolutely required. |
| **Must stay consistent with** | ProjectImplementationSpec.schema.json (spec must validate), TRACE.schema.json (trace must validate), SPEC.json (llm.hard_rules), validator prompt (follows compilation). |

---

## 10. `backend/prompts/v1/validator_llm_optional.txt`

| Aspect | Details |
|--------|---------|
| **Governs** | LLM prompt for detecting issues in compiled specs. Outputs `ValidatorOutput` JSON with issues array. Issues are drafts (backend hydrates id/project_id/snapshot_id/created_at). |
| **Constrains** | Must output valid JSON only. Must emit error issues for schema validation failures. Must emit warn issues for missing trace coverage. Must detect semantic conflicts (entity references, auth mismatches). Must NOT invent fixes. |
| **Must stay consistent with** | SPEC.json (llm.hard_rules re: IssueDraft hydration), OpenAPI Issue schema (output structure), ACCEPTANCE.md (validates acceptance criteria). |

---

# Remaining Ambiguities

| # | Ambiguity | Location | Impact |
|---|-----------|----------|--------|
| 1 | **Question status transitions** | SPEC.json domain_model.Question | When does status change from `unanswered` → `answered` → `needs_review`? Who/what triggers `needs_review`? |
| 2 | **Seed question schema** | SPEC.json question_seeding.minimum_set | Seed questions are plain strings. What type/options/tags/spec_paths do they get? Backend must infer or hardcode. |
| 3 | **Project.updated_at trigger** | SPEC.json domain_model.Project | What events update `updated_at`? Any answer? Only compilations? Any write? |
| 4 | **Compilation trigger on answer submit** | OpenAPI SubmitAnswerRequest.compile | Default is `true`, but SPEC.json doesn't mention this flag. Is `compile=false` a valid use case? |
| 5 | **Answer value validation** | SPEC.json + OpenAPI | Answer.value accepts any JSON. Should backend validate value shape against Question.type/options? |
| 6 | **Snapshot deduplication** | SPEC.json domain_model.SpecSnapshot | If same answers produce identical spec, should backend create a new snapshot or return existing? |
| 7 | **LLM failure handling** | SPEC.json llm.retry_policy | "Never retry invalid JSON outputs without logging" — but then what? Return 422? Retry with different seed? |
| 8 | **Export SPEC.md template** | SPEC.json exports.ai_coder_pack.derivation | "Deterministic render from snapshot.spec using backend template v1" — but no template v1 is defined. |
| 9 | **Trace granularity** | TRACE.schema.json, compiler.txt | Must trace every array element (`/requirements/functional/0`) or just sections (`/requirements`)? |
| 10 | **GET /projects/{id}/snapshots endpoint** | OpenAPI vs SPEC.json | OpenAPI has `listSnapshots`; SPEC.json api.endpoints doesn't list it (only get single snapshot). |

---

# Implicit Assumptions

| # | Assumption | Inferred From | Risk |
|---|------------|---------------|------|
| 1 | **Single-tenant deployment** | SPEC.json scope.out_of_scope ("MVP is single-user per project") | No auth required for MVP. Multi-user = post-MVP. |
| 2 | **Synchronous compilation** | PLAN.md M3 ("sync for MVP"), SPEC.json behavior | Compilation blocks HTTP request. Large projects may timeout. |
| 3 | **English-only** | All prompts, no i18n mentioned | No internationalization support. |
| 4 | **LLM always available** | No offline mode mentioned | Compilation fails if LLM provider is down. |
| 5 | **Frontend fetches latest snapshot implicitly** | SPEC.json behavior.core_flows | After answer submit, frontend must re-fetch snapshot; not pushed. |
| 6 | **Questions are never deleted** | No delete endpoint in OpenAPI | Questions persist forever; only status changes. |
| 7 | **Answers are never deleted** | Immutability invariant | No soft delete or GDPR purge mechanism. |
| 8 | **Export tokens are opaque random strings** | OpenAPI `/downloads/{token}` minLength=8 | No JWT or signed URL; simple lookup. |
| 9 | **Prompts are loaded from filesystem** | PLAN.md M0 prompt versioning convention | No DB storage for prompts; version = directory name. |
| 10 | **Schema validation is JSON Schema only** | SPEC.json llm.hard_rules | No additional semantic validation beyond what validator LLM catches. |
| 11 | **One LLM provider at a time** | SPEC.json llm.provider.selection="config" | No per-request provider selection. |
| 12 | **Trace is part of spec, not separate table** | ProjectImplementationSpec.schema.json includes trace | Trace stored in snapshot.spec.trace, not separate entity. |
