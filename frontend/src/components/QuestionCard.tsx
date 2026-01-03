import { useState, useEffect, useRef } from 'react';
import type { Question, Suggestion } from '../types';

interface QuestionCardProps {
  question: Question;
  suggestion?: Suggestion;
  highlighted?: boolean;
  onSubmitAnswer: (questionId: string, value: unknown) => Promise<void>;
  onClearHighlight?: () => void;
  disabled: boolean;
}

function formatAnswerValue(value: unknown, type: string): string | string[] {
  if (type === 'multi' && Array.isArray(value)) {
    return value as string[];
  }
  if (typeof value === 'string') {
    return value;
  }
  return JSON.stringify(value, null, 2);
}

export function QuestionCard({
  question,
  suggestion,
  highlighted,
  onSubmitAnswer,
  onClearHighlight,
  disabled,
}: QuestionCardProps) {
  const cardRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (highlighted && cardRef.current) {
      cardRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }, [highlighted]);

  const isAnswered = question.status === 'answered';
  const currentAnswerValue = question.current_answer?.value;

  const [answer, setAnswer] = useState<string | string[]>(
    question.type === 'multi' ? [] : ''
  );
  const [submitting, setSubmitting] = useState(false);
  const [editing, setEditing] = useState(false);

  // Initialize answer when entering edit mode
  useEffect(() => {
    if (editing && currentAnswerValue !== undefined) {
      setAnswer(formatAnswerValue(currentAnswerValue, question.type));
    }
  }, [editing, currentAnswerValue, question.type]);

  const handleSubmit = async () => {
    if (disabled || submitting) return;

    const isEmpty =
      question.type === 'multi'
        ? (answer as string[]).length === 0
        : !answer;

    if (isEmpty) return;

    setSubmitting(true);
    try {
      await onSubmitAnswer(question.id, answer);
      setAnswer(question.type === 'multi' ? [] : '');
      setEditing(false);
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancelEdit = () => {
    setEditing(false);
    setAnswer(question.type === 'multi' ? [] : '');
  };

  const handleStartEdit = () => {
    setEditing(true);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && e.metaKey && question.type === 'freeform') {
      handleSubmit();
    }
  };

  const renderInput = () => {
    switch (question.type) {
      case 'single':
        return (
          <div className="options single">
            {question.options?.map((opt) => (
              <label key={opt} className="option">
                <input
                  type="radio"
                  name={`q-${question.id}`}
                  value={opt}
                  checked={answer === opt}
                  onChange={(e) => setAnswer(e.target.value)}
                  disabled={disabled || submitting}
                />
                <span>{opt}</span>
              </label>
            ))}
          </div>
        );

      case 'multi':
        return (
          <div className="options multi">
            {question.options?.map((opt) => (
              <label key={opt} className="option">
                <input
                  type="checkbox"
                  value={opt}
                  checked={(answer as string[]).includes(opt)}
                  onChange={(e) => {
                    const arr = answer as string[];
                    if (e.target.checked) {
                      setAnswer([...arr, opt]);
                    } else {
                      setAnswer(arr.filter((v) => v !== opt));
                    }
                  }}
                  disabled={disabled || submitting}
                />
                <span>{opt}</span>
              </label>
            ))}
          </div>
        );

      case 'freeform':
      default:
        return (
          <textarea
            className="freeform-input"
            placeholder="Type your answer..."
            value={answer as string}
            onChange={(e) => setAnswer(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled || submitting}
            rows={4}
          />
        );
    }
  };

  const applySuggestion = () => {
    if (!suggestion) return;
    const value = suggestion.suggested_value;
    if (question.type === 'multi' && Array.isArray(value)) {
      setAnswer(value as string[]);
    } else if (typeof value === 'string') {
      setAnswer(value);
    } else {
      // For complex values, stringify for freeform
      setAnswer(JSON.stringify(value, null, 2));
    }
  };

  const formatSuggestedValue = (value: unknown): string => {
    if (Array.isArray(value)) {
      return value.join(', ');
    }
    if (typeof value === 'string') {
      return value.length > 100 ? value.substring(0, 100) + '...' : value;
    }
    return JSON.stringify(value);
  };

  const confidenceColor = (conf: string): string => {
    switch (conf) {
      case 'high': return '#2d7a4f';
      case 'medium': return '#b8860b';
      case 'low': return '#8b4049';
      default: return '#666';
    }
  };

  return (
    <div
      ref={cardRef}
      className={`question-card ${isAnswered ? 'answered' : ''} ${highlighted ? 'highlighted' : ''}`}
    >
      <div className="question-header">
        <span className="question-type">{question.type}</span>
        {question.tags.map((tag) => (
          <span key={tag} className="question-tag">
            {tag}
          </span>
        ))}
        <span className={`question-status ${question.status}`}>
          {question.status}
        </span>
        {highlighted && onClearHighlight && (
          <button
            className="clear-highlight"
            onClick={onClearHighlight}
            title="Dismiss highlight"
          >
            ×
          </button>
        )}
      </div>

      <p className="question-text">{question.text}</p>

      {/* Show suggestion for unanswered questions */}
      {!isAnswered && !editing && suggestion && (
        <div className="suggestion-box">
          <div className="suggestion-header">
            <span className="suggestion-label">AI Suggestion</span>
            <span
              className="suggestion-confidence"
              style={{ color: confidenceColor(suggestion.confidence) }}
            >
              {suggestion.confidence} confidence
            </span>
          </div>
          <div className="suggestion-value">
            {formatSuggestedValue(suggestion.suggested_value)}
          </div>
          {suggestion.reasoning && (
            <div className="suggestion-reasoning">{suggestion.reasoning}</div>
          )}
          <button
            className="apply-suggestion"
            onClick={applySuggestion}
            disabled={disabled || submitting}
          >
            Use Suggestion
          </button>
        </div>
      )}

      {/* Show current answer for answered questions (when not editing) */}
      {isAnswered && !editing && (
        <div className="current-answer">
          <div className="current-answer-header">
            <span className="current-answer-label">Current Answer</span>
            {question.current_answer && (
              <span className="answer-version">v{question.current_answer.version}</span>
            )}
          </div>
          <div className="current-answer-value">
            {currentAnswerValue !== undefined
              ? formatSuggestedValue(currentAnswerValue)
              : '(no value)'}
          </div>
          <button
            className="edit-answer"
            onClick={handleStartEdit}
            disabled={disabled}
          >
            Edit Answer
          </button>
        </div>
      )}

      {/* Show input for unanswered questions or when editing */}
      {(!isAnswered || editing) && (
        <>
          {renderInput()}
          <div className="answer-actions">
            <button
              className="submit-answer"
              onClick={handleSubmit}
              disabled={
                disabled ||
                submitting ||
                (question.type === 'multi'
                  ? (answer as string[]).length === 0
                  : !answer)
              }
            >
              {submitting ? 'Submitting...' : editing ? 'Save Changes' : 'Submit Answer'}
            </button>
            {editing && (
              <button
                className="cancel-edit"
                onClick={handleCancelEdit}
                disabled={submitting}
              >
                Cancel
              </button>
            )}
          </div>
        </>
      )}

      {question.spec_paths.length > 0 && (
        <div className="spec-paths">
          Targets:{' '}
          {question.spec_paths.map((p) => (
            <code key={p}>{p}</code>
          ))}
        </div>
      )}
    </div>
  );
}
