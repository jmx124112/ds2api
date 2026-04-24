package chathistory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	FileVersion      = 3
	DisabledLimit    = 0
	DefaultLimit     = 20
	MaxLimit         = 50
	defaultPreviewAt = 160
)

var allowedLimits = map[int]struct{}{
	DisabledLimit: {},
	10:            {},
	20:            {},
	50:            {},
}

var ErrDisabled = errors.New("chat history disabled")

type Stats struct {
	TotalCalls   int64 `json:"total_calls"`
	SuccessCalls int64 `json:"success_calls"`
	FailedCalls  int64 `json:"failed_calls"`
}

type Entry struct {
	ID               string         `json:"id"`
	Revision         int64          `json:"revision"`
	CreatedAt        int64          `json:"created_at"`
	UpdatedAt        int64          `json:"updated_at"`
	CompletedAt      int64          `json:"completed_at,omitempty"`
	Status           string         `json:"status"`
	CallerID         string         `json:"caller_id,omitempty"`
	AccountID        string         `json:"account_id,omitempty"`
	Model            string         `json:"model,omitempty"`
	Stream           bool           `json:"stream"`
	UserInput        string         `json:"user_input,omitempty"`
	Messages         []Message      `json:"messages,omitempty"`
	HistoryText      string         `json:"history_text,omitempty"`
	FinalPrompt      string         `json:"final_prompt,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	Content          string         `json:"content,omitempty"`
	Error            string         `json:"error,omitempty"`
	StatusCode       int            `json:"status_code,omitempty"`
	ElapsedMs        int64          `json:"elapsed_ms,omitempty"`
	FinishReason     string         `json:"finish_reason,omitempty"`
	Usage            map[string]any `json:"usage,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SummaryEntry struct {
	ID             string `json:"id"`
	Revision       int64  `json:"revision"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	CompletedAt    int64  `json:"completed_at,omitempty"`
	Status         string `json:"status"`
	CallerID       string `json:"caller_id,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
	Model          string `json:"model,omitempty"`
	Stream         bool   `json:"stream"`
	UserInput      string `json:"user_input,omitempty"`
	Preview        string `json:"preview,omitempty"`
	StatusCode     int    `json:"status_code,omitempty"`
	ElapsedMs      int64  `json:"elapsed_ms,omitempty"`
	FinishReason   string `json:"finish_reason,omitempty"`
	DetailRevision int64  `json:"detail_revision"`
}

type File struct {
	Version  int            `json:"version"`
	Limit    int            `json:"limit"`
	Revision int64          `json:"revision"`
	Stats    Stats          `json:"stats"`
	Items    []SummaryEntry `json:"items"`
}

type StartParams struct {
	CallerID    string
	AccountID   string
	Model       string
	Stream      bool
	UserInput   string
	Messages    []Message
	HistoryText string
	FinalPrompt string
}

type UpdateParams struct {
	Status           string
	ReasoningContent string
	Content          string
	Error            string
	StatusCode       int
	ElapsedMs        int64
	FinishReason     string
	Usage            map[string]any
	Completed        bool
}

type stateRow struct {
	Version      int
	Limit        int
	Revision     int64
	TotalCalls   int64
	SuccessCalls int64
	FailedCalls  int64
}

type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Store struct {
	mu   sync.Mutex
	path string
	db   *sql.DB
	err  error
}

func New(path string) *Store {
	s := &Store{path: strings.TrimSpace(path)}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = s.initLocked()
	return s
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) DetailDir() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.path) + ".d"
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close chat history sqlite: %w", err)
	}
	s.db = nil
	return nil
}

func (s *Store) Err() error {
	if s == nil {
		return errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Store) Snapshot() (File, error) {
	if s == nil {
		return File{}, errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return File{}, s.err
	}
	return s.snapshotLocked()
}

func (s *Store) Enabled() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return false
	}
	state, err := s.loadStateLocked(s.db)
	if err != nil {
		return false
	}
	return state.Limit != DisabledLimit
}

func (s *Store) Get(id string) (Entry, error) {
	if s == nil {
		return Entry{}, errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return Entry{}, s.err
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return Entry{}, errors.New("history id is required")
	}
	item, err := s.getEntryLocked(s.db, target)
	if err != nil {
		return Entry{}, err
	}
	return cloneEntry(item), nil
}

