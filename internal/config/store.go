package config

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	configSchemaVersion = 1
	configStateRowID    = 1
)

type Store struct {
	mu       sync.RWMutex
	cfg      Config
	path     string
	db       *sql.DB
	fromEnv  bool
	jsonFile bool
	keyMap   map[string]struct{} // O(1) API key lookup index
	accMap   map[string]int      // O(1) account lookup: identifier -> slice index
	accTest  map[string]string   // runtime-only account test status cache
}

func LoadStore() *Store {
	store, err := loadStore()
	if err != nil {
		Logger.Warn("[config] load failed", "error", err)
	}
	if len(store.cfg.Keys) == 0 && len(store.cfg.Accounts) == 0 {
		Logger.Warn("[config] empty config loaded")
	}
	store.rebuildIndexes()
	return store
}

func LoadStoreWithError() (*Store, error) {
	store, err := loadStore()
	if err != nil {
		return nil, err
	}
	store.rebuildIndexes()
	return store, nil
}

func loadStore() (*Store, error) {
	path := ConfigPath()
	if rawCfg := strings.TrimSpace(os.Getenv("DS2API_CONFIG_JSON")); rawCfg != "" {
		if envWritebackEnabled() && !IsVercel() {
			if persisted, exists, persistedErr := loadExistingFileBackedStore(path); persistedErr == nil && exists {
				return persisted, nil
			} else if persistedErr != nil {
				Logger.Warn("[config] persisted config fallback unavailable", "error", persistedErr)
			}
		}

		cfg, err := parseConfigString(rawCfg)
		cfg.NormalizeCredentials()
		cfg.ClearAccountTokens()
		cfg.DropInvalidAccounts()
		if err != nil {
			if envWritebackEnabled() && !IsVercel() {
				if fallback, exists, fallbackErr := loadExistingFileBackedStore(path); fallbackErr == nil && exists {
					return fallback, nil
				} else if fallbackErr != nil {
					Logger.Warn("[config] persisted config fallback unavailable", "error", fallbackErr)
				}
			}
			return &Store{cfg: cfg.Clone(), path: path, fromEnv: true}, err
		}
		if validateErr := ValidateConfig(cfg); validateErr != nil {
			return &Store{cfg: cfg.Clone(), path: path, fromEnv: true}, validateErr
		}
		if envWritebackEnabled() && !IsVercel() {
			store, err := newFileBackedStore(path, cfg)
			if err == nil {
				return store, nil
			}
			Logger.Warn("[config] env writeback bootstrap failed", "error", err)
		}
		return &Store{cfg: cfg.Clone(), path: path, fromEnv: true}, nil
	}

	if IsVercel() {
		if persisted, exists, persistedErr := loadExistingFileBackedStore(path); persistedErr == nil && exists {
			return persisted, nil
		} else if persistedErr != nil {
			Logger.Warn("[config] vercel persisted config unavailable; using memory config", "error", persistedErr)
		}
		return &Store{cfg: Config{}, path: path, fromEnv: true}, nil
	}

	return loadFileBackedStore(path)
}

func loadExistingFileBackedStore(path string) (*Store, bool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, false, nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, true, err
	}
	store, err := loadFileBackedStore(path)
	return store, true, err
}

