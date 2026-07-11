package folder

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

type ID string

type Folder struct {
	ID        ID
	ParentID  *ID
	OwnerID   string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewID() ID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return ID(hex.EncodeToString(b[:]))
}

var (
	ErrNotFound    = errors.New("folder: not found")
	ErrInvalidName = errors.New("folder: invalid name")
	ErrNotEmpty    = errors.New("folder: not empty")
	ErrExists      = errors.New("folder: already exists")
)
