import { useState } from 'react';
import type { Project, ProjectMode } from '../types';

interface ProjectSelectorProps {
  project: Project | null;
  onCreateProject: (name: string, mode: ProjectMode) => Promise<void>;
  loading: boolean;
}

export function ProjectSelector({
  project,
  onCreateProject,
  loading,
}: ProjectSelectorProps) {
  const [name, setName] = useState('');
  const [mode, setMode] = useState<ProjectMode>('basic');
  const [creating, setCreating] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setCreating(true);
    try {
      await onCreateProject(name.trim(), mode);
      setName('');
    } finally {
      setCreating(false);
    }
  };

  if (loading) {
    return <div className="project-selector loading">Loading...</div>;
  }

  if (project) {
    return (
      <div className="project-selector active">
        <span className="project-label">Project:</span>
        <span className="project-name">{project.name}</span>
        <span className="project-mode">{project.mode === 'basic' ? 'Basic' : 'Advanced'}</span>
      </div>
    );
  }

  return (
    <form className="project-selector create" onSubmit={handleSubmit}>
      <input
        type="text"
        placeholder="Enter project name..."
        value={name}
        onChange={(e) => setName(e.target.value)}
        disabled={creating}
      />
      <div className="mode-selector">
        <button
          type="button"
          className={mode === 'basic' ? 'active' : ''}
          onClick={() => setMode('basic')}
          disabled={creating}
          title="Simple questions for non-technical users"
        >
          Basic
        </button>
        <button
          type="button"
          className={mode === 'advanced' ? 'active' : ''}
          onClick={() => setMode('advanced')}
          disabled={creating}
          title="Technical questions for developers"
        >
          Advanced
        </button>
      </div>
      <button type="submit" disabled={creating || !name.trim()}>
        {creating ? 'Creating...' : 'Create Project'}
      </button>
    </form>
  );
}