func loadFileBackedStore(path string) (*Store, error) {
	if shouldUseJSONConfig(path) {
		cfg, err := loadConfigFromJSONFile(path)
		if err != nil {
			if shouldBootstrapMissingConfigFile(err) {
				Logger.Warn("[config] config file missing; starting with empty file-backed config", "path", path)
				cfg = Config{}
				err = nil
			} else {
				return &Store{cfg: Config{}, path: path, jsonFile: true}, err
			}
		}
		cfg.NormalizeCredentials()
		cfg.DropInvalidAccounts()
		if validateErr := ValidateConfig(cfg); validateErr != nil {
			err = errors.Join(err, validateErr)
		}
		return &Store{cfg: cfg.Clone(), path: path, jsonFile: true}, err
	}

	db, err := openConfigSQLite(path)
	if err != nil {
		return &Store{cfg: Config{}, path: path}, err
	}
	cfg, err := loadConfigFromSQLite(db)
	cfg.NormalizeCredentials()
	cfg.DropInvalidAccounts()
	if validateErr := ValidateConfig(cfg); validateErr != nil {
		err = errors.Join(err, validateErr)
	}
	store := &Store{
		cfg:  cfg.Clone(),
		path: path,
		db:   db,
	}
	if saveErr := store.saveLocked(); saveErr != nil {
		err = errors.Join(err, saveErr)
	}
	return store, err
}

func newFileBackedStore(path string, cfg Config) (*Store, error) {
	if shouldUseJSONConfig(path) {
		store := &Store{cfg: cfg.Clone(), path: path, jsonFile: true}
		if err := store.saveLocked(); err != nil {
			return store, err
		}
		return store, nil
	}

	db, err := openConfigSQLite(path)
	if err != nil {
		return &Store{cfg: Config{}, path: path}, err
	}
	store := &Store{cfg: cfg.Clone(), path: path, db: db}
	if err := store.saveLocked(); err != nil {
		return store, err
	}
	return store, nil
}

func loadConfig() (Config, bool, error) {
	store, err := loadStore()
	if store == nil {
		return Config{}, false, err
	}
	cfg := store.Snapshot()
	if closeErr := store.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	return cfg, store.fromEnv, err
}

func shouldBootstrapMissingConfigFile(err error) bool {
	return errors.Is(err, os.ErrNotExist) && strings.TrimSpace(os.Getenv("DS2API_CONFIG_PATH")) != ""
}

func shouldUseJSONConfig(path string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".json")
}

func loadConfigFromJSONFile(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(content, &cfg); err != nil {
		return Config{}, err
	}
	cfg.NormalizeCredentials()
	cfg.DropInvalidAccounts()
	if strings.Contains(string(content), "\"test_status\"") && !IsVercel() {
		persistCfg := cfg.Clone()
		persistCfg.ClearAccountTokens()
		if b, marshalErr := json.MarshalIndent(persistCfg, "", "  "); marshalErr != nil {
			Logger.Warn("[config] sanitize legacy account test status failed", "error", marshalErr)
		} else if writeErr := writeConfigBytes(path, b); writeErr != nil {
			Logger.Warn("[config] sanitize legacy account test status failed", "error", writeErr)
		}
	}
	return cfg, nil
}

func openConfigSQLite(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("config sqlite path is required")
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create config sqlite dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open config sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping config sqlite: %w", err)
	}
	if _, err := db.ExecContext(context.Background(), `PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set config sqlite busy timeout: %w", err)
	}
	if err := ensureConfigSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ensureConfigSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS config_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL,
			revision INTEGER NOT NULL,
			config_json TEXT NOT NULL
		)`,
		fmt.Sprintf(
			`INSERT INTO config_state(id, version, revision, config_json)
			 VALUES (1, %d, 0, '{}')
			 ON CONFLICT(id) DO NOTHING`,
			configSchemaVersion,
		),
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			return fmt.Errorf("init config sqlite schema: %w", err)
		}
	}
	return nil
}

