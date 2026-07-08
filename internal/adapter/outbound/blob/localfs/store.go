package localfs

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"commissary/internal/application/port"
	"commissary/internal/domain/asset"
)

type Store struct {
	root string
}

func New(root string) *Store { return &Store{root: root} }

var _ port.BlobStore = (*Store)(nil)

func (s *Store) path(key asset.StorageKey) string {
	return filepath.Join(s.root, filepath.FromSlash(string(key)))
}

func (s *Store) Put(ctx context.Context, key asset.StorageKey, r io.Reader, size int64) error {
	p := s.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return f.Sync()
}

func (s *Store) Get(ctx context.Context, key asset.StorageKey) (io.ReadCloser, error) {
	return os.Open(s.path(key))
}

func (s *Store) Delete(ctx context.Context, key asset.StorageKey) error {
	err := os.Remove(s.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
