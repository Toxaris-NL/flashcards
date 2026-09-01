// Package admin manages parent-only application settings.
package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// MixedSessionSettings controls multiple-choice percentages for mixed sessions.
type MixedSessionSettings struct {
	Easy int `json:"easy_multiple_choice_percent"`
	OK   int `json:"ok_multiple_choice_percent"`
	Hard int `json:"hard_multiple_choice_percent"`
}

// DefaultMixedSessionSettings returns the v1 mixed-session defaults.
func DefaultMixedSessionSettings() MixedSessionSettings {
	return MixedSessionSettings{Easy: 80, OK: 40, Hard: 5}
}

// SettingsStore persists admin settings in one JSON file.
type SettingsStore struct {
	mu   sync.Mutex
	path string
}

// NewSettingsStore creates a store at the supplied file path.
func NewSettingsStore(path string) *SettingsStore {
	return &SettingsStore{path: path}
}

// Load returns defaults until the admin saves settings.
func (s *SettingsStore) Load() (MixedSessionSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultMixedSessionSettings(), nil
	}
	if err != nil {
		return MixedSessionSettings{}, err
	}
	var settings MixedSessionSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return MixedSessionSettings{}, err
	}
	return settings, nil
}

// Save writes validated settings to disk.
func (s *SettingsStore) Save(settings MixedSessionSettings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// Validate ensures all configured percentages are usable.
func (s MixedSessionSettings) Validate() error {
	for _, percentage := range []int{s.Easy, s.OK, s.Hard} {
		if percentage < 0 || percentage > 100 {
			return fmt.Errorf("multiple-choice percentage must be between 0 and 100")
		}
	}
	return nil
}

// Service applies authorization and validation before changing admin settings.
type Service struct {
	store *SettingsStore
}

// NewService creates an admin settings service.
func NewService(store *SettingsStore) *Service {
	return &Service{store: store}
}

// UpdateMixedSessionSettings saves settings only for an authenticated admin.
func (s *Service) UpdateMixedSessionSettings(authenticated bool, settings MixedSessionSettings) error {
	if !authenticated {
		return errors.New("admin authentication required")
	}
	return s.store.Save(settings)
}
