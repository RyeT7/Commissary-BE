package port

import (
	"context"
	"io"

	"commissary/internal/domain/asset"
)

type BlobStore interface {
	Put(ctx context.Context, key asset.StorageKey, r io.Reader, size int64) error
	Get(ctx context.Context, key asset.StorageKey) (io.ReadCloser, error)
	Delete(ctx context.Context, key asset.StorageKey) error
}

type AssetRepository interface {
	Save(ctx context.Context, a *asset.Asset) error
	FindByID(ctx context.Context, id asset.ID) (*asset.Asset, error)
	Delete(ctx context.Context, id asset.ID) error
}
