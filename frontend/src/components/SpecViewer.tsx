import type { SpecSnapshot, Issue } from '../types';

interface SpecViewerProps {
  snapshot: SpecSnapshot | null;
  issues: Issue[];
  onCompile: () => Promise<void>;
  compiling: boolean;
  disabled: boolean;
  exportUrl: string | null;
}

export function SpecViewer({
  snapshot,
  issues,
  onCompile,
  compiling,
  disabled,
  exportUrl,
}: SpecViewerProps) {
  return (
    <div className="spec-viewer">
      <div className="spec-header">
        <h2>Compiled Specification</h2>
        <div className="spec-actions">
          <button
            className="compile-button"
            onClick={onCompile}
            disabled={disabled || compiling}
          >
            {compiling ? (
              <>
                <span className="spinner" />
                Compiling...
              </>
            ) : (
              'Compile'
            )}
          </button>
          {exportUrl && (
            <a
              className="export-button"
              href={exportUrl}
              download
            >
              Export Pack
            </a>
          )}
        </div>
      </div>

      {compiling ? (
        <div className="compile-loading">
          <div className="compile-spinner" />
          <p>Compiling specification...</p>
          <p className="compile-hint">This may take 2-3 minutes</p>
        </div>
      ) : snapshot ? (
        <div className="spec-content">
          <div className="spec-meta">
            <span>
              Compiled: {new Date(snapshot.created_at).toLocaleString()}
            </span>
            <span>Model: {snapshot.compiler.model}</span>
          </div>
          <pre className="spec-json">
            {JSON.stringify(snapshot.spec, null, 2)}
          </pre>
        </div>
      ) : (
        <div className="spec-empty">
          <p>No compiled specification yet.</p>
          <p>Answer some questions and click "Compile" to generate a spec.</p>
        </div>
      )}

      {issues.length > 0 && (
        <div className="issues-panel">
          <h4>Issues ({issues.length})</h4>
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
    </div>
  );
}
