// Domain types matching backend

export type ProjectMode = 'basic' | 'advanced';

export interface Project {
  id: string;
  name: string;
  mode: ProjectMode;
  created_at: string;
  updated_at: string;
}

export type QuestionType = 'single' | 'multi' | 'freeform';
export type QuestionMode = 'basic' | 'advanced';
export type QuestionStatus = 'unanswered' | 'answered' | 'skipped' | 'superseded';

export interface Question {
  id: string;
  project_id: string;
  text: string;
  type: QuestionType;
  options: string[] | null;
  tags: string[];
  priority: number;
  spec_paths: string[];
  status: QuestionStatus;
  created_at: string;
  current_answer?: Answer;
}

export interface Answer {
  id: string;
  project_id: string;
  question_id: string;
  value: unknown;
  version: number;
  supersedes: string | null;
  created_at: string;
}

export interface SpecSnapshot {
  id: string;
  project_id: string;
  spec: Record<string, unknown>;
  created_at: string;
  derived_from: Record<string, number>;
  compiler: CompilerConfig;
}

export interface CompilerConfig {
  model: string;
  prompt_version: string;
  temperature: number;
}

export type IssueType = 'schema_violation' | 'semantic_conflict' | 'missing_info' | 'assumption' | 'ambiguity';
export type IssueSeverity = 'error' | 'warning' | 'info';

export interface Issue {
  id: string;
  project_id: string;
  snapshot_id: string;
  type: IssueType;
  severity: IssueSeverity;
  message: string;
  related_spec_paths: string[];
  related_question_ids: string[];
  created_at: string;
}

// API response types

export interface ListProjectsResponse {
  projects: Project[];
}

export interface CreateProjectResponse {
  project_id: string;
}

export interface GetProjectResponse {
  project: Project;
  latest_snapshot_id: string | null;
}

export interface ListQuestionsResponse {
  questions: Question[];
}

export interface SubmitAnswerResponse {
  answer_id: string;
  snapshot_id: string | null;
  issues: Issue[];
}

export interface CompileResponse {
  snapshot_id: string;
  issues: Issue[];
}

export interface NextQuestionsResponse {
  questions: Question[];
}

export interface GetSnapshotResponse {
  snapshot: SpecSnapshot;
  issues: Issue[];
}

export interface ListSnapshotsResponse {
  snapshots: SpecSnapshot[];
}

export interface ApiError {
  error: string;
  message: string;
  details?: unknown;
}

// LLM Models
export type Provider = 'google' | 'openai' | 'anthropic';

export interface ModelInfo {
  id: string;
  name: string;
  provider: Provider;
}

export interface ProviderInfo {
  id: Provider;
  name: string;
  available: boolean;
  models: ModelInfo[];
}

export interface ListModelsResponse {
  providers: ProviderInfo[];
  default_provider: Provider;
  default_model: string;
}

// Suggestions
export type SuggestionConfidence = 'high' | 'medium' | 'low';

export interface Suggestion {
  question_id: string;
  suggested_value: unknown;
  confidence: SuggestionConfidence;
  reasoning: string;
}

export interface SuggestionsResponse {
  suggestions: Suggestion[];
}

// Compile streaming types
export type CompileStage = 'preparing' | 'compiling' | 'saving' | 'validating' | 'complete';

export interface CompileStageEvent {
  stage: CompileStage;
  message: string;
  elapsed_ms: number;
  total_ms: number;
  snapshot_id?: string;
  issue_count?: number;
}

export interface CompileErrorEvent {
  error: string;
  message: string;
}

// Next questions streaming types
export type NextQuestionsStage = 'preparing' | 'planning' | 'asking' | 'saving' | 'complete';

export interface NextQuestionsStageEvent {
  stage: NextQuestionsStage;
  message: string;
  elapsed_ms: number;
  total_ms: number;
  question_count?: number;
}

// Suggestions streaming types
export type SuggestionsStage = 'preparing' | 'suggesting' | 'complete';

export interface SuggestionsStageEvent {
  stage: SuggestionsStage;
  message: string;
  elapsed_ms: number;
  total_ms: number;
  suggestion_count?: number;
  suggestions?: Suggestion[];
}

// Export format types
export type ExportFormat = 'default' | 'ralph';
