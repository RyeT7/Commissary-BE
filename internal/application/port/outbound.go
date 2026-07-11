package port

import (
	"context"
	"io"

	"commissary/internal/domain/asset"
	"commissary/internal/domain/folder"
)

type ByteRange struct {
	Start int64
	End   int64
}

type BlobStore interface {
	Put(ctx context.Context, key asset.StorageKey, r io.Reader, size int64) (written int64, err error)
	Get(ctx context.Context, key asset.StorageKey) (io.ReadCloser, error)
	GetRange(ctx context.Context, key asset.StorageKey, rng ByteRange) (io.ReadCloser, error)
	Size(ctx context.Context, key asset.StorageKey) (int64, error)
	Delete(ctx context.Context, key asset.StorageKey) error
}

type AssetRepository interface {
	Save(ctx context.Context, a *asset.Asset) error
	FindByID(ctx context.Context, id asset.ID) (*asset.Asset, error)
	ListByFolder(ctx context.Context, ownerID string, folderID *folder.ID) ([]*asset.Asset, error)
	Delete(ctx context.Context, id asset.ID) error
}

type FolderRepository interface {
	Save(ctx context.Context, f *folder.Folder) error
	FindByID(ctx context.Context, id folder.ID) (*folder.Folder, error)
	FindByParentAndName(ctx context.Context, ownerID string, parentID *folder.ID, name string) (*folder.Folder, error)
	ListChildren(ctx context.Context, ownerID string, parentID *folder.ID) ([]*folder.Folder, error)
	Delete(ctx context.Context, id folder.ID) error
}
