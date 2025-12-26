import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QuestionList } from './QuestionList';
import type { Question } from '../types';

const createQuestion = (overrides: Partial<Question> = {}): Question => ({
  id: 'q1',
  project_id: 'p1',
  text: 'Sample question?',
  type: 'freeform',
  options: null,
  tags: [],
  priority: 100,
  spec_paths: [],
  status: 'unanswered',
  created_at: '2024-01-01T00:00:00Z',
  ...overrides,
});

describe('QuestionList', () => {
  describe('loading state', () => {
    it('shows loading message when loading', () => {
      render(
        <QuestionList
          questions={[]}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={true}
          generating={false}
          loadingSuggestions={false}
        />
      );

      expect(screen.getByText('Loading questions...')).toBeInTheDocument();
    });
  });

  describe('empty state', () => {
    it('shows empty message when no unanswered questions', () => {
      render(
        <QuestionList
          questions={[]}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      expect(screen.getByText(/No unanswered questions/)).toBeInTheDocument();
    });
  });

  describe('question sections', () => {
    it('separates unanswered and answered questions', () => {
      const questions = [
        createQuestion({ id: 'q1', text: 'Unanswered 1', status: 'unanswered' }),
        createQuestion({ id: 'q2', text: 'Answered 1', status: 'answered' }),
        createQuestion({ id: 'q3', text: 'Unanswered 2', status: 'unanswered' }),
      ];

      render(
        <QuestionList
          questions={questions}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      expect(screen.getByText('Unanswered Questions (2)')).toBeInTheDocument();
      expect(screen.getByText('Answered (1)')).toBeInTheDocument();
    });

    it('does not show answered section when no answered questions', () => {
      const questions = [
        createQuestion({ id: 'q1', status: 'unanswered' }),
      ];

      render(
        <QuestionList
          questions={questions}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      expect(screen.queryByText(/Answered \(/)).not.toBeInTheDocument();
    });

    it('sorts unanswered questions by priority descending', () => {
      const questions = [
        createQuestion({ id: 'q1', text: 'Low priority', priority: 10, status: 'unanswered' }),
        createQuestion({ id: 'q2', text: 'High priority', priority: 100, status: 'unanswered' }),
        createQuestion({ id: 'q3', text: 'Medium priority', priority: 50, status: 'unanswered' }),
      ];

      render(
        <QuestionList
          questions={questions}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      const questionTexts = screen.getAllByText(/priority/);
      expect(questionTexts[0].textContent).toBe('High priority');
      expect(questionTexts[1].textContent).toBe('Medium priority');
      expect(questionTexts[2].textContent).toBe('Low priority');
    });
  });

  describe('generate more button', () => {
    it('renders add questions button', () => {
      render(
        <QuestionList
          questions={[]}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      expect(screen.getByRole('button', { name: '+ Add Questions' })).toBeInTheDocument();
    });

    it('calls onGenerateMore when clicked', async () => {
      const user = userEvent.setup();
      const onGenerateMore = vi.fn();

      render(
        <QuestionList
          questions={[]}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={onGenerateMore}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      await user.click(screen.getByRole('button', { name: '+ Add Questions' }));
      expect(onGenerateMore).toHaveBeenCalled();
    });

    it('disables button when disabled prop is true', () => {
      render(
        <QuestionList
          questions={[]}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={true}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      expect(screen.getByRole('button', { name: '+ Add Questions' })).toBeDisabled();
    });
  });

  describe('question cards', () => {
    it('renders question cards for each question', () => {
      const questions = [
        createQuestion({ id: 'q1', text: 'Question 1?' }),
        createQuestion({ id: 'q2', text: 'Question 2?' }),
      ];

      render(
        <QuestionList
          questions={questions}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      expect(screen.getByText('Question 1?')).toBeInTheDocument();
      expect(screen.getByText('Question 2?')).toBeInTheDocument();
    });

    it('passes disabled prop to answered question cards', () => {
      const questions = [
        createQuestion({ id: 'q1', text: 'Answered', status: 'answered' }),
      ];

      render(
        <QuestionList
          questions={questions}
          suggestions={[]}
          onSubmitAnswer={vi.fn()}
          onGenerateMore={vi.fn()}
          disabled={false}
          loading={false}
          generating={false}
          loadingSuggestions={false}
        />
      );

      // Answered questions should not have submit button
      expect(screen.queryByRole('button', { name: 'Submit Answer' })).not.toBeInTheDocument();
    });
  });
});
