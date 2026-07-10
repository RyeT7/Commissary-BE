package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	appasset "commissary/internal/application/asset"
	"commissary/internal/application/port"
	domain "commissary/internal/domain/asset"
)

type Handler struct {
	assets *appasset.Service
}

func NewHandler(assets *appasset.Service) *Handler {
	return &Handler{assets: assets}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /assets", h.upload)
	mux.HandleFunc("GET /assets/{id}", h.getMetadata)
	mux.HandleFunc("GET /assets/{id}/content", h.streamContent)
	return mux
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	name := r.Header.Get("X-Asset-Name")
	if name == "" {
		name = r.URL.Query().Get("name")
	}

	a, err := h.assets.Upload(r.Context(), appasset.UploadCommand{
		OwnerID:  r.URL.Query().Get("owner_id"),
		Name:     name,
		MIMEType: domain.MIMEType(r.Header.Get("Content-Type")),
		Size:     r.ContentLength,
		Content:  r.Body,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toDTO(a))
}

func (h *Handler) getMetadata(w http.ResponseWriter, r *http.Request) {
	id := domain.ID(r.PathValue("id"))
	a, err := h.assets.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toDTO(a))
}

func (h *Handler) streamContent(w http.ResponseWriter, r *http.Request) {
	id := domain.ID(r.PathValue("id"))

	rng, err := parseRange(r.Header.Get("Range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
		return
	}

	result, err := h.assets.Stream(r.Context(), id, rng)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRange) {
			http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		writeError(w, err)
		return
	}
	defer result.Content.Close()

	w.Header().Set("Accept-Ranges", "bytes")
	if ct := string(result.Asset.MIMEType); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Content-Length", strconv.FormatInt(result.ContentLength, 10))

	if result.Range != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", result.Range.Start, result.Range.End, result.TotalSize))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	_, _ = io.Copy(w, result.Content)
}

func parseRange(header string) (*port.ByteRange, error) {
	if header == "" {
		return nil, nil
	}
	const prefix = "bytes="
	if !strings.HasPrefix(header, prefix) {
		return nil, errors.New("unsupported range unit")
	}
	spec := strings.TrimPrefix(header, prefix)
	if strings.Contains(spec, ",") {
		return nil, errors.New("multiple ranges not supported")
	}

	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed range")
	}
	startStr, endStr := parts[0], parts[1]

	if startStr == "" {
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return nil, errors.New("malformed range")
		}
		return &port.ByteRange{Start: -n, End: -1}, nil
	}

	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return nil, errors.New("malformed range")
	}
	if endStr == "" {
		return &port.ByteRange{Start: start, End: -1}, nil
	}
	end, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil || end < start {
		return nil, errors.New("malformed range")
	}
	return &port.ByteRange{Start: start, End: end}, nil
}

type assetDTO struct {
	ID       string   `json:"id"`
	OwnerID  string   `json:"owner_id"`
	Name     string   `json:"name"`
	MIMEType string   `json:"mime_type"`
	Kind     string   `json:"kind"`
	Size     int64    `json:"size"`
	Checksum string   `json:"checksum"`
	Tags     []string `json:"tags"`
}

func toDTO(a *domain.Asset) assetDTO {
	return assetDTO{
		ID:       string(a.ID),
		OwnerID:  a.OwnerID,
		Name:     a.Name,
		MIMEType: string(a.MIMEType),
		Kind:     string(a.Kind()),
		Size:     a.Size,
		Checksum: string(a.Checksum),
		Tags:     a.Tags,
	}
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrEmptyBlob), errors.Is(err, domain.ErrInvalidRange):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
