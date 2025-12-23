import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QuestionCard } from './QuestionCard';
import type { Question } from '../types';

const createQuestion = (overrides: Partial<Question> = {}): Question => ({
  id: 'q1',
  project_id: 'p1',
  text: 'What is the product name?',
  type: 'freeform',
  options: null,
  tags: ['seed'],
  priority: 100,
  spec_paths: ['/product/name'],
  status: 'unanswered',
  created_at: '2024-01-01T00:00:00Z',
  ...overrides,
});

describe('QuestionCard', () => {
  describe('freeform questions', () => {
    it('renders question text', () => {
      const question = createQuestion();
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByText('What is the product name?')).toBeInTheDocument();
    });

    it('renders textarea for freeform questions', () => {
      const question = createQuestion({ type: 'freeform' });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    it('calls onSubmitAnswer when submit button clicked', async () => {
      const user = userEvent.setup();
      const question = createQuestion({ type: 'freeform' });
      const onSubmitAnswer = vi.fn().mockResolvedValue(undefined);

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      const textarea = screen.getByRole('textbox');
      await user.type(textarea, 'My Product');

      await user.click(screen.getByRole('button', { name: 'Submit Answer' }));

      await waitFor(() => {
        expect(onSubmitAnswer).toHaveBeenCalledWith('q1', 'My Product');
      });
    });

    it('disables submit button when disabled prop is true', () => {
      const question = createQuestion();
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={true}
        />
      );

      expect(screen.getByRole('button', { name: 'Submit Answer' })).toBeDisabled();
    });

    it('disables submit button when answer is empty', () => {
      const question = createQuestion();
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByRole('button', { name: 'Submit Answer' })).toBeDisabled();
    });
  });

  describe('single choice questions', () => {
    it('renders radio buttons for options', () => {
      const question = createQuestion({
        type: 'single',
        options: ['Option A', 'Option B', 'Option C'],
      });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByRole('radio', { name: 'Option A' })).toBeInTheDocument();
      expect(screen.getByRole('radio', { name: 'Option B' })).toBeInTheDocument();
      expect(screen.getByRole('radio', { name: 'Option C' })).toBeInTheDocument();
    });

    it('allows selecting a single option', async () => {
      const user = userEvent.setup();
      const question = createQuestion({
        type: 'single',
        options: ['PostgreSQL', 'MySQL', 'SQLite'],
      });
      const onSubmitAnswer = vi.fn().mockResolvedValue(undefined);

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      await user.click(screen.getByRole('radio', { name: 'PostgreSQL' }));
      await user.click(screen.getByRole('button', { name: 'Submit Answer' }));

      await waitFor(() => {
        expect(onSubmitAnswer).toHaveBeenCalledWith('q1', 'PostgreSQL');
      });
    });
  });

  describe('multi choice questions', () => {
    it('renders checkboxes for options', () => {
      const question = createQuestion({
        type: 'multi',
        options: ['Feature A', 'Feature B', 'Feature C'],
      });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByRole('checkbox', { name: 'Feature A' })).toBeInTheDocument();
      expect(screen.getByRole('checkbox', { name: 'Feature B' })).toBeInTheDocument();
      expect(screen.getByRole('checkbox', { name: 'Feature C' })).toBeInTheDocument();
    });

    it('allows selecting multiple options', async () => {
      const user = userEvent.setup();
      const question = createQuestion({
        type: 'multi',
        options: ['Feature A', 'Feature B', 'Feature C'],
      });
      const onSubmitAnswer = vi.fn().mockResolvedValue(undefined);

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      await user.click(screen.getByRole('checkbox', { name: 'Feature A' }));
      await user.click(screen.getByRole('checkbox', { name: 'Feature C' }));
      await user.click(screen.getByRole('button', { name: 'Submit Answer' }));

      await waitFor(() => {
        expect(onSubmitAnswer).toHaveBeenCalledWith('q1', ['Feature A', 'Feature C']);
      });
    });
  });

  describe('answered questions', () => {
    it('does not render input for answered questions', () => {
      const question = createQuestion({ status: 'answered' });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Submit Answer' })).not.toBeInTheDocument();
    });

    it('shows answered status badge', () => {
      const question = createQuestion({ status: 'answered' });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByText('answered')).toBeInTheDocument();
    });
  });

  describe('metadata display', () => {
    it('displays question tags', () => {
      const question = createQuestion({ tags: ['data_model', 'core'] });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByText('data_model')).toBeInTheDocument();
      expect(screen.getByText('core')).toBeInTheDocument();
    });

    it('displays spec paths', () => {
      const question = createQuestion({
        spec_paths: ['/product/name', '/product/description'],
      });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByText('/product/name')).toBeInTheDocument();
      expect(screen.getByText('/product/description')).toBeInTheDocument();
    });

    it('displays question type', () => {
      const question = createQuestion({ type: 'freeform' });
      const onSubmitAnswer = vi.fn();

      render(
        <QuestionCard
          question={question}
          onSubmitAnswer={onSubmitAnswer}
          disabled={false}
        />
      );

      expect(screen.getByText('freeform')).toBeInTheDocument();
    });
  });
});
