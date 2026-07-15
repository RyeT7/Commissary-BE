package port

import (
	"context"
	"io"
	"time"

	"commissary/internal/domain/account"
	"commissary/internal/domain/video"
)

type ByteRange struct {
	Start int64
	End   int64
}

type BlobStore interface {
	Put(ctx context.Context, key video.StorageKey, r io.Reader, size int64) (written int64, err error)
	Get(ctx context.Context, key video.StorageKey) (io.ReadCloser, error)
	GetRange(ctx context.Context, key video.StorageKey, rng ByteRange) (io.ReadCloser, error)
	Size(ctx context.Context, key video.StorageKey) (int64, error)
	Delete(ctx context.Context, key video.StorageKey) error
}

type AccountRepository interface {
	Register(ctx context.Context, u *account.User, c *account.Channel) error
}

type UserRepository interface {
	FindByID(ctx context.Context, id account.UserID) (*account.User, error)
	FindByEmail(ctx context.Context, email string) (*account.User, error)
}

type ChannelRepository interface {
	FindByID(ctx context.Context, id account.ChannelID) (*account.Channel, error)
	FindByUserID(ctx context.Context, userID account.UserID) (*account.Channel, error)
}

type VideoRepository interface {
	Save(ctx context.Context, v *video.Video) error
	FindByID(ctx context.Context, id video.ID) (*video.Video, error)
	ListRecent(ctx context.Context, limit int) ([]*video.Video, error)
	ListByChannel(ctx context.Context, channelID string) ([]*video.Video, error)
	IncrementViews(ctx context.Context, id video.ID) error
}

type Session struct {
	Token     string
	UserID    account.UserID
	ExpiresAt time.Time
}

type SessionRepository interface {
	Save(ctx context.Context, s Session) error
	Find(ctx context.Context, token string) (Session, error)
	Delete(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) error
}

type IdempotencyRecord struct {
	Scope     string
	Status    int
	Body      []byte
	Completed bool
}

type IdempotencyStore interface {
	Begin(ctx context.Context, key, scope string) (rec IdempotencyRecord, found bool, err error)
	Complete(ctx context.Context, key string, status int, body []byte) error
	Discard(ctx context.Context, key string) error
	DeleteExpired(ctx context.Context) error
}
