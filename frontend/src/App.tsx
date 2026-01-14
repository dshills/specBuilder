import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from './api/client';
import { Dashboard, IssuesPanel, ModelSelector, ProjectSelector, QuestionList, SpecViewer, ThemeToggle } from './components';
import type { Project, Question, SpecSnapshot, Issue, ProviderInfo, Provider, ProjectMode, Suggestion, CompileStageEvent, NextQuestionsStageEvent, SuggestionsStageEvent } from './types';
import './App.css';

type Theme = 'light' | 'dark';

function getInitialTheme(): Theme {
  const saved = localStorage.getItem('specbuilder_theme');
  if (saved === 'light' || saved === 'dark') {
    return saved;
  }
  // Check system preference
  if (window.matchMedia?.('(prefers-color-scheme: light)').matches) {
    return 'light';
  }
  return 'dark';
}

function App() {
  // Theme state
  const [theme, setTheme] = useState<Theme>(getInitialTheme);

  // Apply theme to document
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('specbuilder_theme', theme);
  }, [theme]);

  const handleToggleTheme = useCallback(() => {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'));
  }, []);

  // State
  const [projects, setProjects] = useState<Project[]>([]);
  const [project, setProject] = useState<Project | null>(null);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [snapshot, setSnapshot] = useState<SpecSnapshot | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [suggestions, setSuggestions] = useState<Suggestion[]>([]);

  // Model state
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<Provider | null>(null);
  const [selectedModel, setSelectedModel] = useState<string | null>(null);

  // Loading states
  const [loadingProjects, setLoadingProjects] = useState(false);
  const [loadingProject, setLoadingProject] = useState(false);
  const [loadingQuestions, setLoadingQuestions] = useState(false);
  const [compiling, setCompiling] = useState(false);
  const [compileProgress, setCompileProgress] = useState<CompileStageEvent | null>(null);
  const [generating, setGenerating] = useState(false);
  const [generateProgress, setGenerateProgress] = useState<NextQuestionsStageEvent | null>(null);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [suggestionsProgress, setSuggestionsProgress] = useState<SuggestionsStageEvent | null>(null);

  // Refs for SSE cleanup
  const compileCleanupRef = useRef<(() => void) | null>(null);
  const generateCleanupRef = useRef<(() => void) | null>(null);
  const suggestionsCleanupRef = useRef<(() => void) | null>(null);

  // Cleanup SSE connections on unmount to prevent memory leaks
  useEffect(() => {
    return () => {
      compileCleanupRef.current?.();
      generateCleanupRef.current?.();
      suggestionsCleanupRef.current?.();
    };
  }, []);

  // Error state
  const [error, setError] = useState<string | null>(null);

  // Highlighted questions (from clicking issues)
  const [highlightedQuestionIds, setHighlightedQuestionIds] = useState<string[]>([]);

  // Load models on mount
  useEffect(() => {
    const loadModels = async () => {
      try {
        const { providers, default_provider, default_model } = await api.listModels();
        setProviders(providers);
        setSelectedProvider(default_provider);
        setSelectedModel(default_model);
      } catch (err) {
        // Models endpoint failed, LLM not configured
        console.warn('Failed to load models:', err);
      }
    };
    loadModels();
  }, []);

  const loadProjects = useCallback(async () => {
    setLoadingProjects(true);
    try {
      const { projects } = await api.listProjects();
      setProjects(projects);
    } catch (err) {
      console.warn('Failed to load projects:', err);
    } finally {
      setLoadingProjects(false);
    }
  }, []);

  const loadQuestions = useCallback(async (projectId: string) => {
    setLoadingQuestions(true);
    try {
      const { questions } = await api.listQuestions(projectId);
      setQuestions(questions);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setError(`Unable to load questions: ${message}. Try refreshing the page.`);
    } finally {
      setLoadingQuestions(false);
    }
  }, []);

  const loadProject = useCallback(async (projectId: string) => {
    setLoadingProject(true);
    setError(null);
    try {
      const { project, latest_snapshot_id } = await api.getProject(projectId);
      setProject(project);
      localStorage.setItem('specbuilder_project_id', projectId);

      // Load questions
      await loadQuestions(projectId);

      // Load latest snapshot if exists
      if (latest_snapshot_id) {
        const { snapshot, issues } = await api.getSnapshot(
          projectId,
          latest_snapshot_id
        );
        setSnapshot(snapshot);
        setIssues(issues);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setError(`Unable to load project: ${message}. The project may have been deleted or the server is unavailable.`);
      localStorage.removeItem('specbuilder_project_id');
    } finally {
      setLoadingProject(false);
    }
  }, [loadQuestions]);

  // Load projects and check for saved project on mount
  useEffect(() => {
    const savedProjectId = localStorage.getItem('specbuilder_project_id');
    if (savedProjectId) {
      loadProject(savedProjectId);
    } else {
      loadProjects();
    }
  }, [loadProject, loadProjects]);

  const refreshSuggestions = useCallback((projectId: string, provider?: Provider | null, model?: string | null) => {
    // Clean up any existing SSE connection
    if (suggestionsCleanupRef.current) {
      suggestionsCleanupRef.current();
    }

    setLoadingSuggestions(true);
    setSuggestionsProgress(null);

    const cleanup = api.suggestionsStream(
      projectId,
      // onStage
      (event) => {
        setSuggestionsProgress(event);
      },
      // onError
      (event) => {
        // Suggestions are optional, don't show error banner
        console.warn('Failed to generate suggestions:', event.message);
        setLoadingSuggestions(false);
        setSuggestionsProgress(null);
      },
      // onComplete
      (event) => {
        // Use suggestions from the complete event
        if (event.suggestions) {
          setSuggestions(event.suggestions);
        }
        setLoadingSuggestions(false);
        setSuggestionsProgress(null);
      },
      provider || undefined,
      model || undefined
    );

    suggestionsCleanupRef.current = cleanup;
  }, []);

  const handleCreateProject = useCallback(async (name: string, mode: ProjectMode) => {
    setError(null);
    try {
      const { project_id } = await api.createProject(name, mode);
      await loadProject(project_id);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setError(`Unable to create project "${name}": ${message}. Check that the server is running and try again.`);
    }
  }, []);

  const handleDeleteProject = useCallback(async (projectId: string) => {
    setError(null);
    try {
      await api.deleteProject(projectId);
      await loadProjects();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setError(`Unable to delete project: ${message}. The project may already be deleted or is in use.`);
    }
  }, [loadProjects]);

  const handleSubmitAnswer = useCallback(
    async (questionId: string, value: unknown) => {
      if (!project) return;
      setError(null);
      try {
        await api.submitAnswer(project.id, questionId, value, false);
        // Refresh questions to update status
        await loadQuestions(project.id);
        // Refresh suggestions in background (don't await)
        refreshSuggestions(project.id, selectedProvider, selectedModel);
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        setError(`Unable to save your answer: ${message}. Your changes may not have been saved.`);
      }
    },
    [project, refreshSuggestions, selectedProvider, selectedModel]
  );

  const handleGenerateMore = useCallback(() => {
    if (!project) return;

    // Clean up any existing SSE connection
    if (generateCleanupRef.current) {
      generateCleanupRef.current();
    }

    setGenerating(true);
    setGenerateProgress(null);
    setError(null);

    const cleanup = api.nextQuestionsStream(
      project.id,
      // onStage
      (event) => {
        setGenerateProgress(event);
      },
      // onError
      (event) => {
        setError(event.message);
        setGenerating(false);
        setGenerateProgress(null);
      },
      // onComplete
      async () => {
        // Refresh questions list to get newly created questions
        await loadQuestions(project.id);
        // Refresh suggestions for the new questions
        refreshSuggestions(project.id, selectedProvider, selectedModel);
        setGenerating(false);
        setGenerateProgress(null);
      },
      5, // count
      selectedProvider || undefined,
      selectedModel || undefined
    );

    generateCleanupRef.current = cleanup;
  }, [project, refreshSuggestions, selectedProvider, selectedModel]);

  const handleCompile = useCallback(() => {
    if (!project) return;

    // Clean up any existing SSE connection
    if (compileCleanupRef.current) {
      compileCleanupRef.current();
    }

    setCompiling(true);
    setCompileProgress(null);
    setError(null);

    const cleanup = api.compileStream(
      project.id,
      // onStage
      (event) => {
        setCompileProgress(event);
      },
      // onError
      (event) => {
        setError(event.message);
        setCompiling(false);
        setCompileProgress(null);
      },
      // onComplete
      async (event) => {
        if (event.snapshot_id) {
          try {
            const { snapshot, issues: newIssues } = await api.getSnapshot(project.id, event.snapshot_id);
            setSnapshot(snapshot);
            setIssues(newIssues);
          } catch (err) {
            const message = err instanceof Error ? err.message : 'Unknown error';
            setError(`Compilation succeeded but failed to load the result: ${message}. Try clicking Compile again.`);
          }
        }
        setCompiling(false);
        setCompileProgress(null);
      },
      selectedProvider || undefined,
      selectedModel || undefined
    );

    compileCleanupRef.current = cleanup;
  }, [project, selectedProvider, selectedModel]);

  const handleModelSelect = useCallback((provider: Provider, model: string) => {
    setSelectedProvider(provider);
    setSelectedModel(model);
  }, []);

  const handleNewProject = useCallback(() => {
    localStorage.removeItem('specbuilder_project_id');
    setProject(null);
    setQuestions([]);
    setSnapshot(null);
    setIssues([]);
    setSuggestions([]);
    setError(null);
    setHighlightedQuestionIds([]);
    loadProjects();
  }, [loadProjects]);

  const handleIssueClick = useCallback((questionIds: string[]) => {
    setHighlightedQuestionIds(questionIds);
  }, []);

  const handleClearHighlight = useCallback(() => {
    setHighlightedQuestionIds([]);
  }, []);

  const answeredCount = questions.filter((q) => q.status === 'answered').length;
  const isDisabled = !project || compiling || generating;

  return (
    <div className="app">
      <header className="app-header">
        <h1>Spec Builder</h1>
        <p>Question-driven specification compiler for AI coding agents</p>
        {project && (
          <button className="dashboard-btn" onClick={handleNewProject}>
            Dashboard
          </button>
        )}
        <ThemeToggle theme={theme} onToggle={handleToggleTheme} />
      </header>

      {error && (
        <div className="error-banner">
          <span>{error}</span>
          <button onClick={() => setError(null)}>Dismiss</button>
        </div>
      )}

      {project ? (
        <div className="app-content">
          <section className="project-section">
            <ProjectSelector
              project={project}
              onCreateProject={handleCreateProject}
              loading={loadingProject}
            />
            {providers.length > 0 && (
              <ModelSelector
                providers={providers}
                selectedProvider={selectedProvider}
                selectedModel={selectedModel}
                onSelect={handleModelSelect}
                disabled={compiling || generating}
              />
            )}
          </section>

          <div className="main-columns">
            <section className="questions-section">
              <h2>
                Questions{' '}
                <span className="progress">
                  ({answeredCount}/{questions.length} answered)
                </span>
              </h2>
              <QuestionList
                questions={questions}
                suggestions={suggestions}
                highlightedIds={highlightedQuestionIds}
                onSubmitAnswer={handleSubmitAnswer}
                onGenerateMore={handleGenerateMore}
                onClearHighlight={handleClearHighlight}
                disabled={isDisabled}
                loading={loadingQuestions}
                generating={generating}
                generateProgress={generateProgress}
                loadingSuggestions={loadingSuggestions}
                suggestionsProgress={suggestionsProgress}
                hasAnswers={answeredCount > 0}
              />
            </section>

            <section className="spec-section">
              <SpecViewer
                snapshot={snapshot}
                onCompile={handleCompile}
                compiling={compiling}
                compileProgress={compileProgress}
                disabled={isDisabled || answeredCount === 0}
                projectId={project?.id || null}
              />
            </section>
          </div>

          <IssuesPanel issues={issues} onIssueClick={handleIssueClick} />
        </div>
      ) : (
        <Dashboard
          projects={projects}
          onSelectProject={loadProject}
          onCreateProject={handleCreateProject}
          onDeleteProject={handleDeleteProject}
          loading={loadingProjects || loadingProject}
        />
      )}
    </div>
  );
}

export default App;
