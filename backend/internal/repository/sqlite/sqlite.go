package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/specbuilder/backend/internal/domain"
	"github.com/dshills/specbuilder/backend/internal/repository"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// escapeLikePattern escapes special SQL LIKE characters (%, _) to prevent injection.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\") // Escape backslash first
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// New creates a new SQLite repository.
func New(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	repo := &SQLiteRepository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return repo, nil
}

func (r *SQLiteRepository) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		mode TEXT NOT NULL DEFAULT 'advanced',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS questions (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id),
		text TEXT NOT NULL,
		type TEXT NOT NULL,
		options TEXT, -- JSON array or NULL
		tags TEXT NOT NULL DEFAULT '[]', -- JSON array
		priority INTEGER NOT NULL DEFAULT 0,
		spec_paths TEXT NOT NULL DEFAULT '[]', -- JSON array
		status TEXT NOT NULL DEFAULT 'unanswered',
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_questions_project ON questions(project_id);
	CREATE INDEX IF NOT EXISTS idx_questions_status ON questions(status);

	CREATE TABLE IF NOT EXISTS answers (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id),
		question_id TEXT NOT NULL REFERENCES questions(id),
		value TEXT NOT NULL, -- JSON value
		version INTEGER NOT NULL,
		supersedes TEXT REFERENCES answers(id),
		created_at TEXT NOT NULL,
		UNIQUE(question_id, version)
	);
	CREATE INDEX IF NOT EXISTS idx_answers_question ON answers(question_id);
	CREATE INDEX IF NOT EXISTS idx_answers_project ON answers(project_id);

	CREATE TABLE IF NOT EXISTS snapshots (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id),
		spec TEXT NOT NULL, -- JSON object
		created_at TEXT NOT NULL,
		derived_from TEXT NOT NULL, -- JSON object: question_id -> version
		compiler TEXT NOT NULL -- JSON object: CompilerConfig
	);
	CREATE INDEX IF NOT EXISTS idx_snapshots_project ON snapshots(project_id);
	CREATE INDEX IF NOT EXISTS idx_snapshots_created ON snapshots(created_at DESC);

	CREATE TABLE IF NOT EXISTS issues (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id),
		snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
		type TEXT NOT NULL,
		severity TEXT NOT NULL,
		message TEXT NOT NULL,
		related_spec_paths TEXT NOT NULL DEFAULT '[]', -- JSON array
		related_question_ids TEXT NOT NULL DEFAULT '[]', -- JSON array
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_issues_snapshot ON issues(snapshot_id);
	`

	_, err := r.db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: add mode column to existing projects table
	_, _ = r.db.Exec(`ALTER TABLE projects ADD COLUMN mode TEXT NOT NULL DEFAULT 'advanced'`)

	return nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// Projects

func (r *SQLiteRepository) CreateProject(ctx context.Context, p *domain.Project) error {
	mode := string(p.Mode)
	if mode == "" {
		mode = string(domain.ProjectModeAdvanced)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID.String(), p.Name, mode, p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) GetProject(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, mode, created_at, updated_at FROM projects WHERE id = ?`, id.String())
	return scanProject(row)
}

