import { useState } from 'react';
import type { SpecSnapshot, CompileStageEvent, CompileStage, ExportFormat } from '../types';
import { api } from '../api/client';

interface SpecViewerProps {
  snapshot: SpecSnapshot | null;
  onCompile: () => void;
  compiling: boolean;
  compileProgress: CompileStageEvent | null;
  disabled: boolean;
  projectId: string | null;
}

const STAGE_LABELS: Record<CompileStage, string> = {
  preparing: 'Preparing',
  compiling: 'Compiling',
  saving: 'Saving',
  validating: 'Validating',
  complete: 'Complete',
};

const STAGES: CompileStage[] = ['preparing', 'compiling', 'saving', 'validating', 'complete'];

function formatTime(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes > 0) {
    return `${minutes}m ${remainingSeconds}s`;
  }
  return `${seconds}s`;
}

export function SpecViewer({
  snapshot,
  onCompile,
  compiling,
  compileProgress,
  disabled,
  projectId,
}: SpecViewerProps) {
  const [exportFormat, setExportFormat] = useState<ExportFormat>('default');

  const currentStageIndex = compileProgress
    ? STAGES.indexOf(compileProgress.stage)
    : -1;

  const getExportUrl = (): string | null => {
    if (!projectId || !snapshot) return null;
    return api.getExportUrl(projectId, snapshot.id, exportFormat);
  };

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
          {projectId && snapshot && (
            <div className="export-controls">
              <select
                className="export-format-select"
                value={exportFormat}
                onChange={(e) => setExportFormat(e.target.value as ExportFormat)}
              >
                <option value="default">AI Coder Pack</option>
                <option value="ralph">Ralph Format</option>
              </select>
              <a
                className="export-button"
                href={getExportUrl() || '#'}
                download
              >
                Export
              </a>
            </div>
          )}
        </div>
      </div>

      {compiling ? (
        <div className="compile-loading">
          <div className="compile-stages">
            {STAGES.filter(s => s !== 'complete').map((stage, index) => {
              const isActive = stage === compileProgress?.stage;
              const isComplete = currentStageIndex > index;
              const isPending = currentStageIndex < index;

              return (
                <div
                  key={stage}
                  className={`compile-stage ${isActive ? 'active' : ''} ${isComplete ? 'complete' : ''} ${isPending ? 'pending' : ''}`}
                >
                  <div className="stage-indicator">
                    {isComplete ? (
                      <span className="stage-check">✓</span>
                    ) : isActive ? (
                      <span className="stage-spinner" />
                    ) : (
                      <span className="stage-number">{index + 1}</span>
                    )}
                  </div>
                  <div className="stage-content">
                    <span className="stage-label">{STAGE_LABELS[stage]}</span>
                    {isActive && compileProgress && (
                      <span className="stage-message">{compileProgress.message}</span>
                    )}
                  </div>
                  <div className="stage-time">
                    {isActive && compileProgress && (
                      <span className="elapsed-time">{formatTime(compileProgress.elapsed_ms)}</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
          {compileProgress && (
            <div className="compile-total-time">
              Total time: {formatTime(compileProgress.total_ms)}
            </div>
          )}
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
