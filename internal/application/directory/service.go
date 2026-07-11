package directory

import (
	"context"
	"errors"
	"time"

	"commissary/internal/application/port"
	"commissary/internal/domain/asset"
	"commissary/internal/domain/folder"
)

type Service struct {
	folders port.FolderRepository
	assets  port.AssetRepository
	blobs   port.BlobStore
}

func NewService(folders port.FolderRepository, assets port.AssetRepository, blobs port.BlobStore) *Service {
	return &Service{folders: folders, assets: assets, blobs: blobs}
}

type Listing struct {
	Folders []*folder.Folder
	Assets  []*asset.Asset
}

func (s *Service) Get(ctx context.Context, id folder.ID) (*folder.Folder, error) {
	return s.folders.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, ownerID string, parentID *folder.ID) (*Listing, error) {
	if parentID != nil {
		if _, err := s.folders.FindByID(ctx, *parentID); err != nil {
			return nil, err
		}
	}

	folders, err := s.folders.ListChildren(ctx, ownerID, parentID)
	if err != nil {
		return nil, err
	}
	assets, err := s.assets.ListByFolder(ctx, ownerID, parentID)
	if err != nil {
		return nil, err
	}
	return &Listing{Folders: folders, Assets: assets}, nil
}

func (s *Service) Mkdir(ctx context.Context, ownerID string, parentID *folder.ID, name string) (*folder.Folder, error) {
	if name == "" {
		return nil, folder.ErrInvalidName
	}

	_, err := s.folders.FindByParentAndName(ctx, ownerID, parentID, name)
	if err == nil {
		return nil, folder.ErrExists
	}
	if !errors.Is(err, folder.ErrNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	f := &folder.Folder{
		ID:        folder.NewID(),
		ParentID:  parentID,
		OwnerID:   ownerID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.folders.Save(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) Remove(ctx context.Context, ownerID string, parentID *folder.ID, name string) error {
	listing, err := s.List(ctx, ownerID, parentID)
	if err != nil {
		return err
	}

	for _, f := range listing.Folders {
		if f.Name != name {
			continue
		}
		children, err := s.List(ctx, ownerID, &f.ID)
		if err != nil {
			return err
		}
		if len(children.Folders) > 0 || len(children.Assets) > 0 {
			return folder.ErrNotEmpty
		}
		return s.folders.Delete(ctx, f.ID)
	}

	for _, a := range listing.Assets {
		if a.Name != name {
			continue
		}
		if err := s.assets.Delete(ctx, a.ID); err != nil {
			return err
		}
		return s.blobs.Delete(ctx, a.StorageKey)
	}

	return asset.ErrNotFound
}
