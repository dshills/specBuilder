import { useState, useEffect, useCallback } from 'react';
import { api } from './api/client';
import { ProjectSelector, QuestionList, SpecViewer } from './components';
import type { Project, Question, SpecSnapshot, Issue } from './types';
import './App.css';

function App() {
  // State
  const [project, setProject] = useState<Project | null>(null);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [snapshot, setSnapshot] = useState<SpecSnapshot | null>(null);
  const [issues, setIssues] = useState<Issue[]>([]);

  // Loading states
  const [loadingProject, setLoadingProject] = useState(false);
  const [loadingQuestions, setLoadingQuestions] = useState(false);
  const [compiling, setCompiling] = useState(false);
  const [generating, setGenerating] = useState(false);

  // Error state
  const [error, setError] = useState<string | null>(null);

  // Load project from localStorage
  useEffect(() => {
    const savedProjectId = localStorage.getItem('specbuilder_project_id');
    if (savedProjectId) {
      loadProject(savedProjectId);
    }
  }, []);

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

  const handleCreateProject = useCallback(async (name: string) => {
    setError(null);
    try {
      const { project_id } = await api.createProject(name);
      await loadProject(project_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project');
    }
  }, []);

  const handleSubmitAnswer = useCallback(
    async (questionId: string, value: unknown) => {
      if (!project) return;
      setError(null);
      try {
        await api.submitAnswer(project.id, questionId, value, false);
        // Refresh questions to update status
        await loadQuestions(project.id);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : 'Failed to submit answer'
        );
      }
    },
    [project]
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
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to generate questions'
      );
    } finally {
      setGenerating(false);
    }
  }, [project]);

  const handleCompile = useCallback(async () => {
    if (!project) return;
    setCompiling(true);
    setError(null);
    try {
      const { snapshot_id, issues: newIssues } = await api.compile(project.id);
      const { snapshot } = await api.getSnapshot(project.id, snapshot_id);
      setSnapshot(snapshot);
      setIssues(newIssues);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to compile');
    } finally {
      setCompiling(false);
    }
  }, [project]);

  const handleNewProject = useCallback(() => {
    localStorage.removeItem('specbuilder_project_id');
    setProject(null);
    setQuestions([]);
    setSnapshot(null);
    setIssues([]);
    setError(null);
  }, []);

  const answeredCount = questions.filter((q) => q.status === 'answered').length;
  const isDisabled = !project || compiling || generating;

  return (
    <div className="app">
      <header className="app-header">
        <h1>Spec Builder</h1>
        <p>Question-driven specification compiler for AI coding agents</p>
        {project && (
          <button className="new-project-btn" onClick={handleNewProject}>
            New Project
          </button>
        )}
      </header>

      {error && (
        <div className="error-banner">
          <span>{error}</span>
          <button onClick={() => setError(null)}>Dismiss</button>
        </div>
      )}

      <div className="app-content">
        <section className="project-section">
          <ProjectSelector
            project={project}
            onCreateProject={handleCreateProject}
            loading={loadingProject}
          />
        </section>

        {project && (
          <>
            <section className="questions-section">
              <h2>
                Questions{' '}
                <span className="progress">
                  ({answeredCount}/{questions.length} answered)
                </span>
              </h2>
              <QuestionList
                questions={questions}
                onSubmitAnswer={handleSubmitAnswer}
                onGenerateMore={handleGenerateMore}
                disabled={isDisabled}
                loading={loadingQuestions}
              />
            </section>

            <section className="spec-section">
              <SpecViewer
                snapshot={snapshot}
                issues={issues}
                onCompile={handleCompile}
                compiling={compiling}
                disabled={isDisabled || answeredCount === 0}
                exportUrl={snapshot ? api.getExportUrl(project.id, snapshot.id) : null}
              />
            </section>
          </>
        )}
      </div>
    </div>
  );
}

export default App;
