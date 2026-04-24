package config

func (s *Store) IsEnvBacked() bool {
	return false
}

func (s *Store) IsEnvWritebackEnabled() bool {
	return false
}

func (s *Store) HasEnvConfigSource() bool {
	return false
}

func (s *Store) ConfigPath() string {
	if s == nil {
		return ConfigPath()
	}
	return s.path
}
