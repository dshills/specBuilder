import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api } from './client';

describe('ApiClient', () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    mockFetch.mockReset();
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const mockResponse = (data: unknown, ok = true, status = 200) => {
    mockFetch.mockResolvedValueOnce({
      ok,
      status,
      statusText: ok ? 'OK' : 'Error',
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: async () => data,
    } as Response);
  };

  describe('createProject', () => {
    it('sends POST request with project name and mode', async () => {
      mockResponse({ project_id: 'p123' });

      await api.createProject('My Project', 'basic');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ name: 'My Project', mode: 'basic' }),
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        })
      );
    });

    it('defaults to advanced mode', async () => {
      mockResponse({ project_id: 'p123' });

      await api.createProject('My Project');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects'),
        expect.objectContaining({
          body: JSON.stringify({ name: 'My Project', mode: 'advanced' }),
        })
      );
    });

    it('returns project_id from response', async () => {
      mockResponse({ project_id: 'p123' });

      const result = await api.createProject('My Project');

      expect(result.project_id).toBe('p123');
    });

    it('throws error on API failure', async () => {
      mockResponse({ error: 'validation_error', message: 'Name is required' }, false, 400);

      await expect(api.createProject('')).rejects.toThrow('Name is required');
    });
  });

  describe('getProject', () => {
    it('sends GET request with project ID', async () => {
      mockResponse({
        project: { id: 'p1', name: 'Test', created_at: '', updated_at: '' },
        latest_snapshot_id: null,
      });

      await api.getProject('p1');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects/p1'),
        expect.objectContaining({
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        })
      );
    });

    it('returns project data', async () => {
      mockResponse({
        project: { id: 'p1', name: 'Test Project', created_at: '', updated_at: '' },
        latest_snapshot_id: 's1',
      });

      const result = await api.getProject('p1');

      expect(result.project.name).toBe('Test Project');
      expect(result.latest_snapshot_id).toBe('s1');
    });
  });

  describe('listQuestions', () => {
    it('sends GET request to questions endpoint', async () => {
      mockResponse({ questions: [] });

      await api.listQuestions('p1');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects/p1/questions'),
        expect.anything()
      );
    });

    it('adds status query param when provided', async () => {
      mockResponse({ questions: [] });

      await api.listQuestions('p1', 'unanswered');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('status=unanswered'),
        expect.anything()
      );
    });

    it('adds tag query param when provided', async () => {
      mockResponse({ questions: [] });

      await api.listQuestions('p1', undefined, 'seed');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('tag=seed'),
        expect.anything()
      );
    });

    it('adds both params when provided', async () => {
      mockResponse({ questions: [] });

      await api.listQuestions('p1', 'answered', 'core');

      const url = mockFetch.mock.calls[0][0];
      expect(url).toContain('status=answered');
      expect(url).toContain('tag=core');
    });

    it('returns questions array', async () => {
      mockResponse({
        questions: [
          { id: 'q1', text: 'Question 1?' },
          { id: 'q2', text: 'Question 2?' },
        ],
      });

      const result = await api.listQuestions('p1');

      expect(result.questions).toHaveLength(2);
      expect(result.questions[0].text).toBe('Question 1?');
    });
  });

  describe('generateNextQuestions', () => {
    it('sends POST request with count', async () => {
      mockResponse({ questions: [] });

      await api.generateNextQuestions('p1', 3);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects/p1/next-questions'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ count: 3 }),
        })
      );
    });

    it('uses default count of 5', async () => {
      mockResponse({ questions: [] });

      await api.generateNextQuestions('p1');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({
          body: JSON.stringify({ count: 5 }),
        })
      );
    });
  });

  describe('submitAnswer', () => {
    it('sends POST request with answer data', async () => {
      mockResponse({ answer_id: 'a1', snapshot_id: null, issues: [] });

      await api.submitAnswer('p1', 'q1', 'My answer', false);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects/p1/answers'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            question_id: 'q1',
            value: 'My answer',
            compile: false,
          }),
        })
      );
    });

    it('handles array values for multi-choice', async () => {
      mockResponse({ answer_id: 'a1', snapshot_id: null, issues: [] });

      await api.submitAnswer('p1', 'q1', ['Option A', 'Option B'], false);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({
          body: JSON.stringify({
            question_id: 'q1',
            value: ['Option A', 'Option B'],
            compile: false,
          }),
        })
      );
    });

    it('returns answer response', async () => {
      mockResponse({ answer_id: 'a123', snapshot_id: 's1', issues: [] });

      const result = await api.submitAnswer('p1', 'q1', 'Answer', true);

      expect(result.answer_id).toBe('a123');
      expect(result.snapshot_id).toBe('s1');
    });
  });

  describe('compile', () => {
    it('sends POST request with latest_answers mode', async () => {
      mockResponse({ snapshot_id: 's1', issues: [] });

      await api.compile('p1');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects/p1/compile'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ mode: 'latest_answers' }),
        })
      );
    });

    it('returns compile response', async () => {
      mockResponse({
        snapshot_id: 's123',
        issues: [{ id: 'i1', message: 'Warning' }],
      });

      const result = await api.compile('p1');

      expect(result.snapshot_id).toBe('s123');
      expect(result.issues).toHaveLength(1);
    });
  });

  describe('listSnapshots', () => {
    it('sends GET request with limit', async () => {
      mockResponse({ snapshots: [] });

      await api.listSnapshots('p1', 10);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects/p1/snapshots?limit=10'),
        expect.anything()
      );
    });

    it('uses default limit of 50', async () => {
      mockResponse({ snapshots: [] });

      await api.listSnapshots('p1');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('limit=50'),
        expect.anything()
      );
    });

    it('returns snapshots array', async () => {
      mockResponse({
        snapshots: [
          { id: 's1', spec: {} },
          { id: 's2', spec: {} },
        ],
      });

      const result = await api.listSnapshots('p1');

      expect(result.snapshots).toHaveLength(2);
    });
  });

  describe('getSnapshot', () => {
    it('sends GET request with snapshot ID', async () => {
      mockResponse({
        snapshot: { id: 's1', spec: {} },
        issues: [],
      });

      await api.getSnapshot('p1', 's1');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/projects/p1/snapshots/s1'),
        expect.anything()
      );
    });

    it('returns snapshot with issues', async () => {
      mockResponse({
        snapshot: { id: 's1', spec: { product: { name: 'Test' } } },
        issues: [{ id: 'i1', message: 'Warning' }],
      });

      const result = await api.getSnapshot('p1', 's1');

      expect(result.snapshot.id).toBe('s1');
      expect(result.issues).toHaveLength(1);
    });
  });

  describe('getExportUrl', () => {
    it('returns export URL without snapshot ID', () => {
      const url = api.getExportUrl('p1');

      expect(url).toContain('/projects/p1/export');
      expect(url).not.toContain('snapshot_id');
    });

    it('returns export URL with snapshot ID', () => {
      const url = api.getExportUrl('p1', 's123');

      expect(url).toContain('/projects/p1/export');
      expect(url).toContain('snapshot_id=s123');
    });
  });

  describe('error handling', () => {
    it('throws error with API message', async () => {
      mockResponse(
        { error: 'not_found', message: 'Project not found' },
        false,
        404
      );

      await expect(api.getProject('nonexistent')).rejects.toThrow('Project not found');
    });

    it('throws generic error when no message', async () => {
      mockResponse({ error: 'internal_error' }, false, 500);

      await expect(api.getProject('p1')).rejects.toThrow('Server returned error 500');
    });
  });
});
