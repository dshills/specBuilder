import type { SpecSnapshot } from '../types';

interface SpecViewerProps {
  snapshot: SpecSnapshot | null;
  onCompile: () => Promise<void>;
  compiling: boolean;
  disabled: boolean;
  exportUrl: string | null;
}

export function SpecViewer({
  snapshot,
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
    </div>
  );
}
