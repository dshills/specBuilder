import { useState } from 'react';
import type { Question, Suggestion } from '../types';

interface QuestionCardProps {
  question: Question;
  suggestion?: Suggestion;
  onSubmitAnswer: (questionId: string, value: unknown) => Promise<void>;
  disabled: boolean;
}

export function QuestionCard({
  question,
  suggestion,
  onSubmitAnswer,
  disabled,
}: QuestionCardProps) {
  const [answer, setAnswer] = useState<string | string[]>(
    question.type === 'multi' ? [] : ''
  );
  const [submitting, setSubmitting] = useState(false);

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
    } finally {
      setSubmitting(false);
    }
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

  const isAnswered = question.status === 'answered';

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
    <div className={`question-card ${isAnswered ? 'answered' : ''}`}>
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
      </div>

      <p className="question-text">{question.text}</p>

      {!isAnswered && suggestion && (
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

      {!isAnswered && (
        <>
          {renderInput()}
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
            {submitting ? 'Submitting...' : 'Submit Answer'}
          </button>
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
