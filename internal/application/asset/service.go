package asset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"

	"commissary/internal/application/port"
	domain "commissary/internal/domain/asset"
	"commissary/internal/domain/folder"
)

type Service struct {
	blobs port.BlobStore
	repo  port.AssetRepository
}

func NewService(blobs port.BlobStore, repo port.AssetRepository) *Service {
	return &Service{blobs: blobs, repo: repo}
}

type UploadCommand struct {
	OwnerID  string
	FolderID *folder.ID
	Name     string
	MIMEType domain.MIMEType
	Size     int64
	Content  io.Reader
}

func (s *Service) Upload(ctx context.Context, cmd UploadCommand) (*domain.Asset, error) {
	if cmd.Name == "" {
		return nil, domain.ErrInvalidName
	}
	if cmd.Content == nil {
		return nil, domain.ErrEmptyBlob
	}

	id := domain.NewID()
	key := domain.StorageKey(id)

	hasher := sha256.New()
	written, err := s.blobs.Put(ctx, key, io.TeeReader(cmd.Content, hasher), cmd.Size)
	if err != nil {
		return nil, err
	}
	if written == 0 {
		_ = s.blobs.Delete(ctx, key)
		return nil, domain.ErrEmptyBlob
	}

	now := time.Now().UTC()
	a := &domain.Asset{
		ID:         id,
		OwnerID:    cmd.OwnerID,
		FolderID:   cmd.FolderID,
		Name:       cmd.Name,
		MIMEType:   cmd.MIMEType,
		Size:       written,
		Checksum:   domain.Checksum(hex.EncodeToString(hasher.Sum(nil))),
		StorageKey: key,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.Save(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) Get(ctx context.Context, id domain.ID) (*domain.Asset, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) Download(ctx context.Context, id domain.ID) (*domain.Asset, io.ReadCloser, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	r, err := s.blobs.Get(ctx, a.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	return a, r, nil
}

type StreamResult struct {
	Asset         *domain.Asset
	Content       io.ReadCloser
	ContentLength int64
	TotalSize     int64
	Range         *port.ByteRange
}

func (s *Service) Stream(ctx context.Context, id domain.ID, rng *port.ByteRange) (*StreamResult, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	total, err := s.blobs.Size(ctx, a.StorageKey)
	if err != nil {
		return nil, err
	}

	if rng == nil {
		r, err := s.blobs.Get(ctx, a.StorageKey)
		if err != nil {
			return nil, err
		}
		return &StreamResult{Asset: a, Content: r, ContentLength: total, TotalSize: total}, nil
	}

	resolved := *rng
	if resolved.Start < 0 {
		n := -resolved.Start
		if n > total {
			n = total
		}
		resolved.Start = total - n
		resolved.End = total - 1
	}
	if resolved.End < 0 || resolved.End >= total {
		resolved.End = total - 1
	}
	if total == 0 || resolved.Start < 0 || resolved.Start > resolved.End {
		return nil, domain.ErrInvalidRange
	}

	r, err := s.blobs.GetRange(ctx, a.StorageKey, resolved)
	if err != nil {
		return nil, err
	}
	return &StreamResult{
		Asset:         a,
		Content:       r,
		ContentLength: resolved.End - resolved.Start + 1,
		TotalSize:     total,
		Range:         &resolved,
	}, nil
}
