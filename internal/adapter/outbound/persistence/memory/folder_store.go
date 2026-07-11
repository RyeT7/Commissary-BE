package memory

import (
	"context"
	"sync"

	"commissary/internal/application/port"
	"commissary/internal/domain/folder"
)

type FolderStore struct {
	mu   sync.RWMutex
	data map[folder.ID]*folder.Folder
}

func NewFolderStore() *FolderStore {
	return &FolderStore{data: make(map[folder.ID]*folder.Folder)}
}

var _ port.FolderRepository = (*FolderStore)(nil)

func (s *FolderStore) Save(ctx context.Context, f *folder.Folder) error {
	cp := *f
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[f.ID] = &cp
	return nil
}

func (s *FolderStore) FindByID(ctx context.Context, id folder.ID) (*folder.Folder, error) {
	s.mu.RLock()
	f, ok := s.data[id]
	s.mu.RUnlock()
	if !ok {
		return nil, folder.ErrNotFound
	}
	cp := *f
	return &cp, nil
}

func (s *FolderStore) FindByParentAndName(ctx context.Context, ownerID string, parentID *folder.ID, name string) (*folder.Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.data {
		if f.OwnerID == ownerID && sameFolderID(f.ParentID, parentID) && f.Name == name {
			cp := *f
			return &cp, nil
		}
	}
	return nil, folder.ErrNotFound
}

func (s *FolderStore) ListChildren(ctx context.Context, ownerID string, parentID *folder.ID) ([]*folder.Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*folder.Folder, 0)
	for _, f := range s.data {
		if f.OwnerID == ownerID && sameFolderID(f.ParentID, parentID) {
			cp := *f
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *FolderStore) Delete(ctx context.Context, id folder.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}
