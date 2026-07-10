package memory

import (
	"context"
	"sync"

	"commissary/internal/application/port"
	"commissary/internal/domain/asset"
)

type Store struct {
	mu   sync.RWMutex
	data map[asset.ID]*asset.Asset
}

func New() *Store {
	return &Store{data: make(map[asset.ID]*asset.Asset)}
}

var _ port.AssetRepository = (*Store)(nil)

func (s *Store) Save(ctx context.Context, a *asset.Asset) error {
	cp := *a
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[a.ID] = &cp
	return nil
}

func (s *Store) FindByID(ctx context.Context, id asset.ID) (*asset.Asset, error) {
	s.mu.RLock()
	a, ok := s.data[id]
	s.mu.RUnlock()
	if !ok {
		return nil, asset.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (s *Store) Delete(ctx context.Context, id asset.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}
