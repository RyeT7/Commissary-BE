package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"commissary/internal/application/port"
	"commissary/internal/domain/account"
	"commissary/internal/domain/video"
)

type AccountStore struct {
	pool *pgxpool.Pool
}

func NewAccountStore(pool *pgxpool.Pool) *AccountStore { return &AccountStore{pool: pool} }

var _ port.AccountRepository = (*AccountStore)(nil)

func (s *AccountStore) Register(ctx context.Context, u *account.User, c *account.Channel) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, created_at) VALUES ($1, $2, $3, $4)`,
		string(u.ID), u.Email, u.PasswordHash, u.CreatedAt); err != nil {
		return mapUniqueViolation(err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO channels (id, user_id, name, handle, created_at) VALUES ($1, $2, $3, $4, $5)`,
		string(c.ID), string(c.UserID), c.Name, c.Handle, c.CreatedAt); err != nil {
		return mapUniqueViolation(err)
	}
	return tx.Commit(ctx)
}

func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return account.ErrEmailTaken
		case "channels_handle_key":
			return account.ErrHandleTaken
		}
	}
	return err
}

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore { return &UserStore{pool: pool} }

var _ port.UserRepository = (*UserStore)(nil)

func (s *UserStore) FindByID(ctx context.Context, id account.UserID) (*account.User, error) {
	return s.queryUser(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE id = $1`, string(id))
}

func (s *UserStore) FindByEmail(ctx context.Context, email string) (*account.User, error) {
	return s.queryUser(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE email = $1`, email)
}

func (s *UserStore) queryUser(ctx context.Context, sql string, arg string) (*account.User, error) {
	var (
		id, email, hash string
		createdAt       time.Time
	)
	err := s.pool.QueryRow(ctx, sql, arg).Scan(&id, &email, &hash, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, account.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &account.User{ID: account.UserID(id), Email: email, PasswordHash: hash, CreatedAt: createdAt}, nil
}

type ChannelStore struct {
	pool *pgxpool.Pool
}

func NewChannelStore(pool *pgxpool.Pool) *ChannelStore { return &ChannelStore{pool: pool} }

var _ port.ChannelRepository = (*ChannelStore)(nil)

func (s *ChannelStore) FindByID(ctx context.Context, id account.ChannelID) (*account.Channel, error) {
	return s.queryChannel(ctx, `SELECT id, user_id, name, handle, created_at FROM channels WHERE id = $1`, string(id))
}

func (s *ChannelStore) FindByUserID(ctx context.Context, userID account.UserID) (*account.Channel, error) {
	return s.queryChannel(ctx, `SELECT id, user_id, name, handle, created_at FROM channels WHERE user_id = $1`, string(userID))
}

func (s *ChannelStore) queryChannel(ctx context.Context, sql string, arg string) (*account.Channel, error) {
	var (
		id, userID, name, handle string
		createdAt                time.Time
	)
	err := s.pool.QueryRow(ctx, sql, arg).Scan(&id, &userID, &name, &handle, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, account.ErrChannelNotFound
	}
	if err != nil {
		return nil, err
	}
	return &account.Channel{
		ID:        account.ChannelID(id),
		UserID:    account.UserID(userID),
		Name:      name,
		Handle:    handle,
		CreatedAt: createdAt,
	}, nil
}

type VideoStore struct {
	pool *pgxpool.Pool
}

func NewVideoStore(pool *pgxpool.Pool) *VideoStore { return &VideoStore{pool: pool} }

var _ port.VideoRepository = (*VideoStore)(nil)

const videoColumns = `id, channel_id, title, description, storage_key, mime_type, size, thumbnail_key, thumbnail_mime, views, created_at`

func (s *VideoStore) Save(ctx context.Context, v *video.Video) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO videos (`+videoColumns+`)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (id) DO UPDATE SET
		   title = EXCLUDED.title,
		   description = EXCLUDED.description,
		   thumbnail_key = EXCLUDED.thumbnail_key,
		   thumbnail_mime = EXCLUDED.thumbnail_mime`,
		string(v.ID), v.ChannelID, v.Title, v.Description, string(v.StorageKey),
		v.MIMEType, v.Size, string(v.ThumbnailKey), v.ThumbnailMIME, v.Views, v.CreatedAt)
	return err
}

func (s *VideoStore) FindByID(ctx context.Context, id video.ID) (*video.Video, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+videoColumns+` FROM videos WHERE id = $1`, string(id))
	v, err := scanVideo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, video.ErrNotFound
	}
	return v, err
}

func (s *VideoStore) ListRecent(ctx context.Context, limit int) ([]*video.Video, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectVideos(rows)
}

func (s *VideoStore) ListByChannel(ctx context.Context, channelID string) ([]*video.Video, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+videoColumns+` FROM videos WHERE channel_id = $1 ORDER BY created_at DESC`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectVideos(rows)
}

func (s *VideoStore) IncrementViews(ctx context.Context, id video.ID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE videos SET views = views + 1 WHERE id = $1`, string(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return video.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanVideo(row scanner) (*video.Video, error) {
	var (
		id, channelID, title, description, storageKey, mimeType, thumbKey, thumbMIME string
		size, views                                                                  int64
		createdAt                                                                    time.Time
	)
	if err := row.Scan(&id, &channelID, &title, &description, &storageKey, &mimeType, &size, &thumbKey, &thumbMIME, &views, &createdAt); err != nil {
		return nil, err
	}
	return &video.Video{
		ID:            video.ID(id),
		ChannelID:     channelID,
		Title:         title,
		Description:   description,
		StorageKey:    video.StorageKey(storageKey),
		MIMEType:      mimeType,
		Size:          size,
		ThumbnailKey:  video.StorageKey(thumbKey),
		ThumbnailMIME: thumbMIME,
		Views:         views,
		CreatedAt:     createdAt,
	}, nil
}

func collectVideos(rows pgx.Rows) ([]*video.Video, error) {
	result := make([]*video.Video, 0)
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

type SessionStore struct {
	pool *pgxpool.Pool
}

func NewSessionStore(pool *pgxpool.Pool) *SessionStore { return &SessionStore{pool: pool} }

var _ port.SessionRepository = (*SessionStore)(nil)

func (s *SessionStore) Save(ctx context.Context, session port.Session) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)
		 ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at`,
		session.Token, string(session.UserID), session.ExpiresAt)
	return err
}

func (s *SessionStore) Find(ctx context.Context, token string) (port.Session, error) {
	var (
		tok, userID string
		expiresAt   time.Time
	)
	err := s.pool.QueryRow(ctx, `SELECT token, user_id, expires_at FROM sessions WHERE token = $1`, token).
		Scan(&tok, &userID, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.Session{}, account.ErrInvalidLogin
	}
	if err != nil {
		return port.Session{}, err
	}
	return port.Session{Token: tok, UserID: account.UserID(userID), ExpiresAt: expiresAt}, nil
}

func (s *SessionStore) Delete(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	return err
}
