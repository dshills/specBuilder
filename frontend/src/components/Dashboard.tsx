import { useState } from 'react';
import type { Project, ProjectMode } from '../types';

interface DashboardProps {
  projects: Project[];
  onSelectProject: (projectId: string) => void;
  onCreateProject: (name: string, mode: ProjectMode) => Promise<void>;
  onDeleteProject: (projectId: string) => Promise<void>;
  loading: boolean;
}

export function Dashboard({
  projects,
  onSelectProject,
  onCreateProject,
  onDeleteProject,
  loading,
}: DashboardProps) {
  const [name, setName] = useState('');
  const [mode, setMode] = useState<ProjectMode>('basic');
  const [creating, setCreating] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState<Project | null>(null);
  const [deleting, setDeleting] = useState(false);

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

  const handleDeleteClick = (e: React.MouseEvent, project: Project) => {
    e.stopPropagation();
    setDeleteConfirm(project);
  };

  const handleConfirmDelete = async () => {
    if (!deleteConfirm) return;
    setDeleting(true);
    try {
      await onDeleteProject(deleteConfirm.id);
      setDeleteConfirm(null);
    } finally {
      setDeleting(false);
    }
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <h2>Your Projects</h2>
        <p>Select a project to continue or create a new one</p>
      </div>

      <form className="dashboard-create" onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="New project name..."
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
        <button type="submit" className="create-btn" disabled={creating || !name.trim()}>
          {creating ? 'Creating...' : 'Create Project'}
        </button>
      </form>

      <div className="dashboard-projects">
        {loading ? (
          <div className="dashboard-loading">Loading projects...</div>
        ) : projects.length === 0 ? (
          <div className="dashboard-empty">
            <p>No projects yet. Create your first project above.</p>
          </div>
        ) : (
          <div className="project-grid">
            {projects.map((project) => (
              <div key={project.id} className="project-card-wrapper">
                <button
                  className="project-card"
                  onClick={() => onSelectProject(project.id)}
                >
                  <div className="project-card-header">
                    <span className="project-card-name">{project.name}</span>
                    <span className={`project-card-mode ${project.mode}`}>
                      {project.mode === 'basic' ? 'Basic' : 'Advanced'}
                    </span>
                  </div>
                  <div className="project-card-footer">
                    <span className="project-card-date">
                      Updated {formatDate(project.updated_at)}
                    </span>
                  </div>
                </button>
                <button
                  className="project-delete-btn"
                  onClick={(e) => handleDeleteClick(e, project)}
                  title="Delete project"
                >
                  &times;
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {deleteConfirm && (
        <div className="delete-modal-overlay" onClick={() => setDeleteConfirm(null)}>
          <div className="delete-modal" onClick={(e) => e.stopPropagation()}>
            <h3>Delete Project?</h3>
            <p>
              Are you sure you want to delete <strong>{deleteConfirm.name}</strong>?
              This will permanently remove all questions, answers, and specs.
            </p>
            <div className="delete-modal-actions">
              <button
                className="delete-modal-cancel"
                onClick={() => setDeleteConfirm(null)}
                disabled={deleting}
              >
                Cancel
              </button>
              <button
                className="delete-modal-confirm"
                onClick={handleConfirmDelete}
                disabled={deleting}
              >
                {deleting ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
