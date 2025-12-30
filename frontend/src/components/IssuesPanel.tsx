import { useState } from 'react';
import type { Issue } from '../types';

interface IssuesPanelProps {
  issues: Issue[];
  onIssueClick?: (questionIds: string[]) => void;
}

export function IssuesPanel({ issues, onIssueClick }: IssuesPanelProps) {
  const [isCollapsed, setIsCollapsed] = useState(false);

  if (issues.length === 0) {
    return null;
  }

  const handleIssueClick = (issue: Issue) => {
    if (issue.related_question_ids.length > 0 && onIssueClick) {
      onIssueClick(issue.related_question_ids);
    }
  };

  return (
    <section className={`issues-section ${isCollapsed ? 'collapsed' : ''}`}>
      <button
        className="issues-toggle"
        onClick={() => setIsCollapsed(!isCollapsed)}
      >
        <span className="issues-toggle-icon">{isCollapsed ? '▲' : '▼'}</span>
        <span>Issues ({issues.length})</span>
      </button>
      {!isCollapsed && (
        <div className="issues-panel-content">
          <ul className="issue-list">
            {issues.map((issue) => {
              const hasQuestions = issue.related_question_ids.length > 0;
              return (
                <li
                  key={issue.id}
                  className={`issue ${issue.severity} ${hasQuestions ? 'clickable' : ''}`}
                  onClick={() => handleIssueClick(issue)}
                  role={hasQuestions ? 'button' : undefined}
                  tabIndex={hasQuestions ? 0 : undefined}
                  onKeyDown={(e) => {
                    if (hasQuestions && (e.key === 'Enter' || e.key === ' ')) {
                      e.preventDefault();
                      handleIssueClick(issue);
                    }
                  }}
                >
                  <span className="issue-type">{issue.type}</span>
                  <span className="issue-message">{issue.message}</span>
                  {issue.related_spec_paths.length > 0 && (
                    <span className="issue-paths">
                      {issue.related_spec_paths.map((p) => (
                        <code key={p}>{p}</code>
                      ))}
                    </span>
                  )}
                  {hasQuestions && (
                    <span className="issue-question-hint">
                      {issue.related_question_ids.length} related question{issue.related_question_ids.length > 1 ? 's' : ''}
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </section>
  );
}
