package service

import (
	"context"

	"emosup/backend/internal/client"
	"emosup/backend/internal/store"
)

type EmosService struct {
	store  *store.FileStore
	client client.EmosClient
}

func NewEmosService(store *store.FileStore, emosClient client.EmosClient) *EmosService {
	return &EmosService{
		store:  store,
		client: emosClient,
	}
}

func (s *EmosService) GetVideoTree(ctx context.Context, tmdbID int64, videoType string) (client.EmosVideoTree, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return client.EmosVideoTree{}, err
	}

	return s.client.GetVideoTree(ctx, client.EmosAccess{
		BaseURL: cfg.Emos.BaseURL,
		Token:   cfg.Emos.Token,
	}, tmdbID, videoType)
}

func (s *EmosService) GetVideoBase(ctx context.Context, itemType string, itemID int64) (client.EmosVideoBase, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return client.EmosVideoBase{}, err
	}

	return s.client.GetVideoBase(ctx, client.EmosAccess{
		BaseURL: cfg.Emos.BaseURL,
		Token:   cfg.Emos.Token,
	}, itemType, itemID)
}
