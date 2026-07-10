package localfs

import (
	"io"
	"os"
	"path/filepath"

	"context"

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

func (s *Store) Put(ctx context.Context, key asset.StorageKey, r io.Reader, size int64) (int64, error) {
	p := s.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(p)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	if err != nil {
		return n, err
	}
	return n, f.Sync()
}

func (s *Store) Get(ctx context.Context, key asset.StorageKey) (io.ReadCloser, error) {
	return os.Open(s.path(key))
}

func (s *Store) GetRange(ctx context.Context, key asset.StorageKey, rng port.ByteRange) (io.ReadCloser, error) {
	f, err := os.Open(s.path(key))
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(rng.Start, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}
	length := rng.End - rng.Start + 1
	return &limitedFile{f: f, r: io.LimitReader(f, length)}, nil
}

func (s *Store) Size(ctx context.Context, key asset.StorageKey) (int64, error) {
	fi, err := os.Stat(s.path(key))
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (s *Store) Delete(ctx context.Context, key asset.StorageKey) error {
	err := os.Remove(s.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type limitedFile struct {
	f *os.File
	r io.Reader
}

func (l *limitedFile) Read(p []byte) (int, error) { return l.r.Read(p) }
func (l *limitedFile) Close() error               { return l.f.Close() }
