package service

import (
	"sync"

	"magaz/internal/models"
	"magaz/internal/repository"
)

// SettingsService caches site_settings in memory.
// Call Reload() after any write to refresh the cache.
type SettingsService struct {
	repo  *repository.SettingsRepository
	mu    sync.RWMutex
	cache models.SiteSettings
}

func NewSettingsService(repo *repository.SettingsRepository) (*SettingsService, error) {
	svc := &SettingsService{repo: repo}
	if err := svc.Reload(); err != nil {
		return nil, err
	}
	return svc, nil
}

// Reload fetches all settings from DB into memory.
func (s *SettingsService) Reload() error {
	m, err := s.repo.GetAll()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cache = m
	s.mu.Unlock()
	return nil
}

// Get returns the value for key, or fallback if not found / empty.
func (s *SettingsService) Get(key, fallback string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache.Get(key, fallback)
}

// GetAll returns a snapshot of all settings.
func (s *SettingsService) GetAll() models.SiteSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := make(models.SiteSettings, len(s.cache))
	for k, v := range s.cache {
		snap[k] = v
	}
	return snap
}

// Set saves a single key and reloads the cache.
func (s *SettingsService) Set(key, value string) error {
	if err := s.repo.Set(key, value); err != nil {
		return err
	}
	return s.Reload()
}