func (r *SQLiteRepository) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, mode, created_at, updated_at FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p, err := scanProjectRow(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *SQLiteRepository) UpdateProject(ctx context.Context, p *domain.Project) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.UpdatedAt.Format(time.RFC3339), p.ID.String())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) DeleteProject(ctx context.Context, id uuid.UUID) error {
	idStr := id.String()
	// Delete in order respecting foreign key constraints
	if _, err := r.db.ExecContext(ctx, `DELETE FROM issues WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM snapshots WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM answers WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM questions WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, idStr)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLiteRepository) GetLatestSnapshotID(ctx context.Context, projectID uuid.UUID) (*uuid.UUID, error) {
	var idStr sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM snapshots WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`,
		projectID.String()).Scan(&idStr)
	if err == sql.ErrNoRows || !idStr.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(idStr.String)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func scanProject(row *sql.Row) (*domain.Project, error) {
	var p domain.Project
	var idStr, modeStr, createdStr, updatedStr string
	if err := row.Scan(&idStr, &p.Name, &modeStr, &createdStr, &updatedStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	var err error
	p.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	p.Mode = domain.ProjectMode(modeStr)
	if p.Mode == "" {
		p.Mode = domain.ProjectModeAdvanced
	}
	p.CreatedAt, err = time.Parse(time.RFC3339, createdStr)
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339, updatedStr)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanProjectRow(rows *sql.Rows) (*domain.Project, error) {
	var p domain.Project
	var idStr, modeStr, createdStr, updatedStr string
	if err := rows.Scan(&idStr, &p.Name, &modeStr, &createdStr, &updatedStr); err != nil {
		return nil, err
	}
	var err error
	p.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	p.Mode = domain.ProjectMode(modeStr)
	if p.Mode == "" {
		p.Mode = domain.ProjectModeAdvanced
	}
	p.CreatedAt, err = time.Parse(time.RFC3339, createdStr)
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339, updatedStr)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Questions

func (r *SQLiteRepository) CreateQuestion(ctx context.Context, q *domain.Question) error {
	optionsJSON, _ := json.Marshal(q.Options)
	tagsJSON, _ := json.Marshal(q.Tags)
	pathsJSON, _ := json.Marshal(q.SpecPaths)

	var optionsVal interface{}
	if q.Options != nil {
		optionsVal = string(optionsJSON)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO questions (id, project_id, text, type, options, tags, priority, spec_paths, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		q.ID.String(), q.ProjectID.String(), q.Text, string(q.Type),
		optionsVal, string(tagsJSON), q.Priority, string(pathsJSON),
		string(q.Status), q.CreatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) GetQuestion(ctx context.Context, id uuid.UUID) (*domain.Question, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, text, type, options, tags, priority, spec_paths, status, created_at
		 FROM questions WHERE id = ?`, id.String())
	return scanQuestion(row)
}

func (r *SQLiteRepository) GetQuestionsByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Question, error) {
	if len(ids) == 0 {
		return []*domain.Question{}, nil
	}

	// Build placeholders and args
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id.String()
	}

	query := `SELECT id, project_id, text, type, options, tags, priority, spec_paths, status, created_at
		      FROM questions WHERE id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []*domain.Question
	for rows.Next() {
		q, err := scanQuestionFromRows(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

func (r *SQLiteRepository) ListQuestions(ctx context.Context, projectID uuid.UUID, status *domain.QuestionStatus, tag *string) ([]*domain.Question, error) {
	query := `SELECT id, project_id, text, type, options, tags, priority, spec_paths, status, created_at
		      FROM questions WHERE project_id = ?`
	args := []interface{}{projectID.String()}

	if status != nil {
		query += ` AND status = ?`
		args = append(args, string(*status))
	}
	if tag != nil {
		query += ` AND tags LIKE ? ESCAPE '\'`
		args = append(args, "%\""+escapeLikePattern(*tag)+"\"%")
	}
	query += ` ORDER BY priority DESC, created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []*domain.Question
	for rows.Next() {
		q, err := scanQuestionFromRows(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

func (r *SQLiteRepository) UpdateQuestionStatus(ctx context.Context, id uuid.UUID, status domain.QuestionStatus) error {
	res, err := r.db.ExecContext(ctx, `UPDATE questions SET status = ? WHERE id = ?`, string(status), id.String())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanQuestion(row *sql.Row) (*domain.Question, error) {
	var q domain.Question
	var idStr, projStr, typeStr, statusStr, createdStr string
	var optionsJSON sql.NullString
	var tagsJSON, pathsJSON string

	if err := row.Scan(&idStr, &projStr, &q.Text, &typeStr, &optionsJSON, &tagsJSON, &q.Priority, &pathsJSON, &statusStr, &createdStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return parseQuestion(idStr, projStr, typeStr, statusStr, createdStr, optionsJSON, tagsJSON, pathsJSON, &q)
}

func scanQuestionFromRows(rows *sql.Rows) (*domain.Question, error) {
	var q domain.Question
	var idStr, projStr, typeStr, statusStr, createdStr string
	var optionsJSON sql.NullString
	var tagsJSON, pathsJSON string

	if err := rows.Scan(&idStr, &projStr, &q.Text, &typeStr, &optionsJSON, &tagsJSON, &q.Priority, &pathsJSON, &statusStr, &createdStr); err != nil {
		return nil, err
	}
	return parseQuestion(idStr, projStr, typeStr, statusStr, createdStr, optionsJSON, tagsJSON, pathsJSON, &q)
}

func parseQuestion(idStr, projStr, typeStr, statusStr, createdStr string, optionsJSON sql.NullString, tagsJSON, pathsJSON string, q *domain.Question) (*domain.Question, error) {
	var err error
	q.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	q.ProjectID, err = uuid.Parse(projStr)
	if err != nil {
		return nil, err
	}
	q.Type = domain.QuestionType(typeStr)
	q.Status = domain.QuestionStatus(statusStr)
	q.CreatedAt, err = time.Parse(time.RFC3339, createdStr)
	if err != nil {
		return nil, err
	}
	if optionsJSON.Valid {
		if err := json.Unmarshal([]byte(optionsJSON.String), &q.Options); err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal([]byte(tagsJSON), &q.Tags); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(pathsJSON), &q.SpecPaths); err != nil {
		return nil, err
	}
	return q, nil
}

// Answers

func (r *SQLiteRepository) CreateAnswer(ctx context.Context, a *domain.Answer) error {
	var supersedesVal interface{}
	if a.Supersedes != nil {
		supersedesVal = a.Supersedes.String()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO answers (id, project_id, question_id, value, version, supersedes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID.String(), a.ProjectID.String(), a.QuestionID.String(),
		string(a.Value), a.Version, supersedesVal, a.CreatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) GetAnswer(ctx context.Context, id uuid.UUID) (*domain.Answer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at FROM answers WHERE id = ?`,
		id.String())
	return scanAnswer(row)
}

func (r *SQLiteRepository) GetLatestAnswer(ctx context.Context, questionID uuid.UUID) (*domain.Answer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at
		 FROM answers WHERE question_id = ? ORDER BY version DESC LIMIT 1`,
		questionID.String())
	return scanAnswer(row)
}

func (r *SQLiteRepository) GetAnswerByVersion(ctx context.Context, questionID uuid.UUID, version int) (*domain.Answer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at
		 FROM answers WHERE question_id = ? AND version = ?`,
		questionID.String(), version)
	return scanAnswer(row)
}

func (r *SQLiteRepository) ListAnswers(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at
		 FROM answers WHERE project_id = ? ORDER BY created_at ASC`,
		projectID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []*domain.Answer
	for rows.Next() {
		a, err := scanAnswerFromRows(rows)
		if err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

func (r *SQLiteRepository) GetLatestAnswersForProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error) {
	// Get the latest version of each answer per question
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.project_id, a.question_id, a.value, a.version, a.supersedes, a.created_at
		FROM answers a
		INNER JOIN (
			SELECT question_id, MAX(version) as max_version
			FROM answers WHERE project_id = ?
			GROUP BY question_id
		) latest ON a.question_id = latest.question_id AND a.version = latest.max_version
		WHERE a.project_id = ?`,
		projectID.String(), projectID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []*domain.Answer
	for rows.Next() {
		a, err := scanAnswerFromRows(rows)
		if err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

func scanAnswer(row *sql.Row) (*domain.Answer, error) {
	var a domain.Answer
	var idStr, projStr, qStr, valueStr, createdStr string
	var supersedesStr sql.NullString

	if err := row.Scan(&idStr, &projStr, &qStr, &valueStr, &a.Version, &supersedesStr, &createdStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return parseAnswer(idStr, projStr, qStr, valueStr, createdStr, supersedesStr, &a)
}

func scanAnswerFromRows(rows *sql.Rows) (*domain.Answer, error) {
	var a domain.Answer
	var idStr, projStr, qStr, valueStr, createdStr string
	var supersedesStr sql.NullString

	if err := rows.Scan(&idStr, &projStr, &qStr, &valueStr, &a.Version, &supersedesStr, &createdStr); err != nil {
		return nil, err
	}
	return parseAnswer(idStr, projStr, qStr, valueStr, createdStr, supersedesStr, &a)
}

func parseAnswer(idStr, projStr, qStr, valueStr, createdStr string, supersedesStr sql.NullString, a *domain.Answer) (*domain.Answer, error) {
	var err error
	a.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	a.ProjectID, err = uuid.Parse(projStr)
	if err != nil {
		return nil, err
	}
	a.QuestionID, err = uuid.Parse(qStr)
	if err != nil {
		return nil, err
	}
	a.Value = json.RawMessage(valueStr)
	a.CreatedAt, err = time.Parse(time.RFC3339, createdStr)
	if err != nil {
		return nil, err
	}
	if supersedesStr.Valid {
		sid, err := uuid.Parse(supersedesStr.String)
		if err != nil {
			return nil, err
		}
		a.Supersedes = &sid
	}
	return a, nil
}

// Snapshots

func (r *SQLiteRepository) CreateSnapshot(ctx context.Context, s *domain.SpecSnapshot) error {
	derivedJSON, _ := json.Marshal(convertDerivedFromToString(s.DerivedFrom))
	compilerJSON, _ := json.Marshal(s.Compiler)

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO snapshots (id, project_id, spec, created_at, derived_from, compiler)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID.String(), s.ProjectID.String(), string(s.Spec),
		s.CreatedAt.Format(time.RFC3339), string(derivedJSON), string(compilerJSON))
	return err
}

func (r *SQLiteRepository) GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.SpecSnapshot, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, spec, created_at, derived_from, compiler FROM snapshots WHERE id = ?`,
		id.String())
	return scanSnapshot(row)
}

func (r *SQLiteRepository) ListSnapshots(ctx context.Context, projectID uuid.UUID, limit int) ([]*domain.SpecSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, spec, created_at, derived_from, compiler
		 FROM snapshots WHERE project_id = ? ORDER BY created_at DESC LIMIT ?`,
		projectID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []*domain.SpecSnapshot
	for rows.Next() {
		s, err := scanSnapshotFromRows(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

func scanSnapshot(row *sql.Row) (*domain.SpecSnapshot, error) {
	var s domain.SpecSnapshot
	var idStr, projStr, specStr, createdStr, derivedStr, compilerStr string

	if err := row.Scan(&idStr, &projStr, &specStr, &createdStr, &derivedStr, &compilerStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return parseSnapshot(idStr, projStr, specStr, createdStr, derivedStr, compilerStr, &s)
}

func scanSnapshotFromRows(rows *sql.Rows) (*domain.SpecSnapshot, error) {
	var s domain.SpecSnapshot
	var idStr, projStr, specStr, createdStr, derivedStr, compilerStr string

	if err := rows.Scan(&idStr, &projStr, &specStr, &createdStr, &derivedStr, &compilerStr); err != nil {
		return nil, err
	}
	return parseSnapshot(idStr, projStr, specStr, createdStr, derivedStr, compilerStr, &s)
}

func parseSnapshot(idStr, projStr, specStr, createdStr, derivedStr, compilerStr string, s *domain.SpecSnapshot) (*domain.SpecSnapshot, error) {
	var err error
	s.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	s.ProjectID, err = uuid.Parse(projStr)
	if err != nil {
		return nil, err
	}
	s.Spec = json.RawMessage(specStr)
	s.CreatedAt, err = time.Parse(time.RFC3339, createdStr)
	if err != nil {
		return nil, err
	}

	var derivedStrMap map[string]int
	if err := json.Unmarshal([]byte(derivedStr), &derivedStrMap); err != nil {
		return nil, err
	}
	s.DerivedFrom = make(map[uuid.UUID]int)
	for k, v := range derivedStrMap {
		uid, err := uuid.Parse(k)
		if err != nil {
			return nil, err
		}
		s.DerivedFrom[uid] = v
	}

	if err := json.Unmarshal([]byte(compilerStr), &s.Compiler); err != nil {
		return nil, err
	}
	return s, nil
}

func convertDerivedFromToString(m map[uuid.UUID]int) map[string]int {
	result := make(map[string]int)
	for k, v := range m {
		result[k.String()] = v
	}
	return result
}

// Issues

func (r *SQLiteRepository) CreateIssue(ctx context.Context, i *domain.Issue) error {
	pathsJSON, _ := json.Marshal(i.RelatedSpecPaths)
	qIDsJSON, _ := json.Marshal(convertUUIDsToStrings(i.RelatedQuestionIDs))

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO issues (id, project_id, snapshot_id, type, severity, message, related_spec_paths, related_question_ids, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID.String(), i.ProjectID.String(), i.SnapshotID.String(),
		string(i.Type), string(i.Severity), i.Message,
		string(pathsJSON), string(qIDsJSON), i.CreatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) ListIssuesForSnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*domain.Issue, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, snapshot_id, type, severity, message, related_spec_paths, related_question_ids, created_at
		 FROM issues WHERE snapshot_id = ?`,
		snapshotID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*domain.Issue
	for rows.Next() {
		i, err := scanIssueFromRows(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

func scanIssueFromRows(rows *sql.Rows) (*domain.Issue, error) {
	var i domain.Issue
	var idStr, projStr, snapStr, typeStr, sevStr, createdStr string
	var pathsJSON, qIDsJSON string

	if err := rows.Scan(&idStr, &projStr, &snapStr, &typeStr, &sevStr, &i.Message, &pathsJSON, &qIDsJSON, &createdStr); err != nil {
		return nil, err
	}

	var err error
	i.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	i.ProjectID, err = uuid.Parse(projStr)
	if err != nil {
		return nil, err
	}
	i.SnapshotID, err = uuid.Parse(snapStr)
	if err != nil {
		return nil, err
	}
	i.Type = domain.IssueType(typeStr)
	i.Severity = domain.IssueSeverity(sevStr)
	i.CreatedAt, err = time.Parse(time.RFC3339, createdStr)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(pathsJSON), &i.RelatedSpecPaths); err != nil {
		return nil, err
	}
	var qIDStrs []string
	if err := json.Unmarshal([]byte(qIDsJSON), &qIDStrs); err != nil {
		return nil, err
	}
	i.RelatedQuestionIDs = make([]uuid.UUID, len(qIDStrs))
	for idx, s := range qIDStrs {
		i.RelatedQuestionIDs[idx], err = uuid.Parse(s)
		if err != nil {
			return nil, err
		}
	}
	return &i, nil
}

func convertUUIDsToStrings(uuids []uuid.UUID) []string {
	result := make([]string, len(uuids))
	for i, u := range uuids {
		result[i] = u.String()
	}
	return result
}

// WithTx executes fn within a transaction.
func (r *SQLiteRepository) WithTx(ctx context.Context, fn func(repository.Repository) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	txRepo := &txRepository{tx: tx, parent: r}
	if err := fn(txRepo); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// txRepository wraps a transaction for Repository operations.
type txRepository struct {
	tx     *sql.Tx
	parent *SQLiteRepository
}

func (t *txRepository) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *txRepository) queryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *txRepository) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

// Implement all Repository methods for txRepository (delegating to transaction)
// For brevity, these mirror the main implementations but use t.tx instead of r.db

func (t *txRepository) CreateProject(ctx context.Context, p *domain.Project) error {
	mode := string(p.Mode)
	if mode == "" {
		mode = string(domain.ProjectModeAdvanced)
	}
	_, err := t.execContext(ctx,
		`INSERT INTO projects (id, name, mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID.String(), p.Name, mode, p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339))
	return err
}

func (t *txRepository) GetProject(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	row := t.queryRowContext(ctx, `SELECT id, name, mode, created_at, updated_at FROM projects WHERE id = ?`, id.String())
	return scanProject(row)
}

func (t *txRepository) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	rows, err := t.queryContext(ctx, `SELECT id, name, mode, created_at, updated_at FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p, err := scanProjectRow(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (t *txRepository) UpdateProject(ctx context.Context, p *domain.Project) error {
	res, err := t.execContext(ctx,
		`UPDATE projects SET name = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.UpdatedAt.Format(time.RFC3339), p.ID.String())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (t *txRepository) DeleteProject(ctx context.Context, id uuid.UUID) error {
	idStr := id.String()
	if _, err := t.execContext(ctx, `DELETE FROM issues WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	if _, err := t.execContext(ctx, `DELETE FROM snapshots WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	if _, err := t.execContext(ctx, `DELETE FROM answers WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	if _, err := t.execContext(ctx, `DELETE FROM questions WHERE project_id = ?`, idStr); err != nil {
		return err
	}
	res, err := t.execContext(ctx, `DELETE FROM projects WHERE id = ?`, idStr)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (t *txRepository) GetLatestSnapshotID(ctx context.Context, projectID uuid.UUID) (*uuid.UUID, error) {
	var idStr sql.NullString
	err := t.queryRowContext(ctx,
		`SELECT id FROM snapshots WHERE project_id = ? ORDER BY created_at DESC LIMIT 1`,
		projectID.String()).Scan(&idStr)
	if err == sql.ErrNoRows || !idStr.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(idStr.String)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (t *txRepository) CreateQuestion(ctx context.Context, q *domain.Question) error {
	optionsJSON, _ := json.Marshal(q.Options)
	tagsJSON, _ := json.Marshal(q.Tags)
	pathsJSON, _ := json.Marshal(q.SpecPaths)

	var optionsVal interface{}
	if q.Options != nil {
		optionsVal = string(optionsJSON)
	}

	_, err := t.execContext(ctx,
		`INSERT INTO questions (id, project_id, text, type, options, tags, priority, spec_paths, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		q.ID.String(), q.ProjectID.String(), q.Text, string(q.Type),
		optionsVal, string(tagsJSON), q.Priority, string(pathsJSON),
		string(q.Status), q.CreatedAt.Format(time.RFC3339))
	return err
}

func (t *txRepository) GetQuestion(ctx context.Context, id uuid.UUID) (*domain.Question, error) {
	row := t.queryRowContext(ctx,
		`SELECT id, project_id, text, type, options, tags, priority, spec_paths, status, created_at
		 FROM questions WHERE id = ?`, id.String())
	return scanQuestion(row)
}

func (t *txRepository) GetQuestionsByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Question, error) {
	if len(ids) == 0 {
		return []*domain.Question{}, nil
	}

	// Build placeholders and args
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id.String()
	}

	query := `SELECT id, project_id, text, type, options, tags, priority, spec_paths, status, created_at
		      FROM questions WHERE id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := t.queryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []*domain.Question
	for rows.Next() {
		q, err := scanQuestionFromRows(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

func (t *txRepository) ListQuestions(ctx context.Context, projectID uuid.UUID, status *domain.QuestionStatus, tag *string) ([]*domain.Question, error) {
	query := `SELECT id, project_id, text, type, options, tags, priority, spec_paths, status, created_at
		      FROM questions WHERE project_id = ?`
	args := []interface{}{projectID.String()}

	if status != nil {
		query += ` AND status = ?`
		args = append(args, string(*status))
	}
	if tag != nil {
		query += ` AND tags LIKE ? ESCAPE '\'`
		args = append(args, "%\""+escapeLikePattern(*tag)+"\"%")
	}
	query += ` ORDER BY priority DESC, created_at ASC`

	rows, err := t.queryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []*domain.Question
	for rows.Next() {
		q, err := scanQuestionFromRows(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

func (t *txRepository) UpdateQuestionStatus(ctx context.Context, id uuid.UUID, status domain.QuestionStatus) error {
	res, err := t.execContext(ctx, `UPDATE questions SET status = ? WHERE id = ?`, string(status), id.String())
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (t *txRepository) CreateAnswer(ctx context.Context, a *domain.Answer) error {
	var supersedesVal interface{}
	if a.Supersedes != nil {
		supersedesVal = a.Supersedes.String()
	}
	_, err := t.execContext(ctx,
		`INSERT INTO answers (id, project_id, question_id, value, version, supersedes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID.String(), a.ProjectID.String(), a.QuestionID.String(),
		string(a.Value), a.Version, supersedesVal, a.CreatedAt.Format(time.RFC3339))
	return err
}

func (t *txRepository) GetAnswer(ctx context.Context, id uuid.UUID) (*domain.Answer, error) {
	row := t.queryRowContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at FROM answers WHERE id = ?`,
		id.String())
	return scanAnswer(row)
}

func (t *txRepository) GetLatestAnswer(ctx context.Context, questionID uuid.UUID) (*domain.Answer, error) {
	row := t.queryRowContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at
		 FROM answers WHERE question_id = ? ORDER BY version DESC LIMIT 1`,
		questionID.String())
	return scanAnswer(row)
}

func (t *txRepository) GetAnswerByVersion(ctx context.Context, questionID uuid.UUID, version int) (*domain.Answer, error) {
	row := t.queryRowContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at
		 FROM answers WHERE question_id = ? AND version = ?`,
		questionID.String(), version)
	return scanAnswer(row)
}

func (t *txRepository) ListAnswers(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error) {
	rows, err := t.queryContext(ctx,
		`SELECT id, project_id, question_id, value, version, supersedes, created_at
		 FROM answers WHERE project_id = ? ORDER BY created_at ASC`,
		projectID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []*domain.Answer
	for rows.Next() {
		a, err := scanAnswerFromRows(rows)
		if err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

func (t *txRepository) GetLatestAnswersForProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Answer, error) {
	rows, err := t.queryContext(ctx, `
		SELECT a.id, a.project_id, a.question_id, a.value, a.version, a.supersedes, a.created_at
		FROM answers a
		INNER JOIN (
			SELECT question_id, MAX(version) as max_version
			FROM answers WHERE project_id = ?
			GROUP BY question_id
		) latest ON a.question_id = latest.question_id AND a.version = latest.max_version
		WHERE a.project_id = ?`,
		projectID.String(), projectID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []*domain.Answer
	for rows.Next() {
		a, err := scanAnswerFromRows(rows)
		if err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, rows.Err()
}

func (t *txRepository) CreateSnapshot(ctx context.Context, s *domain.SpecSnapshot) error {
	derivedJSON, _ := json.Marshal(convertDerivedFromToString(s.DerivedFrom))
	compilerJSON, _ := json.Marshal(s.Compiler)

	_, err := t.execContext(ctx,
		`INSERT INTO snapshots (id, project_id, spec, created_at, derived_from, compiler)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID.String(), s.ProjectID.String(), string(s.Spec),
		s.CreatedAt.Format(time.RFC3339), string(derivedJSON), string(compilerJSON))
	return err
}

func (t *txRepository) GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.SpecSnapshot, error) {
	row := t.queryRowContext(ctx,
		`SELECT id, project_id, spec, created_at, derived_from, compiler FROM snapshots WHERE id = ?`,
		id.String())
	return scanSnapshot(row)
}

func (t *txRepository) ListSnapshots(ctx context.Context, projectID uuid.UUID, limit int) ([]*domain.SpecSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := t.queryContext(ctx,
		`SELECT id, project_id, spec, created_at, derived_from, compiler
		 FROM snapshots WHERE project_id = ? ORDER BY created_at DESC LIMIT ?`,
		projectID.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []*domain.SpecSnapshot
	for rows.Next() {
		s, err := scanSnapshotFromRows(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, rows.Err()
}

func (t *txRepository) CreateIssue(ctx context.Context, i *domain.Issue) error {
	pathsJSON, _ := json.Marshal(i.RelatedSpecPaths)
	qIDsJSON, _ := json.Marshal(convertUUIDsToStrings(i.RelatedQuestionIDs))

	_, err := t.execContext(ctx,
		`INSERT INTO issues (id, project_id, snapshot_id, type, severity, message, related_spec_paths, related_question_ids, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID.String(), i.ProjectID.String(), i.SnapshotID.String(),
		string(i.Type), string(i.Severity), i.Message,
		string(pathsJSON), string(qIDsJSON), i.CreatedAt.Format(time.RFC3339))
	return err
}

func (t *txRepository) ListIssuesForSnapshot(ctx context.Context, snapshotID uuid.UUID) ([]*domain.Issue, error) {
	rows, err := t.queryContext(ctx,
		`SELECT id, project_id, snapshot_id, type, severity, message, related_spec_paths, related_question_ids, created_at
		 FROM issues WHERE snapshot_id = ?`,
		snapshotID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*domain.Issue
	for rows.Next() {
		i, err := scanIssueFromRows(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

func (t *txRepository) WithTx(ctx context.Context, fn func(repository.Repository) error) error {
	// Already in a transaction, just execute
	return fn(t)
}

func (t *txRepository) Close() error {
	return nil // No-op for transaction wrapper
}

// Ensure implementations satisfy the interface
var _ repository.Repository = (*SQLiteRepository)(nil)
var _ repository.Repository = (*txRepository)(nil)
