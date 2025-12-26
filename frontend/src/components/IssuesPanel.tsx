import { useState } from 'react';
import type { Issue } from '../types';

interface IssuesPanelProps {
  issues: Issue[];
}

export function IssuesPanel({ issues }: IssuesPanelProps) {
  const [isCollapsed, setIsCollapsed] = useState(false);

  if (issues.length === 0) {
    return null;
  }

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
            {issues.map((issue) => (
              <li key={issue.id} className={`issue ${issue.severity}`}>
                <span className="issue-type">{issue.type}</span>
                <span className="issue-message">{issue.message}</span>
                {issue.related_spec_paths.length > 0 && (
                  <span className="issue-paths">
                    {issue.related_spec_paths.map((p) => (
                      <code key={p}>{p}</code>
                    ))}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
