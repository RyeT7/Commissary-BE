package video

import (
	"context"
	"io"
	"strings"
	"testing"

	"commissary/internal/application/port"
	domain "commissary/internal/domain/video"
)

type fakeRepo struct {
	v *domain.Video
}

func (f *fakeRepo) Save(context.Context, *domain.Video) error { return nil }
func (f *fakeRepo) FindByID(context.Context, domain.ID) (*domain.Video, error) {
	return f.v, nil
}
func (f *fakeRepo) ListRecent(context.Context, int) ([]*domain.Video, error)       { return nil, nil }
func (f *fakeRepo) ListByChannel(context.Context, string) ([]*domain.Video, error) { return nil, nil }
func (f *fakeRepo) IncrementViews(context.Context, domain.ID) error                { return nil }

type fakeBlobs struct {
	size int64
}

func (b *fakeBlobs) Put(context.Context, domain.StorageKey, io.Reader, int64) (int64, error) {
	return 0, nil
}
func (b *fakeBlobs) Get(context.Context, domain.StorageKey) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (b *fakeBlobs) GetRange(context.Context, domain.StorageKey, port.ByteRange) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (b *fakeBlobs) Size(context.Context, domain.StorageKey) (int64, error) { return b.size, nil }
func (b *fakeBlobs) Delete(context.Context, domain.StorageKey) error        { return nil }

func newService(size int64) *Service {
	return NewService(&fakeRepo{v: &domain.Video{ID: "v1", StorageKey: "videos/v1"}}, &fakeBlobs{size: size})
}

func TestStreamFullContent(t *testing.T) {
	res, err := newService(1000).Stream(context.Background(), "v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Range != nil {
		t.Error("expected nil Range for full content")
	}
	if res.ContentLength != 1000 || res.TotalSize != 1000 {
		t.Errorf("got len=%d total=%d", res.ContentLength, res.TotalSize)
	}
}

func TestStreamRanges(t *testing.T) {
	cases := []struct {
		name                        string
		in                          port.ByteRange
		wantStart, wantEnd, wantLen int64
	}{
		{"closed", port.ByteRange{Start: 0, End: 99}, 0, 99, 100},
		{"open-ended", port.ByteRange{Start: 950, End: -1}, 950, 999, 50},
		{"suffix", port.ByteRange{Start: -100, End: -1}, 900, 999, 100},
		{"suffix-larger-than-file", port.ByteRange{Start: -5000, End: -1}, 0, 999, 1000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := c.in
			res, err := newService(1000).Stream(context.Background(), "v1", &in)
			if err != nil {
				t.Fatal(err)
			}
			if res.Range.Start != c.wantStart || res.Range.End != c.wantEnd {
				t.Errorf("range = %d-%d, want %d-%d", res.Range.Start, res.Range.End, c.wantStart, c.wantEnd)
			}
			if res.ContentLength != c.wantLen {
				t.Errorf("len = %d, want %d", res.ContentLength, c.wantLen)
			}
		})
	}
}

func TestStreamInvalidRange(t *testing.T) {
	in := port.ByteRange{Start: 2000, End: 3000}
	if _, err := newService(1000).Stream(context.Background(), "v1", &in); err != domain.ErrInvalidRange {
		t.Errorf("want ErrInvalidRange, got %v", err)
	}
	empty := port.ByteRange{Start: 0, End: 0}
	if _, err := newService(0).Stream(context.Background(), "v1", &empty); err != domain.ErrInvalidRange {
		t.Errorf("want ErrInvalidRange for empty blob, got %v", err)
	}
}
