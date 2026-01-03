import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SpecViewer } from './SpecViewer';
import type { SpecSnapshot } from '../types';

const createSnapshot = (overrides: Partial<SpecSnapshot> = {}): SpecSnapshot => ({
  id: 's1',
  project_id: 'p1',
  spec: { product: { name: 'Test Product' } },
  created_at: '2024-01-01T00:00:00Z',
  derived_from: {},
  compiler: {
    model: 'gpt-4',
    prompt_version: '1.0',
    temperature: 0.7,
  },
  ...overrides,
});

describe('SpecViewer', () => {
  describe('empty state', () => {
    it('shows empty message when no snapshot', () => {
      render(
        <SpecViewer
          snapshot={null}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.getByText('No compiled specification yet.')).toBeInTheDocument();
      expect(screen.getByText(/Answer some questions/)).toBeInTheDocument();
    });
  });

  describe('compile button', () => {
    it('renders compile button', () => {
      render(
        <SpecViewer
          snapshot={null}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.getByRole('button', { name: 'Compile' })).toBeInTheDocument();
    });

    it('calls onCompile when clicked', async () => {
      const user = userEvent.setup();
      const onCompile = vi.fn();

      render(
        <SpecViewer
          snapshot={null}
          onCompile={onCompile}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      await user.click(screen.getByRole('button', { name: 'Compile' }));
      expect(onCompile).toHaveBeenCalled();
    });

    it('shows Compiling... when compiling', () => {
      render(
        <SpecViewer
          snapshot={null}
          onCompile={vi.fn()}
          compiling={true}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.getByRole('button', { name: 'Compiling...' })).toBeInTheDocument();
    });

    it('disables button when disabled', () => {
      render(
        <SpecViewer
          snapshot={null}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={true}
          exportUrl={null}
        />
      );

      expect(screen.getByRole('button', { name: 'Compile' })).toBeDisabled();
    });

    it('disables button when compiling', () => {
      render(
        <SpecViewer
          snapshot={null}
          onCompile={vi.fn()}
          compiling={true}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.getByRole('button', { name: 'Compiling...' })).toBeDisabled();
    });
  });

  describe('snapshot display', () => {
    it('displays spec as JSON', () => {
      const snapshot = createSnapshot({
        spec: { product: { name: 'My Product', purpose: 'Testing' } },
      });

      render(
        <SpecViewer
          snapshot={snapshot}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.getByText(/"name": "My Product"/)).toBeInTheDocument();
      expect(screen.getByText(/"purpose": "Testing"/)).toBeInTheDocument();
    });

    it('displays compiler model', () => {
      const snapshot = createSnapshot({
        compiler: { model: 'claude-3', prompt_version: '2.0', temperature: 0.5 },
      });

      render(
        <SpecViewer
          snapshot={snapshot}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.getByText('Model: claude-3')).toBeInTheDocument();
    });

    it('displays compilation timestamp', () => {
      const snapshot = createSnapshot({
        created_at: '2024-06-15T10:30:00Z',
      });

      render(
        <SpecViewer
          snapshot={snapshot}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.getByText(/Compiled:/)).toBeInTheDocument();
    });
  });

  describe('export button', () => {
    it('does not show export button when no exportUrl', () => {
      render(
        <SpecViewer
          snapshot={createSnapshot()}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl={null}
        />
      );

      expect(screen.queryByRole('link', { name: 'Export Pack' })).not.toBeInTheDocument();
    });

    it('shows export button when exportUrl provided', () => {
      render(
        <SpecViewer
          snapshot={createSnapshot()}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl="http://localhost:8080/projects/p1/export"
        />
      );

      expect(screen.getByRole('link', { name: 'Export Pack' })).toBeInTheDocument();
    });

    it('export link has correct href', () => {
      render(
        <SpecViewer
          snapshot={createSnapshot()}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl="http://localhost:8080/projects/p1/export"
        />
      );

      const link = screen.getByRole('link', { name: 'Export Pack' });
      expect(link).toHaveAttribute('href', 'http://localhost:8080/projects/p1/export');
    });

    it('export link has download attribute', () => {
      render(
        <SpecViewer
          snapshot={createSnapshot()}
          onCompile={vi.fn()}
          compiling={false}
          compileProgress={null}
          disabled={false}
          exportUrl="http://localhost:8080/projects/p1/export"
        />
      );

      const link = screen.getByRole('link', { name: 'Export Pack' });
      expect(link).toHaveAttribute('download');
    });
  });
});
