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
  NextQuestionsStageEvent,
  SuggestionsStageEvent,
} from '../types';

// In production (Docker), use relative paths. In dev, fall back to localhost:8080
const API_BASE = import.meta.env.VITE_API_URL || (import.meta.env.PROD ? '' : 'http://localhost:8080');

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
        throw new Error(error.message || `Server returned error ${response.status}`);
      }

      return data as T;
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        throw new Error(`Request timed out after ${timeoutMs / 1000} seconds. The server may be overloaded or the LLM is taking longer than expected.`);
      }
      if (err instanceof TypeError && err.message.includes('fetch')) {
        throw new Error('Unable to reach the server. Check your network connection and ensure the backend is running.');
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
      throw new Error(error.message || `Server returned error ${response.status} while deleting project`);
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

  // Next questions with SSE progress streaming
  nextQuestionsStream(
    projectId: string,
    onStage: (event: NextQuestionsStageEvent) => void,
    onError: (event: CompileErrorEvent) => void,
    onComplete: (event: NextQuestionsStageEvent) => void,
    count: number = 5,
    provider?: Provider,
    model?: string
  ): () => void {
    const params = new URLSearchParams();
    params.set('count', count.toString());
    if (provider) params.set('provider', provider);
    if (model) params.set('model', model);
    const url = `${API_BASE}/projects/${projectId}/next-questions/stream?${params}`;

    const eventSource = new EventSource(url);
    let hasReceivedEvent = false;
    let isComplete = false;

    eventSource.addEventListener('stage', (e) => {
      hasReceivedEvent = true;
      const data = JSON.parse(e.data) as NextQuestionsStageEvent;
      onStage(data);
    });

    eventSource.addEventListener('complete', (e) => {
      hasReceivedEvent = true;
      isComplete = true;
      const data = JSON.parse(e.data) as NextQuestionsStageEvent;
      onComplete(data);
      eventSource.close();
    });

    // Note: We use "fail" instead of "error" because "error" is a reserved EventSource event
    eventSource.addEventListener('fail', (e) => {
      const data = JSON.parse(e.data) as CompileErrorEvent;
      onError(data);
      eventSource.close();
    });

    eventSource.onerror = () => {
      if (!isComplete) {
        if (!hasReceivedEvent) {
          onError({ error: 'connection_error', message: 'Unable to connect to server for question generation. Check that the backend is running and try again.' });
        } else {
          onError({ error: 'connection_error', message: 'Connection lost during question generation. The server may have restarted. Please try again.' });
        }
        eventSource.close();
      }
    };

    return () => eventSource.close();
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

    // Note: We use "fail" instead of "error" because "error" is a reserved EventSource event
    eventSource.addEventListener('fail', (e) => {
      const data = JSON.parse(e.data) as CompileErrorEvent;
      onError(data);
      eventSource.close();
    });

    eventSource.onerror = () => {
      // Only report error if we haven't completed successfully
      if (!isComplete) {
        if (!hasReceivedEvent) {
          onError({ error: 'connection_error', message: 'Unable to connect to server for compilation. Check that the backend is running and try again.' });
        } else {
          onError({ error: 'connection_error', message: 'Connection lost during compilation. The server may have restarted or the LLM timed out. Please try again.' });
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

  // Suggestions with SSE progress streaming
  suggestionsStream(
    projectId: string,
    onStage: (event: SuggestionsStageEvent) => void,
    onError: (event: CompileErrorEvent) => void,
    onComplete: (event: SuggestionsStageEvent) => void,
    provider?: Provider,
    model?: string
  ): () => void {
    const params = new URLSearchParams();
    if (provider) params.set('provider', provider);
    if (model) params.set('model', model);
    const query = params.toString();
    const url = `${API_BASE}/projects/${projectId}/suggestions/stream${query ? `?${query}` : ''}`;

    const eventSource = new EventSource(url);
    let hasReceivedEvent = false;
    let isComplete = false;

    eventSource.addEventListener('stage', (e) => {
      hasReceivedEvent = true;
      const data = JSON.parse(e.data) as SuggestionsStageEvent;
      onStage(data);
    });

    eventSource.addEventListener('complete', (e) => {
      hasReceivedEvent = true;
      isComplete = true;
      const data = JSON.parse(e.data) as SuggestionsStageEvent;
      onComplete(data);
      eventSource.close();
    });

    // Note: We use "fail" instead of "error" because "error" is a reserved EventSource event
    eventSource.addEventListener('fail', (e) => {
      const data = JSON.parse(e.data) as CompileErrorEvent;
      onError(data);
      eventSource.close();
    });

    eventSource.onerror = () => {
      if (!isComplete) {
        if (!hasReceivedEvent) {
          onError({ error: 'connection_error', message: 'Unable to connect to server for suggestions. Check that the backend is running.' });
        } else {
          onError({ error: 'connection_error', message: 'Connection lost while generating suggestions. They will be refreshed on next answer.' });
        }
        eventSource.close();
      }
    };

    return () => eventSource.close();
  }
}

export const api = new ApiClient();
