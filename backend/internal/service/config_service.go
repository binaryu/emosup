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
	return s.store.LoadConfig()
}

func (s *ConfigService) SaveConfig(_ context.Context, cfg model.AppConfig) (model.AppConfig, error) {
	if err := s.store.SaveConfig(cfg); err != nil {
		return model.AppConfig{}, err
	}

	return cfg, nil
}
