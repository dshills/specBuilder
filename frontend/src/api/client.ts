import type {
  ListProjectsResponse,
  CreateProjectResponse,
  GetProjectResponse,
  ListQuestionsResponse,
  SubmitAnswerResponse,
  CompileResponse,
  NextQuestionsResponse,
  GetSnapshotResponse,
  ListSnapshotsResponse,
  ListModelsResponse,
  SuggestionsResponse,
  Provider,
  ProjectMode,
  ApiError,
  CompileStageEvent,
  CompileErrorEvent,
} from '../types';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

class ApiClient {
  private async request<T>(
    path: string,
    options?: RequestInit,
    timeoutMs: number = 30000
  ): Promise<T> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

    try {
      const response = await fetch(`${API_BASE}${path}`, {
        ...options,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          ...options?.headers,
        },
      });

      const data = await response.json();

      if (!response.ok) {
        const error = data as ApiError;
        throw new Error(error.message || 'API request failed');
      }

      return data as T;
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        throw new Error(`Request timed out after ${timeoutMs / 1000}s`);
      }
      throw err;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  // Projects
  async listProjects(): Promise<ListProjectsResponse> {
    return this.request<ListProjectsResponse>('/projects');
  }

  async deleteProject(projectId: string): Promise<void> {
    const response = await fetch(`${API_BASE}/projects/${projectId}`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Failed to delete project');
    }
  }

  async createProject(name: string, mode: ProjectMode = 'advanced'): Promise<CreateProjectResponse> {
    return this.request<CreateProjectResponse>('/projects', {
      method: 'POST',
      body: JSON.stringify({ name, mode }),
    });
  }

  async getProject(projectId: string): Promise<GetProjectResponse> {
    return this.request<GetProjectResponse>(`/projects/${projectId}`);
  }

  // Questions
  async listQuestions(
    projectId: string,
    status?: string,
    tag?: string
  ): Promise<ListQuestionsResponse> {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    if (tag) params.set('tag', tag);
    const query = params.toString();
    return this.request<ListQuestionsResponse>(
      `/projects/${projectId}/questions${query ? `?${query}` : ''}`
    );
  }

  async generateNextQuestions(
    projectId: string,
    count: number = 5
  ): Promise<NextQuestionsResponse> {
    // Question generation calls LLM (planner + asker), can take 1-2 minutes
    return this.request<NextQuestionsResponse>(
      `/projects/${projectId}/next-questions`,
      {
        method: 'POST',
        body: JSON.stringify({ count }),
      },
      180000 // 3 minute timeout
    );
  }

  // Answers
  async submitAnswer(
    projectId: string,
    questionId: string,
    value: unknown,
    compile: boolean = false
  ): Promise<SubmitAnswerResponse> {
    return this.request<SubmitAnswerResponse>(`/projects/${projectId}/answers`, {
      method: 'POST',
      body: JSON.stringify({
        question_id: questionId,
        value,
        compile,
      }),
    });
  }

  // Models
  async listModels(): Promise<ListModelsResponse> {
    return this.request<ListModelsResponse>('/models');
  }

  // Compilation
  async compile(
    projectId: string,
    provider?: Provider,
    model?: string
  ): Promise<CompileResponse> {
    // Compilation can take 2-5 minutes for large specs
    return this.request<CompileResponse>(
      `/projects/${projectId}/compile`,
      {
        method: 'POST',
        body: JSON.stringify({
          mode: 'latest_answers',
          provider: provider || undefined,
          model: model || undefined,
        }),
      },
      300000 // 5 minute timeout
    );
  }

  // Compilation with SSE progress streaming
  compileStream(
    projectId: string,
    onStage: (event: CompileStageEvent) => void,
    onError: (event: CompileErrorEvent) => void,
    onComplete: (event: CompileStageEvent) => void,
    provider?: Provider,
    model?: string
  ): () => void {
    const params = new URLSearchParams();
    if (provider) params.set('provider', provider);
    if (model) params.set('model', model);
    const query = params.toString();
    const url = `${API_BASE}/projects/${projectId}/compile/stream${query ? `?${query}` : ''}`;

    const eventSource = new EventSource(url);
    let hasReceivedEvent = false;
    let isComplete = false;

    eventSource.addEventListener('stage', (e) => {
      hasReceivedEvent = true;
      const data = JSON.parse(e.data) as CompileStageEvent;
      onStage(data);
    });

    eventSource.addEventListener('complete', (e) => {
      hasReceivedEvent = true;
      isComplete = true;
      const data = JSON.parse(e.data) as CompileStageEvent;
      onComplete(data);
      eventSource.close();
    });

    eventSource.addEventListener('error', (e) => {
      if (e instanceof MessageEvent && e.data) {
        const data = JSON.parse(e.data) as CompileErrorEvent;
        onError(data);
        eventSource.close();
      }
      // Don't handle generic errors here - let onerror handle them
    });

    eventSource.onerror = () => {
      // Only report error if we haven't completed successfully
      if (!isComplete) {
        if (!hasReceivedEvent) {
          onError({ error: 'connection_error', message: 'Failed to connect to server' });
        } else {
          onError({ error: 'connection_error', message: 'Connection to server lost' });
        }
        eventSource.close();
      }
    };

    // Return cleanup function
    return () => eventSource.close();
  }

  // Snapshots
  async listSnapshots(
    projectId: string,
    limit: number = 50
  ): Promise<ListSnapshotsResponse> {
    return this.request<ListSnapshotsResponse>(
      `/projects/${projectId}/snapshots?limit=${limit}`
    );
  }

  async getSnapshot(
    projectId: string,
    snapshotId: string
  ): Promise<GetSnapshotResponse> {
    return this.request<GetSnapshotResponse>(
      `/projects/${projectId}/snapshots/${snapshotId}`
    );
  }

  // Export
  getExportUrl(projectId: string, snapshotId?: string): string {
    const base = `${API_BASE}/projects/${projectId}/export`;
    if (snapshotId) {
      return `${base}?snapshot_id=${snapshotId}`;
    }
    return base;
  }

  // Suggestions
  async generateSuggestions(projectId: string): Promise<SuggestionsResponse> {
    // Suggestion generation calls LLM, can take 1-2 minutes
    return this.request<SuggestionsResponse>(
      `/projects/${projectId}/suggestions`,
      { method: 'POST' },
      120000 // 2 minute timeout
    );
  }
}

export const api = new ApiClient();
