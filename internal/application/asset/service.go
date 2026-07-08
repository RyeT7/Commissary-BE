package asset

import (
	"context"
	"io"

	"commissary/internal/application/port"
	domain "commissary/internal/domain/asset"
)

type Service struct {
	blobs port.BlobStore
	repo  port.AssetRepository
}

func NewService(blobs port.BlobStore, repo port.AssetRepository) *Service {
	return &Service{blobs: blobs, repo: repo}
}

type UploadCommand struct {
	OwnerID string
	Name    string
	Size    int64
	Content io.Reader
}

func (s *Service) Upload(ctx context.Context, cmd UploadCommand) (*domain.Asset, error) {
	panic("not implemented")
}

func (s *Service) Download(ctx context.Context, id domain.ID) (*domain.Asset, io.ReadCloser, error) {
	panic("not implemented")
}
