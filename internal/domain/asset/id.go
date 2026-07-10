package asset

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID() ID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return ID(hex.EncodeToString(b[:]))
}
