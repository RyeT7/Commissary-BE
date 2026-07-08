package asset

import "errors"

var (
	ErrNotFound    = errors.New("asset: not found")
	ErrInvalidName = errors.New("asset: invalid name")
	ErrEmptyBlob   = errors.New("asset: empty content")
)
