package account

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

type UserID string

type ChannelID string

type User struct {
	ID           UserID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Channel struct {
	ID        ChannelID
	UserID    UserID
	Name      string
	Handle    string
	CreatedAt time.Time
}

func NewUserID() UserID {
	return UserID(newID())
}

func NewChannelID() ChannelID {
	return ChannelID(newID())
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

var (
	ErrUserNotFound    = errors.New("account: user not found")
	ErrChannelNotFound = errors.New("account: channel not found")
	ErrEmailTaken      = errors.New("account: email already registered")
	ErrHandleTaken     = errors.New("account: channel handle already taken")
	ErrInvalidEmail    = errors.New("account: invalid email")
	ErrInvalidName     = errors.New("account: channel name required")
	ErrWeakPassword    = errors.New("account: password too short")
	ErrInvalidLogin    = errors.New("account: invalid email or password")
)
