import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProjectSelector } from './ProjectSelector';
import type { Project } from '../types';

const createProject = (overrides: Partial<Project> = {}): Project => ({
  id: 'p1',
  name: 'Test Project',
  mode: 'basic',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  ...overrides,
});

describe('ProjectSelector', () => {
  describe('loading state', () => {
    it('shows loading indicator when loading', () => {
      render(
        <ProjectSelector
          project={null}
          onCreateProject={vi.fn()}
          loading={true}
        />
      );

      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });
  });

  describe('no project selected', () => {
    it('renders create form when no project', () => {
      render(
        <ProjectSelector
          project={null}
          onCreateProject={vi.fn()}
          loading={false}
        />
      );

      expect(screen.getByRole('textbox')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Create Project' })).toBeInTheDocument();
    });

    it('disables create button when name is empty', () => {
      render(
        <ProjectSelector
          project={null}
          onCreateProject={vi.fn()}
          loading={false}
        />
      );

      expect(screen.getByRole('button', { name: 'Create Project' })).toBeDisabled();
    });

    it('enables create button when name is entered', async () => {
      const user = userEvent.setup();

      render(
        <ProjectSelector
          project={null}
          onCreateProject={vi.fn()}
          loading={false}
        />
      );

      await user.type(screen.getByRole('textbox'), 'My Project');

      expect(screen.getByRole('button', { name: 'Create Project' })).toBeEnabled();
    });

    it('calls onCreateProject when form submitted', async () => {
      const user = userEvent.setup();
      const onCreateProject = vi.fn().mockResolvedValue(undefined);

      render(
        <ProjectSelector
          project={null}
          onCreateProject={onCreateProject}
          loading={false}
        />
      );

      await user.type(screen.getByRole('textbox'), 'My Project');
      await user.click(screen.getByRole('button', { name: 'Create Project' }));

      await waitFor(() => {
        expect(onCreateProject).toHaveBeenCalledWith('My Project', 'basic');
      });
    });

    it('trims whitespace from project name', async () => {
      const user = userEvent.setup();
      const onCreateProject = vi.fn().mockResolvedValue(undefined);

      render(
        <ProjectSelector
          project={null}
          onCreateProject={onCreateProject}
          loading={false}
        />
      );

      await user.type(screen.getByRole('textbox'), '  My Project  ');
      await user.click(screen.getByRole('button', { name: 'Create Project' }));

      await waitFor(() => {
        expect(onCreateProject).toHaveBeenCalledWith('My Project', 'basic');
      });
    });

    it('clears input after successful creation', async () => {
      const user = userEvent.setup();
      const onCreateProject = vi.fn().mockResolvedValue(undefined);

      render(
        <ProjectSelector
          project={null}
          onCreateProject={onCreateProject}
          loading={false}
        />
      );

      const input = screen.getByRole('textbox');
      await user.type(input, 'My Project');
      await user.click(screen.getByRole('button', { name: 'Create Project' }));

      await waitFor(() => {
        expect(input).toHaveValue('');
      });
    });

    it('shows Creating... text while submitting', async () => {
      const user = userEvent.setup();
      const onCreateProject = vi.fn().mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      );

      render(
        <ProjectSelector
          project={null}
          onCreateProject={onCreateProject}
          loading={false}
        />
      );

      await user.type(screen.getByRole('textbox'), 'My Project');
      await user.click(screen.getByRole('button', { name: 'Create Project' }));

      expect(screen.getByRole('button', { name: 'Creating...' })).toBeInTheDocument();
    });
  });

  describe('project selected', () => {
    it('displays project name when project exists', () => {
      const project = createProject({ name: 'Awesome Project' });

      render(
        <ProjectSelector
          project={project}
          onCreateProject={vi.fn()}
          loading={false}
        />
      );

      expect(screen.getByText('Project:')).toBeInTheDocument();
      expect(screen.getByText('Awesome Project')).toBeInTheDocument();
    });

    it('does not show create form when project exists', () => {
      const project = createProject();

      render(
        <ProjectSelector
          project={project}
          onCreateProject={vi.fn()}
          loading={false}
        />
      );

      expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Create Project' })).not.toBeInTheDocument();
    });
  });
});
