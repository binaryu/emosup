package service

import (
	"context"

	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
)

type ConfigService struct {
	store *store.FileStore
}

func NewConfigService(store *store.FileStore) *ConfigService {
	return &ConfigService{store: store}
}

func (s *ConfigService) GetConfig(_ context.Context) (model.AppConfig, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.AppConfig{}, err
	}
	return RedactAuthForResponse(cfg), nil
}

func (s *ConfigService) SaveConfig(_ context.Context, cfg model.AppConfig) (model.AppConfig, error) {
	existing, err := s.store.LoadConfig()
	if err != nil {
		return model.AppConfig{}, err
	}

	merged, err := MergeAuthOnSave(existing, cfg)
	if err != nil {
		return model.AppConfig{}, err
	}

	if err := s.store.SaveConfig(merged); err != nil {
		return model.AppConfig{}, err
	}

	return RedactAuthForResponse(merged), nil
}
