package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"commissary/internal/application/port"
	"commissary/internal/domain/account"
)

const (
	sessionDuration = 30 * 24 * time.Hour
	maxHandleTries  = 5
)

type Service struct {
	accounts port.AccountRepository
	users    port.UserRepository
	sessions port.SessionRepository
}

func NewService(accounts port.AccountRepository, users port.UserRepository, sessions port.SessionRepository) *Service {
	return &Service{accounts: accounts, users: users, sessions: sessions}
}

type SignupCommand struct {
	Email       string
	Password    string
	ChannelName string
}

func (s *Service) Signup(ctx context.Context, cmd SignupCommand) (*account.User, *account.Channel, string, error) {
	email := strings.TrimSpace(strings.ToLower(cmd.Email))
	if !validEmail(email) {
		return nil, nil, "", account.ErrInvalidEmail
	}
	if len(cmd.Password) < 8 {
		return nil, nil, "", account.ErrWeakPassword
	}
	name := strings.TrimSpace(cmd.ChannelName)
	if name == "" {
		return nil, nil, "", account.ErrInvalidName
	}

	hash, err := hashPassword(cmd.Password)
	if err != nil {
		return nil, nil, "", err
	}

	now := time.Now().UTC()
	user := &account.User{
		ID:           account.NewUserID(),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    now,
	}

	var channel *account.Channel
	for try := 0; try < maxHandleTries; try++ {
		channel = &account.Channel{
			ID:        account.NewChannelID(),
			UserID:    user.ID,
			Name:      name,
			Handle:    makeHandle(name),
			CreatedAt: now,
		}
		err = s.accounts.Register(ctx, user, channel)
		if errors.Is(err, account.ErrHandleTaken) {
			continue
		}
		break
	}
	if err != nil {
		return nil, nil, "", err
	}

	token, err := s.newSession(ctx, user.ID)
	if err != nil {
		return nil, nil, "", err
	}
	return user, channel, token, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, account.ErrUserNotFound) {
		return "", account.ErrInvalidLogin
	}
	if err != nil {
		return "", err
	}
	if !verifyPassword(password, user.PasswordHash) {
		return "", account.ErrInvalidLogin
	}
	return s.newSession(ctx, user.ID)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.Delete(ctx, token)
}

func (s *Service) Authenticate(ctx context.Context, token string) (*account.User, error) {
	session, err := s.sessions.Find(ctx, token)
	if err != nil {
		return nil, err
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.sessions.Delete(ctx, token)
		return nil, account.ErrInvalidLogin
	}
	return s.users.FindByID(ctx, session.UserID)
}

func (s *Service) newSession(ctx context.Context, userID account.UserID) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b[:])
	session := port.Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(sessionDuration),
	}
	if err := s.sessions.Save(ctx, session); err != nil {
		return "", err
	}
	return token, nil
}

func validEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.IndexByte(email[at+1:], '.') > 0
}

func makeHandle(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			sb.WriteByte('-')
		}
	}
	handle := strings.Trim(sb.String(), "-")
	var suffix [3]byte
	_, _ = rand.Read(suffix[:])
	if handle == "" {
		handle = "channel"
	}
	return handle + "-" + hex.EncodeToString(suffix[:])
}
