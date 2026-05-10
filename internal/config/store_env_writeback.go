package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func envWritebackEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DS2API_ENV_WRITEBACK")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (s *Store) IsEnvBacked() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fromEnv
}

func (s *Store) IsEnvWritebackEnabled() bool {
	return envWritebackEnabled()
}

func (s *Store) HasEnvConfigSource() bool {
	return strings.TrimSpace(os.Getenv("DS2API_CONFIG_JSON")) != ""
}

func (s *Store) ConfigPath() string {
	if s == nil {
		return ConfigPath()
	}
	return s.path
}

func writeConfigBytes(path string, b []byte) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return os.WriteFile(path, b, 0o644)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	return os.WriteFile(path, b, 0o644)
}
