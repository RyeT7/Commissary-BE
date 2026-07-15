package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"commissary/internal/application/port"
)

type fakeIdempotency struct {
	mu   sync.Mutex
	recs map[string]port.IdempotencyRecord
}

func newFakeIdempotency() *fakeIdempotency {
	return &fakeIdempotency{recs: map[string]port.IdempotencyRecord{}}
}

func (f *fakeIdempotency) Begin(_ context.Context, key, scope string) (port.IdempotencyRecord, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rec, ok := f.recs[key]; ok {
		return rec, true, nil
	}
	f.recs[key] = port.IdempotencyRecord{Scope: scope}
	return port.IdempotencyRecord{}, false, nil
}

func (f *fakeIdempotency) Complete(_ context.Context, key string, status int, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec := f.recs[key]
	rec.Status = status
	rec.Body = append([]byte(nil), body...)
	rec.Completed = true
	f.recs[key] = rec
	return nil
}

func (f *fakeIdempotency) Discard(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.recs, key)
	return nil
}

func (f *fakeIdempotency) DeleteExpired(context.Context) error { return nil }

func testHandler(store port.IdempotencyStore) (*Handler, *int) {
	calls := 0
	h := &Handler{opts: Options{
		Idempotency:   store,
		IsUnavailable: func(error) bool { return false },
	}}
	return h, &calls
}

func TestIdempotencyReplaysWithoutRerunning(t *testing.T) {
	store := newFakeIdempotency()
	h, calls := testHandler(store)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	})
	mw := h.idempotencyMW(next)

	do := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/videos", nil)
		req.Header.Set("Idempotency-Key", "key-123")
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		return rr
	}

	first := do()
	second := do()

	if *calls != 1 {
		t.Fatalf("handler ran %d times, want 1 (second call should replay)", *calls)
	}
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("codes: first=%d second=%d, want 201/201", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("bodies differ: %q vs %q", first.Body.String(), second.Body.String())
	}
	if second.Header().Get("Idempotent-Replayed") != "true" {
		t.Error("replayed response missing Idempotent-Replayed header")
	}
}

func TestIdempotencyIgnoredWithoutKey(t *testing.T) {
	store := newFakeIdempotency()
	h, calls := testHandler(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		w.WriteHeader(http.StatusCreated)
	})
	mw := h.idempotencyMW(next)

	for range 3 {
		req := httptest.NewRequest(http.MethodPost, "/videos", nil)
		mw.ServeHTTP(httptest.NewRecorder(), req)
	}
	if *calls != 3 {
		t.Fatalf("handler ran %d times, want 3 (no key = no dedup)", *calls)
	}
}
