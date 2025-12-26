import type { Question, Suggestion } from '../types';
import { QuestionCard } from './QuestionCard';

interface QuestionListProps {
  questions: Question[];
  suggestions: Suggestion[];
  onSubmitAnswer: (questionId: string, value: unknown) => Promise<void>;
  onGenerateMore: () => Promise<void>;
  disabled: boolean;
  loading: boolean;
  loadingSuggestions: boolean;
}

export function QuestionList({
  questions,
  suggestions,
  onSubmitAnswer,
  onGenerateMore,
  disabled,
  loading,
  loadingSuggestions,
}: QuestionListProps) {
  // Create a map for quick suggestion lookup by question ID
  const suggestionMap = new Map(
    suggestions.map((s) => [s.question_id, s])
  );
  const unanswered = questions.filter((q) => q.status === 'unanswered');
  const answered = questions.filter((q) => q.status === 'answered');

  if (loading) {
    return <div className="question-list loading">Loading questions...</div>;
  }

  return (
    <div className="question-list">
      <div className="question-section">
        <h3>
          Unanswered Questions ({unanswered.length})
          {loadingSuggestions && (
            <span className="suggestions-loading" title="Generating suggested answers...">
              🔄
            </span>
          )}
          <button
            className="generate-more"
            onClick={onGenerateMore}
            disabled={disabled}
            title="Use AI to generate more questions based on your answers and spec gaps"
          >
            + Add Questions
          </button>
        </h3>
        {unanswered.length === 0 ? (
          <p className="empty">
            No unanswered questions. Click "Add Questions" to continue.
          </p>
        ) : (
          unanswered
            .sort((a, b) => b.priority - a.priority)
            .map((q) => (
              <QuestionCard
                key={q.id}
                question={q}
                suggestion={suggestionMap.get(q.id)}
                onSubmitAnswer={onSubmitAnswer}
                disabled={disabled}
              />
            ))
        )}
      </div>

      {answered.length > 0 && (
        <div className="question-section answered-section">
          <h3>Answered ({answered.length})</h3>
          {answered.map((q) => (
            <QuestionCard
              key={q.id}
              question={q}
              onSubmitAnswer={onSubmitAnswer}
              disabled={true}
            />
          ))}
        </div>
      )}
    </div>
  );
}