func loadConfigFromSQLite(db *sql.DB) (Config, error) {
	var raw string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT config_json FROM config_state WHERE id = ?`,
		configStateRowID,
	).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Config{}, errors.New("config sqlite state is not initialized")
		}
		return Config{}, fmt.Errorf("read config sqlite row: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config sqlite json: %w", err)
	}
	return cfg, nil
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
		return fmt.Errorf("close config sqlite: %w", err)
	}
	s.db = nil
	return nil
}

func (s *Store) Snapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Clone()
}

func (s *Store) HasAPIKey(k string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.keyMap[k]
	return ok
}

func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.cfg.Keys)
}

func (s *Store) Accounts() []Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.cfg.Accounts)
}

func (s *Store) FindAccount(identifier string) (Account, bool) {
	identifier = strings.TrimSpace(identifier)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if idx, ok := s.findAccountIndexLocked(identifier); ok {
		return s.cfg.Accounts[idx], true
	}
	return Account{}, false
}

func (s *Store) UpdateAccountTestStatus(identifier, status string) error {
	identifier = strings.TrimSpace(identifier)
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, ok := s.findAccountIndexLocked(identifier)
	if !ok {
		return errors.New("account not found")
	}
	s.setAccountTestStatusLocked(s.cfg.Accounts[idx], status, identifier)
	return nil
}

func (s *Store) AccountTestStatus(identifier string) (string, bool) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	status, ok := s.accTest[identifier]
	return status, ok
}

func (s *Store) UpdateAccountToken(identifier, token string) error {
	identifier = strings.TrimSpace(identifier)
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, ok := s.findAccountIndexLocked(identifier)
	if !ok {
		return errors.New("account not found")
	}
	oldID := s.cfg.Accounts[idx].Identifier()
	s.cfg.Accounts[idx].Token = token
	newID := s.cfg.Accounts[idx].Identifier()
	// Keep historical aliases usable for long-lived queues while also adding
	// the latest identifier after token refresh.
	if identifier != "" {
		s.accMap[identifier] = idx
	}
	if oldID != "" {
		s.accMap[oldID] = idx
	}
	if newID != "" {
		s.accMap[newID] = idx
	}
	return s.saveLocked()
}

func (s *Store) Replace(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg.NormalizeCredentials()
	s.cfg = cfg.Clone()
	s.rebuildIndexes()
	return s.saveLocked()
}

func (s *Store) Update(mutator func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	base := s.cfg.Clone()
	cfg := base.Clone()
	if err := mutator(&cfg); err != nil {
		return err
	}
	cfg.ReconcileCredentials(base)
	cfg.NormalizeCredentials()
	s.cfg = cfg
	s.rebuildIndexes()
	return s.saveLocked()
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if s == nil {
		return errors.New("config store is nil")
	}
	if s.fromEnv && (IsVercel() || !envWritebackEnabled()) {
		Logger.Info("[save_config] source from env, skip write")
		return nil
	}
	if s.jsonFile {
		persistCfg := s.cfg.Clone()
		persistCfg.ClearAccountTokens()
		b, err := json.MarshalIndent(persistCfg, "", "  ")
		if err != nil {
			return err
		}
		if err := writeConfigBytes(s.path, b); err != nil {
			return err
		}
		s.fromEnv = false
		return nil
	}
	if s == nil || s.db == nil {
		return errors.New("config sqlite is not initialized")
	}
	persistCfg := s.cfg.Clone()
	persistCfg.ClearAccountTokens()
	b, err := json.Marshal(persistCfg)
	if err != nil {
		return err
	}
	revision := time.Now().UnixNano()
	if _, err := s.db.ExecContext(
		context.Background(),
		`UPDATE config_state SET version = ?, revision = ?, config_json = ? WHERE id = ?`,
		configSchemaVersion,
		revision,
		string(b),
		configStateRowID,
	); err != nil {
		return fmt.Errorf("write config sqlite row: %w", err)
	}
	s.fromEnv = false
	return nil
}

func (s *Store) SetVercelSync(hash string, ts int64) error {
	return s.Update(func(c *Config) error {
		c.VercelSyncHash = hash
		c.VercelSyncTime = ts
		return nil
	})
}

func (s *Store) ExportJSONAndBase64() (string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exportCfg := s.cfg.Clone()
	exportCfg.ClearAccountTokens()
	b, err := json.Marshal(exportCfg)
	if err != nil {
		return "", "", err
	}
	return string(b), base64.StdEncoding.EncodeToString(b), nil
}
