# Spec Builder Code Review

**Date**: 2026-01-14
**Reviewer**: Claude Code
**Scope**: Full codebase review (Go backend + React/TypeScript frontend)

---

## Executive Summary

Spec Builder is a well-architected application with clear separation of concerns, good TypeScript usage, and reasonable test coverage. The codebase demonstrates solid fundamentals but has several areas requiring attention before production deployment.

| Severity | Backend | Frontend | Total |
|----------|---------|----------|-------|
| Critical | 2 | 3 | 5 |
| High | 8 | 4 | 12 |
| Medium | 6 | 7 | 13 |
| Low | 4 | 9 | 13 |

---

## Critical Issues

### 1. SQL Injection Vulnerability via LIKE Pattern
**Location**: `backend/internal/repository/sqlite/sqlite.go:305-306, 900-901`

The tag filter uses string concatenation with LIKE, allowing SQL injection via special characters (`%`, `_`).

```go
if tag != nil {
    query += ` AND tags LIKE ?`
    args = append(args, "%\""+*tag+"\"%")
}
```

**Risk**: An attacker could manipulate the LIKE pattern to expose unintended data.

**Fix**: Escape special LIKE characters or use JSON functions (`json_each`) for proper JSON array searching.

---

### 2. CORS Allows All Origins
**Location**: `backend/internal/api/middleware.go:11-12`

