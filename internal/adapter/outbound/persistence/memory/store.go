package memory

import (
	"context"
	"sync"

	"commissary/internal/application/port"
	"commissary/internal/domain/asset"
	"commissary/internal/domain/folder"
)

func sameFolderID(a, b *folder.ID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

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

func (s *Store) ListByFolder(ctx context.Context, ownerID string, folderID *folder.ID) ([]*asset.Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*asset.Asset, 0)
	for _, a := range s.data {
		if a.OwnerID == ownerID && sameFolderID(a.FolderID, folderID) {
			cp := *a
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *Store) Delete(ctx context.Context, id asset.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}
