import { useState } from 'react';
import type { Project } from '../types';

interface ProjectSelectorProps {
  project: Project | null;
  onCreateProject: (name: string) => Promise<void>;
  loading: boolean;
}

export function ProjectSelector({
  project,
  onCreateProject,
  loading,
}: ProjectSelectorProps) {
  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setCreating(true);
    try {
      await onCreateProject(name.trim());
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
      <button type="submit" disabled={creating || !name.trim()}>
        {creating ? 'Creating...' : 'Create Project'}
      </button>
    </form>
  );
}
