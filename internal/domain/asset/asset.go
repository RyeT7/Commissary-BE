package asset

import "time"

type Asset struct {
	ID         ID
	OwnerID    string
	Name       string
	MIMEType   MIMEType
	Size       int64
	Checksum   Checksum
	StorageKey StorageKey
	FolderID   *ID
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