func (s *Store) Start(params StartParams) (Entry, error) {
	if s == nil {
		return Entry{}, errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return Entry{}, s.err
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Entry{}, fmt.Errorf("begin chat history start tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	state, err := s.loadStateLocked(tx)
	if err != nil {
		return Entry{}, err
	}
	if state.Limit == DisabledLimit {
		return Entry{}, ErrDisabled
	}

	now := time.Now().UnixMilli()
	revision := nextRevision(state.Revision)
	entry := Entry{
		ID:          "chat_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Revision:    revision,
		CreatedAt:   now,
		UpdatedAt:   now,
		Status:      "streaming",
		CallerID:    strings.TrimSpace(params.CallerID),
		AccountID:   strings.TrimSpace(params.AccountID),
		Model:       strings.TrimSpace(params.Model),
		Stream:      params.Stream,
		UserInput:   strings.TrimSpace(params.UserInput),
		Messages:    cloneMessages(params.Messages),
		HistoryText: params.HistoryText,
		FinalPrompt: strings.TrimSpace(params.FinalPrompt),
	}
	if err := s.insertEntryLocked(tx, entry); err != nil {
		return Entry{}, err
	}
	if state.Limit > DisabledLimit {
		if err := s.trimEntriesToLimitLocked(tx, state.Limit); err != nil {
			return Entry{}, err
		}
	}
	state.Revision = revision
	if err := s.saveStateLocked(tx, state); err != nil {
		return Entry{}, err
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, fmt.Errorf("commit chat history start tx: %w", err)
	}
	committed = true
	return cloneEntry(entry), nil
}

func (s *Store) Update(id string, params UpdateParams) (Entry, error) {
	if s == nil {
		return Entry{}, errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return Entry{}, s.err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return Entry{}, errors.New("history id is required")
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return Entry{}, fmt.Errorf("begin chat history update tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	state, err := s.loadStateLocked(tx)
	if err != nil {
		return Entry{}, err
	}
	item, err := s.getEntryLocked(tx, target)
	if err != nil {
		return Entry{}, err
	}

	now := time.Now().UnixMilli()
	item.Revision = nextRevision(state.Revision)
	item.UpdatedAt = now
	if params.Status != "" {
		item.Status = params.Status
	}
	item.ReasoningContent = params.ReasoningContent
	item.Content = params.Content
	item.Error = strings.TrimSpace(params.Error)
	item.StatusCode = params.StatusCode
	item.ElapsedMs = params.ElapsedMs
	item.FinishReason = strings.TrimSpace(params.FinishReason)
	if params.Usage != nil {
		item.Usage = cloneMap(params.Usage)
	}
	if params.Completed {
		item.CompletedAt = now
	}
	if err := s.updateEntryLocked(tx, item); err != nil {
		return Entry{}, err
	}
	state.Revision = item.Revision
	if err := s.saveStateLocked(tx, state); err != nil {
		return Entry{}, err
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, fmt.Errorf("commit chat history update tx: %w", err)
	}
	committed = true
	return cloneEntry(item), nil
}

func (s *Store) Delete(id string) error {
	if s == nil {
		return errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("history id is required")
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin chat history delete tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	state, err := s.loadStateLocked(tx)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(context.Background(), `DELETE FROM entries WHERE id = ?`, target)
	if err != nil {
		return fmt.Errorf("delete chat history entry: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read chat history delete rows affected: %w", err)
	}
	if affected == 0 {
		return errors.New("chat history entry not found")
	}
	state.Revision = nextRevision(state.Revision)
	if err := s.saveStateLocked(tx, state); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chat history delete tx: %w", err)
	}
	committed = true
	return nil
}

func (s *Store) Clear() error {
	if s == nil {
		return errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin chat history clear tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	state, err := s.loadStateLocked(tx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(context.Background(), `DELETE FROM entries`); err != nil {
		return fmt.Errorf("clear chat history entries: %w", err)
	}
	state.Revision = nextRevision(state.Revision)
	if err := s.saveStateLocked(tx, state); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chat history clear tx: %w", err)
	}
	committed = true
	return nil
}

func (s *Store) SetLimit(limit int) (File, error) {
	if s == nil {
		return File{}, errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return File{}, s.err
	}
	if !isAllowedLimit(limit) {
		return File{}, fmt.Errorf("unsupported chat history limit: %d", limit)
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return File{}, fmt.Errorf("begin chat history set limit tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	state, err := s.loadStateLocked(tx)
	if err != nil {
		return File{}, err
	}
	state.Limit = limit
	state.Revision = nextRevision(state.Revision)
	if state.Limit > DisabledLimit {
		if err := s.trimEntriesToLimitLocked(tx, state.Limit); err != nil {
			return File{}, err
		}
	}
	if err := s.saveStateLocked(tx, state); err != nil {
		return File{}, err
	}
	if err := tx.Commit(); err != nil {
		return File{}, fmt.Errorf("commit chat history set limit tx: %w", err)
	}
	committed = true

	return s.snapshotLocked()
}

func (s *Store) RecordCall(statusCode int) error {
	if s == nil {
		return errors.New("chat history store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if statusCode <= 0 {
		return nil
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin chat history record call tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	state, err := s.loadStateLocked(tx)
	if err != nil {
		return err
	}
	state.TotalCalls++
	if statusCode >= httpStatusSuccessLowerBound && statusCode < httpStatusFailureLowerBound {
		state.SuccessCalls++
	}
	if statusCode >= httpStatusFailureLowerBound {
		state.FailedCalls++
	}
	state.Revision = nextRevision(state.Revision)
	if err := s.saveStateLocked(tx, state); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chat history record call tx: %w", err)
	}
	committed = true
	return nil
}

const (
	httpStatusSuccessLowerBound = 200
	httpStatusFailureLowerBound = 400
)

func (s *Store) initLocked() error {
	if strings.TrimSpace(s.path) == "" {
		return errors.New("chat history path is required")
	}
	dir := filepath.Dir(s.path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create chat history dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return fmt.Errorf("open chat history sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping chat history sqlite: %w", err)
	}
	s.db = db

	if _, err := s.db.ExecContext(context.Background(), `PRAGMA busy_timeout = 5000`); err != nil {
		return fmt.Errorf("set chat history sqlite busy timeout: %w", err)
	}

	if err := s.ensureSchemaLocked(); err != nil {
		return err
	}
	state, err := s.loadStateLocked(s.db)
	if err != nil {
		return err
	}

	changed := false
	if state.Version != FileVersion {
		state.Version = FileVersion
		changed = true
	}
	if !isAllowedLimit(state.Limit) {
		state.Limit = DefaultLimit
		changed = true
	}
	if changed {
		state.Revision = nextRevision(state.Revision)
		if err := s.saveStateLocked(s.db, state); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureSchemaLocked() error {
	if s.db == nil {
		return errors.New("chat history sqlite is not initialized")
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL,
			limit_value INTEGER NOT NULL,
			revision INTEGER NOT NULL,
			total_calls INTEGER NOT NULL,
			success_calls INTEGER NOT NULL,
			failed_calls INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS entries (
			id TEXT PRIMARY KEY,
			revision INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			completed_at INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			caller_id TEXT NOT NULL DEFAULT '',
			account_id TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			stream INTEGER NOT NULL DEFAULT 0,
			user_input TEXT NOT NULL DEFAULT '',
			messages_json TEXT NOT NULL DEFAULT '[]',
			history_text TEXT NOT NULL DEFAULT '',
			final_prompt TEXT NOT NULL DEFAULT '',
			reasoning_content TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			error_text TEXT NOT NULL DEFAULT '',
			status_code INTEGER NOT NULL DEFAULT 0,
			elapsed_ms INTEGER NOT NULL DEFAULT 0,
			finish_reason TEXT NOT NULL DEFAULT '',
			usage_json TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_updated_at ON entries(updated_at DESC, created_at DESC)`,
		fmt.Sprintf(
			`INSERT INTO state(id, version, limit_value, revision, total_calls, success_calls, failed_calls)
			 VALUES (1, %d, %d, 0, 0, 0, 0)
			 ON CONFLICT(id) DO NOTHING`,
			FileVersion,
			DefaultLimit,
		),
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(context.Background(), stmt); err != nil {
			return fmt.Errorf("init chat history sqlite schema: %w", err)
		}
	}
	return nil
}

func (s *Store) snapshotLocked() (File, error) {
	state, err := s.loadStateLocked(s.db)
	if err != nil {
		return File{}, err
	}
	items, err := s.listSummaryEntriesLocked(s.db, state.Limit)
	if err != nil {
		return File{}, err
	}
	return File{
		Version:  state.Version,
		Limit:    state.Limit,
		Revision: state.Revision,
		Stats: Stats{
			TotalCalls:   state.TotalCalls,
			SuccessCalls: state.SuccessCalls,
			FailedCalls:  state.FailedCalls,
		},
		Items: items,
	}, nil
}

func (s *Store) loadStateLocked(q queryer) (stateRow, error) {
	var state stateRow
	err := q.QueryRowContext(
		context.Background(),
		`SELECT version, limit_value, revision, total_calls, success_calls, failed_calls FROM state WHERE id = 1`,
	).Scan(
		&state.Version,
		&state.Limit,
		&state.Revision,
		&state.TotalCalls,
		&state.SuccessCalls,
		&state.FailedCalls,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return stateRow{}, errors.New("chat history state is not initialized")
		}
		return stateRow{}, fmt.Errorf("read chat history state: %w", err)
	}
	return state, nil
}

func (s *Store) saveStateLocked(q queryer, state stateRow) error {
	if !isAllowedLimit(state.Limit) {
		state.Limit = DefaultLimit
	}
	if _, err := q.ExecContext(
		context.Background(),
		`UPDATE state
		 SET version = ?, limit_value = ?, revision = ?, total_calls = ?, success_calls = ?, failed_calls = ?
		 WHERE id = 1`,
		state.Version,
		state.Limit,
		state.Revision,
		state.TotalCalls,
		state.SuccessCalls,
		state.FailedCalls,
	); err != nil {
		return fmt.Errorf("write chat history state: %w", err)
	}
	return nil
}

func (s *Store) insertEntryLocked(q queryer, item Entry) error {
	messagesJSON, err := json.Marshal(cloneMessages(item.Messages))
	if err != nil {
		return fmt.Errorf("encode chat history messages: %w", err)
	}
	usageJSON, err := encodeUsage(item.Usage)
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(
		context.Background(),
		`INSERT INTO entries (
			id, revision, created_at, updated_at, completed_at, status, caller_id, account_id, model, stream,
			user_input, messages_json, history_text, final_prompt, reasoning_content, content, error_text,
			status_code, elapsed_ms, finish_reason, usage_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		item.Revision,
		item.CreatedAt,
		item.UpdatedAt,
		item.CompletedAt,
		item.Status,
		item.CallerID,
		item.AccountID,
		item.Model,
		boolToInt(item.Stream),
		item.UserInput,
		string(messagesJSON),
		item.HistoryText,
		item.FinalPrompt,
		item.ReasoningContent,
		item.Content,
		item.Error,
		item.StatusCode,
		item.ElapsedMs,
		item.FinishReason,
		usageJSON,
	); err != nil {
		return fmt.Errorf("insert chat history entry: %w", err)
	}
	return nil
}

func (s *Store) updateEntryLocked(q queryer, item Entry) error {
	messagesJSON, err := json.Marshal(cloneMessages(item.Messages))
	if err != nil {
		return fmt.Errorf("encode chat history messages: %w", err)
	}
	usageJSON, err := encodeUsage(item.Usage)
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(
		context.Background(),
		`UPDATE entries SET
			revision = ?, updated_at = ?, completed_at = ?, status = ?, caller_id = ?, account_id = ?, model = ?, stream = ?,
			user_input = ?, messages_json = ?, history_text = ?, final_prompt = ?, reasoning_content = ?, content = ?,
			error_text = ?, status_code = ?, elapsed_ms = ?, finish_reason = ?, usage_json = ?
		WHERE id = ?`,
		item.Revision,
		item.UpdatedAt,
		item.CompletedAt,
		item.Status,
		item.CallerID,
		item.AccountID,
		item.Model,
		boolToInt(item.Stream),
		item.UserInput,
		string(messagesJSON),
		item.HistoryText,
		item.FinalPrompt,
		item.ReasoningContent,
		item.Content,
		item.Error,
		item.StatusCode,
		item.ElapsedMs,
		item.FinishReason,
		usageJSON,
		item.ID,
	); err != nil {
		return fmt.Errorf("update chat history entry: %w", err)
	}
	return nil
}

func (s *Store) trimEntriesToLimitLocked(q queryer, limit int) error {
	if limit <= DisabledLimit {
		return nil
	}
	if _, err := q.ExecContext(
		context.Background(),
		`DELETE FROM entries
		 WHERE id IN (
			SELECT id FROM entries
			ORDER BY updated_at DESC, created_at DESC
			LIMIT -1 OFFSET ?
		 )`,
		limit,
	); err != nil {
		return fmt.Errorf("trim chat history entries: %w", err)
	}
	return nil
}

func (s *Store) listSummaryEntriesLocked(q queryer, limit int) ([]SummaryEntry, error) {
	query := `SELECT
		id, revision, created_at, updated_at, completed_at, status, caller_id, account_id, model,
		stream, user_input, status_code, elapsed_ms, finish_reason, reasoning_content, content, error_text
	FROM entries
	ORDER BY updated_at DESC, created_at DESC`
	args := []any{}
	if limit > DisabledLimit {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := q.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chat history summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]SummaryEntry, 0)
	for rows.Next() {
		var (
			item       Entry
			streamFlag int
		)
		if err := rows.Scan(
			&item.ID,
			&item.Revision,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.CompletedAt,
			&item.Status,
			&item.CallerID,
			&item.AccountID,
			&item.Model,
			&streamFlag,
			&item.UserInput,
			&item.StatusCode,
			&item.ElapsedMs,
			&item.FinishReason,
			&item.ReasoningContent,
			&item.Content,
			&item.Error,
		); err != nil {
			return nil, fmt.Errorf("scan chat history summary: %w", err)
		}
		item.Stream = streamFlag == 1
		out = append(out, summaryFromEntry(item))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat history summaries: %w", err)
	}
	return out, nil
}

func (s *Store) getEntryLocked(q queryer, id string) (Entry, error) {
	var (
		item         Entry
		streamFlag   int
		messagesJSON string
		usageJSON    string
	)
	err := q.QueryRowContext(
		context.Background(),
		`SELECT
			id, revision, created_at, updated_at, completed_at, status, caller_id, account_id, model, stream,
			user_input, messages_json, history_text, final_prompt, reasoning_content, content, error_text,
			status_code, elapsed_ms, finish_reason, usage_json
		FROM entries
		WHERE id = ?`,
		id,
	).Scan(
		&item.ID,
		&item.Revision,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.CompletedAt,
		&item.Status,
		&item.CallerID,
		&item.AccountID,
		&item.Model,
		&streamFlag,
		&item.UserInput,
		&messagesJSON,
		&item.HistoryText,
		&item.FinalPrompt,
		&item.ReasoningContent,
		&item.Content,
		&item.Error,
		&item.StatusCode,
		&item.ElapsedMs,
		&item.FinishReason,
		&usageJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, errors.New("chat history entry not found")
		}
		return Entry{}, fmt.Errorf("read chat history entry: %w", err)
	}
	item.Stream = streamFlag == 1

	messages, err := decodeMessages(messagesJSON)
	if err != nil {
		return Entry{}, err
	}
	item.Messages = messages
	usage, err := decodeUsage(usageJSON)
	if err != nil {
		return Entry{}, err
	}
	item.Usage = usage
	return item, nil
}

func summaryFromEntry(item Entry) SummaryEntry {
	return SummaryEntry{
		ID:             item.ID,
		Revision:       item.Revision,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		CompletedAt:    item.CompletedAt,
		Status:         item.Status,
		CallerID:       item.CallerID,
		AccountID:      item.AccountID,
		Model:          item.Model,
		Stream:         item.Stream,
		UserInput:      item.UserInput,
		Preview:        buildPreview(item),
		StatusCode:     item.StatusCode,
		ElapsedMs:      item.ElapsedMs,
		FinishReason:   item.FinishReason,
		DetailRevision: item.Revision,
	}
}

func buildPreview(item Entry) string {
	candidate := strings.TrimSpace(item.Content)
	if candidate == "" {
		candidate = strings.TrimSpace(item.ReasoningContent)
	}
	if candidate == "" {
		candidate = strings.TrimSpace(item.Error)
	}
	if candidate == "" {
		candidate = strings.TrimSpace(item.UserInput)
	}
	if len(candidate) > defaultPreviewAt {
		return candidate[:defaultPreviewAt] + "..."
	}
	return candidate
}

func encodeUsage(in map[string]any) (string, error) {
	if in == nil {
		return "", nil
	}
	body, err := json.Marshal(cloneMap(in))
	if err != nil {
		return "", fmt.Errorf("encode chat history usage: %w", err)
	}
	return string(body), nil
}

func decodeUsage(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode chat history usage: %w", err)
	}
	return out, nil
}

func decodeMessages(raw string) ([]Message, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []Message{}, nil
	}
	var out []Message
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode chat history messages: %w", err)
	}
	return cloneMessages(out), nil
}

func nextRevision(current int64) int64 {
	next := time.Now().UnixNano()
	if next <= current {
		next = current + 1
	}
	return next
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ListETag(revision int64) string {
	return fmt.Sprintf(`W/"chat-history-list-%d"`, revision)
}

func DetailETag(id string, revision int64) string {
	return fmt.Sprintf(`W/"chat-history-detail-%s-%d"`, strings.TrimSpace(id), revision)
}

func isAllowedLimit(limit int) bool {
	_, ok := allowedLimits[limit]
	return ok
}

func cloneEntry(item Entry) Entry {
	item.Usage = cloneMap(item.Usage)
	item.Messages = cloneMessages(item.Messages)
	return item
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return []Message{}
	}
	out := make([]Message, len(messages))
	copy(out, messages)
	return out
}
