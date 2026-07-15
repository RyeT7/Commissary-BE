package video

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

type ID string

type StorageKey string

type Video struct {
	ID            ID
	ChannelID     string
	Title         string
	Description   string
	StorageKey    StorageKey
	MIMEType      string
	Size          int64
	ThumbnailKey  StorageKey
	ThumbnailMIME string
	Views         int64
	CreatedAt     time.Time
}

func NewID() ID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return ID(hex.EncodeToString(b[:]))
}

var (
	ErrNotFound     = errors.New("video: not found")
	ErrInvalidTitle = errors.New("video: title required")
	ErrEmptyContent = errors.New("video: empty content")
	ErrInvalidRange = errors.New("video: invalid byte range")
)
