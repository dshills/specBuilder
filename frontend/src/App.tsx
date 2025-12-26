import { useState, useEffect, useCallback } from 'react';
import { api } from './api/client';
import { Dashboard, IssuesPanel, ModelSelector, ProjectSelector, QuestionList, SpecViewer } from './components';
import type { Project, Question, SpecSnapshot, Issue, ProviderInfo, Provider, ProjectMode, Suggestion } from './types';
import './App.css';

function App() {
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
  const [generating, setGenerating] = useState(false);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);

  // Error state
  const [error, setError] = useState<string | null>(null);

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

  // Load projects and check for saved project on mount
  useEffect(() => {
    const savedProjectId = localStorage.getItem('specbuilder_project_id');
    if (savedProjectId) {
      loadProject(savedProjectId);
    } else {
      loadProjects();
    }
  }, [loadProjects]);

  const loadProject = async (projectId: string) => {
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
      setError(err instanceof Error ? err.message : 'Failed to load project');
      localStorage.removeItem('specbuilder_project_id');
    } finally {
      setLoadingProject(false);
    }
  };

  const loadQuestions = async (projectId: string) => {
    setLoadingQuestions(true);
    try {
      const { questions } = await api.listQuestions(projectId);
      setQuestions(questions);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to load questions'
      );
    } finally {
      setLoadingQuestions(false);
    }
  };

  const refreshSuggestions = useCallback(async (projectId: string) => {
    // Generate suggestions in background - don't block UI
    setLoadingSuggestions(true);
    try {
      const { suggestions: newSuggestions } = await api.generateSuggestions(projectId);
      setSuggestions(newSuggestions);
    } catch (err) {
      // Suggestions are optional, don't show error banner
      console.warn('Failed to generate suggestions:', err);
    } finally {
      setLoadingSuggestions(false);
    }
  }, []);

  const handleCreateProject = useCallback(async (name: string, mode: ProjectMode) => {
    setError(null);
    try {
      const { project_id } = await api.createProject(name, mode);
      await loadProject(project_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    }
  }, []);

  const handleDeleteProject = useCallback(async (projectId: string) => {
    setError(null);
    try {
      await api.deleteProject(projectId);
      await loadProjects();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete project');
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
        refreshSuggestions(project.id);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : 'Failed to submit answer'
        );
      }
    },
    [project, refreshSuggestions]
  );

  const handleGenerateMore = useCallback(async () => {
    if (!project) return;
    setGenerating(true);
    setError(null);
    try {
      const { questions: newQuestions } = await api.generateNextQuestions(
        project.id,
        5
      );
      setQuestions((prev) => [...prev, ...newQuestions]);
      // Refresh suggestions for the new questions (don't await)
      refreshSuggestions(project.id);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to generate questions'
      );
    } finally {
      setGenerating(false);
    }
  }, [project, refreshSuggestions]);

  const handleCompile = useCallback(async () => {
    if (!project) return;
    setCompiling(true);
    setError(null);
    try {
      const { snapshot_id, issues: newIssues } = await api.compile(
        project.id,
        selectedProvider || undefined,
        selectedModel || undefined
      );
      const { snapshot } = await api.getSnapshot(project.id, snapshot_id);
      setSnapshot(snapshot);
      setIssues(newIssues);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to compile');
    } finally {
      setCompiling(false);
    }
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
    loadProjects();
  }, [loadProjects]);

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
                onSubmitAnswer={handleSubmitAnswer}
                onGenerateMore={handleGenerateMore}
                disabled={isDisabled}
                loading={loadingQuestions}
                loadingSuggestions={loadingSuggestions}
              />
            </section>

            <section className="spec-section">
              <SpecViewer
                snapshot={snapshot}
                onCompile={handleCompile}
                compiling={compiling}
                disabled={isDisabled || answeredCount === 0}
                exportUrl={snapshot ? api.getExportUrl(project.id, snapshot.id) : null}
              />
            </section>
          </div>

          <IssuesPanel issues={issues} />
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
