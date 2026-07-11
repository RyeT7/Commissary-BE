package asset

import (
	"strings"
	"time"

	"commissary/internal/domain/folder"
)

type Asset struct {
	ID         ID
	OwnerID    string
	Name       string
	MIMEType   MIMEType
	Size       int64
	Checksum   Checksum
	StorageKey StorageKey
	FolderID   *folder.ID
	Tags       []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Kind string

const (
	KindDocument Kind = "document"
	KindImage    Kind = "image"
	KindVideo    Kind = "video"
	KindAudio    Kind = "audio"
	KindOther    Kind = "other"
)

func KindFromMIME(m MIMEType) Kind {
	switch {
	case strings.HasPrefix(string(m), "video/"):
		return KindVideo
	case strings.HasPrefix(string(m), "audio/"):
		return KindAudio
	case strings.HasPrefix(string(m), "image/"):
		return KindImage
	case m == "":
		return KindOther
	default:
		return KindDocument
	}
}

func (a *Asset) Kind() Kind { return KindFromMIME(a.MIMEType) }
