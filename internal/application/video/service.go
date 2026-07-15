package video

import (
	"context"
	"io"
	"time"

	"commissary/internal/application/port"
	domain "commissary/internal/domain/video"
)

type Service struct {
	repo  port.VideoRepository
	blobs port.BlobStore
}

func NewService(repo port.VideoRepository, blobs port.BlobStore) *Service {
	return &Service{repo: repo, blobs: blobs}
}

type UploadCommand struct {
	ChannelID     string
	Title         string
	Description   string
	MIMEType      string
	Size          int64
	Content       io.Reader
	Thumbnail     io.Reader
	ThumbnailMIME string
}

func (s *Service) Upload(ctx context.Context, cmd UploadCommand) (*domain.Video, error) {
	if cmd.Title == "" {
		return nil, domain.ErrInvalidTitle
	}
	if cmd.Content == nil {
		return nil, domain.ErrEmptyContent
	}

	id := domain.NewID()
	key := domain.StorageKey("videos/" + string(id))

	written, err := s.blobs.Put(ctx, key, cmd.Content, cmd.Size)
	if err != nil {
		return nil, err
	}
	if written == 0 {
		_ = s.blobs.Delete(ctx, key)
		return nil, domain.ErrEmptyContent
	}

	v := &domain.Video{
		ID:          id,
		ChannelID:   cmd.ChannelID,
		Title:       cmd.Title,
		Description: cmd.Description,
		StorageKey:  key,
		MIMEType:    cmd.MIMEType,
		Size:        written,
		CreatedAt:   time.Now().UTC(),
	}

	if cmd.Thumbnail != nil {
		thumbKey := domain.StorageKey("thumbnails/" + string(id))
		if _, err := s.blobs.Put(ctx, thumbKey, cmd.Thumbnail, -1); err != nil {
			_ = s.blobs.Delete(ctx, key)
			return nil, err
		}
		v.ThumbnailKey = thumbKey
		v.ThumbnailMIME = cmd.ThumbnailMIME
	}

	if err := s.repo.Save(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Service) Get(ctx context.Context, id domain.ID) (*domain.Video, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) Feed(ctx context.Context, limit int) ([]*domain.Video, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListRecent(ctx, limit)
}

func (s *Service) ByChannel(ctx context.Context, channelID string) ([]*domain.Video, error) {
	return s.repo.ListByChannel(ctx, channelID)
}

func (s *Service) RecordView(ctx context.Context, id domain.ID) error {
	return s.repo.IncrementViews(ctx, id)
}

func (s *Service) OpenThumbnail(ctx context.Context, v *domain.Video) (io.ReadCloser, string, error) {
	if v.ThumbnailKey == "" {
		return nil, "", domain.ErrNotFound
	}
	r, err := s.blobs.Get(ctx, v.ThumbnailKey)
	if err != nil {
		return nil, "", err
	}
	return r, v.ThumbnailMIME, nil
}

type StreamResult struct {
	Video         *domain.Video
	Content       io.ReadCloser
	ContentLength int64
	TotalSize     int64
	Range         *port.ByteRange
}

func (s *Service) Stream(ctx context.Context, id domain.ID, rng *port.ByteRange) (*StreamResult, error) {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	total, err := s.blobs.Size(ctx, v.StorageKey)
	if err != nil {
		return nil, err
	}

	if rng == nil {
		r, err := s.blobs.Get(ctx, v.StorageKey)
		if err != nil {
			return nil, err
		}
		return &StreamResult{Video: v, Content: r, ContentLength: total, TotalSize: total}, nil
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

	r, err := s.blobs.GetRange(ctx, v.StorageKey, resolved)
	if err != nil {
		return nil, err
	}
	return &StreamResult{
		Video:         v,
		Content:       r,
		ContentLength: resolved.End - resolved.Start + 1,
		TotalSize:     total,
		Range:         &resolved,
	}, nil
}