```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

**Risk**: Any website can make requests to the API, enabling potential CSRF attacks.

**Fix**: Configure allowed origins explicitly via environment variable. Only allow trusted origins in production.

---

### 3. SSE Connection Memory Leak on Component Unmount
**Location**: `frontend/src/App.tsx:59-62`

The SSE cleanup refs are set but never cleaned up on unmount. Active connections continue updating state on unmounted components.

**Risk**: Memory leaks, React warnings, degraded performance in long sessions.

**Fix**: Add cleanup effect:
```typescript
useEffect(() => {
  return () => {
    compileCleanupRef.current?.();
    generateCleanupRef.current?.();
    suggestionsCleanupRef.current?.();
  };
}, []);
```

---

### 4. Race Condition in loadProject Dependencies
**Location**: `frontend/src/App.tsx:98-106`

The `useEffect` depends on `loadProjects` but calls `loadProject` which is not memoized or in the dependency array, causing stale closures.

**Fix**: Wrap `loadProject` in `useCallback` and add to dependency array.

---

### 5. API Client Test-Implementation Mismatch
**Location**: `frontend/src/api/client.ts:49` vs `client.test.ts:361`

Test expects `'API request failed'` but implementation throws `'Server returned error ${status}'`.

**Fix**: Align test expectation with actual error message or update error message format.

---

## High Priority Issues

### Backend

#### 6. N+1 Query Problem (3 locations)
**Locations**:
- `backend/internal/api/handlers.go:614-628` (Compile)
- `backend/internal/api/handlers.go:779-795` (CompileStream)
- `backend/internal/api/handlers.go` (NextQuestionsStream)

Each answer triggers an individual `GetQuestion` call inside loops.

```go
for _, a := range answers {
    q, err := h.repo.GetQuestion(r.Context(), a.QuestionID)  // N queries!
    // ...
}
```

**Fix**: Add `GetQuestionsByIDs(ctx, ids []uuid.UUID)` method to batch queries.

---

#### 7. Silently Ignored Errors (4 locations)
**Locations**:
- `handlers.go:179-181` - `seedQuestions` error
- `handlers.go:448-450` - `UpdateQuestionStatus` error
- `handlers.go:673-682` - `CreateIssue` errors in loop
- `handlers.go:687, 846-848` - `UpdateProject`, `CreateIssue` errors

**Fix**: At minimum, log errors. Consider returning errors for critical operations.

---

#### 8. Unused Variable in Compiler
**Location**: `backend/internal/compiler/compiler.go:199`

`projectJSON` is marshaled but never used:
```go
projectJSON, _ := json.Marshal(project)
_ = projectJSON  // Unused
```

---

#### 9. Potential Panic from Empty Response Bodies
**Locations**:
- `backend/internal/llm/anthropic.go:135, 140, 210`
- `backend/internal/llm/openai.go:179`
- `backend/internal/llm/gemini.go:151, 224`

Truncating error messages without explicit empty-body checks.

---

### Frontend

#### 10. Missing useCallback for loadProject and loadQuestions
**Location**: `frontend/src/App.tsx:108-148`

These async functions are not memoized but called from effects and callbacks, causing unnecessary re-renders.

---

#### 11. deleteProject Bypasses request() Helper
**Location**: `frontend/src/api/client.ts:71-80`

Directly uses `fetch` without timeout handling or abort controller:
```typescript
async deleteProject(projectId: string): Promise<void> {
  const response = await fetch(`${API_BASE}/projects/${projectId}`, {
    method: 'DELETE',
    // No timeout, no abort controller
  });
}
```

---

#### 12. Unchecked Array Access in ModelSelector
**Location**: `frontend/src/components/ModelSelector.tsx:34`

```typescript
const [provider, model] = e.target.value.split(':') as [Provider, string];
// provider could be partial, model could be undefined
```

---

#### 13. Potential XSS in Question Content Rendering
**Location**: `frontend/src/components/QuestionCard.tsx:210, 225, 252`

While React escapes by default, JSON values rendered via `JSON.stringify` in `<pre>` tags could cause display issues with special characters.

---

## Medium Priority Issues

### Backend

#### 14. Massive Code Duplication (SQLiteRepository vs txRepository)
**Location**: `backend/internal/repository/sqlite/sqlite.go:753-1102`

~350 lines of duplicated code between the two repository implementations.

**Fix**: Refactor to use a shared `dbExecutor` interface.

---

#### 15. Inconsistent Error Handling in JSON Marshal
**Locations**: `backend/internal/compiler/planner.go:86, 91-93, 175-176, 181-182, 269, 295`

Multiple `json.Marshal` calls ignore errors using `_`.

---

#### 16. No Input Validation on Question Type
**Location**: `backend/internal/api/handlers.go:970-971`

LLM-generated question types are cast without validation:
```go
Type: domain.QuestionType(aq.Type),  // Could be invalid
```

---

#### 17. Deprecated Function Usage (strings.Title)
**Location**: `backend/internal/export/export.go:143, 221-222, 1033-1035`

`strings.Title()` is deprecated since Go 1.18.

---

#### 18. Hard-coded HTTP Timeouts
**Locations**: All LLM clients use hard-coded 300-second timeout.

---

#### 19. Model List Functions Create New HTTP Clients
**Locations**: `anthropic.go:192`, `openai.go:162`, `gemini.go:213`

---

### Frontend

#### 20. Inline Styles Ignore Theme System
**Location**: `frontend/src/components/ProjectSelector.tsx:58-60, 70-72`

Hardcoded colors won't respond to light/dark theme changes.

---

#### 21. Missing Error Boundary
**Location**: `frontend/src/App.tsx`

No Error Boundary wraps the application for graceful error handling.

---

#### 22. Prop Drilling (12 props to QuestionList)
**Location**: `frontend/src/App.tsx:390-404`

Consider React Context for shared state.

---

#### 23. Large Spec JSON Rendering Performance
**Location**: `frontend/src/components/SpecViewer.tsx:146-148`

Entire spec JSON stringified and rendered could cause performance issues.

---

#### 24. Sort Mutates Array During Render
**Location**: `frontend/src/components/QuestionList.tsx:77-78`

```typescript
unanswered.sort(...)  // Mutates during render
```

**Fix**: Use `[...unanswered].sort(...)`

---

#### 25. Missing Debounce on Answer Submission
**Location**: `frontend/src/components/QuestionCard.tsx:55-73`

Rapid clicks could trigger multiple API calls.

---

#### 26. Inconsistent Sorting (Answered vs Unanswered)
**Location**: `frontend/src/components/QuestionList.tsx:96-106`

Unanswered questions sorted by priority, answered questions in original order.

---

## Low Priority Issues

### Backend

- **Magic numbers** throughout handlers and compiler
- **Inconsistent nil vs empty slice** handling
- **Missing context-based cancellation** for file operations
- **Long functions** in export.go (100+ lines each)

### Frontend

- **Type safety**: `Answer.value` typed as `unknown`
- **Missing accessibility**: textarea lacks `aria-label`
- **Inconsistent error clearing**: some handlers clear, others don't
- **Test coverage gaps**: Dashboard, IssuesPanel, ModelSelector, ThemeToggle untested
- **Magic numbers** in API client timeouts
- **Mixed naming patterns**: `handle*` vs `load*` vs bare names
- **Duplicate mode selector** in Dashboard and ProjectSelector

---

## Positive Patterns

### Backend
- Good error wrapping with `fmt.Errorf("context: %w", err)`
- Clean interface design (`repository.Repository`)
- Proper resource cleanup (`defer rows.Close()`)
- Type-safe enums for statuses and types
- Clear separation of concerns
- Consistent context propagation
- Well-structured SSE implementation

### Frontend
- Good TypeScript usage with strict types
- Reasonable component organization
- Adequate test coverage for core components
- Clean separation between API client and components
- Proper use of React hooks patterns

---

## Recommendations

### Immediate (Before Production)
1. Fix SQL injection in tag filtering
2. Configure CORS for production origins
3. Add SSE cleanup effect to prevent memory leaks
4. Add logging for silently ignored errors

### Short-term
5. Fix N+1 query patterns with batch queries
6. Add Error Boundary component
7. Wrap async functions in useCallback
8. Fix deleteProject to use request() helper

### Medium-term
9. Refactor repository duplication
10. Consider React Context for shared state
11. Add input validation for LLM-generated data
12. Add tests for untested components

### Long-term
13. Extract magic numbers to constants
14. Implement virtualization for large JSON rendering
15. Standardize naming conventions
16. Replace deprecated strings.Title

---

## Files Changed Summary

No files were modified during this review. This document contains findings only.
