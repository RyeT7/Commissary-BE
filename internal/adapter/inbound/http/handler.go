package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	appasset "commissary/internal/application/asset"
	appdirectory "commissary/internal/application/directory"
	"commissary/internal/application/port"
	domain "commissary/internal/domain/asset"
	domainfolder "commissary/internal/domain/folder"
)

const defaultOwnerID = "local"

type Handler struct {
	assets    *appasset.Service
	directory *appdirectory.Service
}

func NewHandler(assets *appasset.Service, directory *appdirectory.Service) *Handler {
	return &Handler{assets: assets, directory: directory}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /assets", h.upload)
	mux.HandleFunc("GET /assets/{id}", h.getMetadata)
	mux.HandleFunc("GET /assets/{id}/content", h.streamContent)
	mux.HandleFunc("GET /folders/{id}", h.getFolder)
	mux.HandleFunc("GET /directory", h.listDirectory)
	mux.HandleFunc("POST /directory/folders", h.mkdir)
	mux.HandleFunc("DELETE /directory/entries", h.removeEntry)
	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Asset-Name, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ownerIDFrom(r *http.Request) string {
	if id := r.URL.Query().Get("owner_id"); id != "" {
		return id
	}
	return defaultOwnerID
}

func folderIDFrom(raw string) *domainfolder.ID {
	if raw == "" {
		return nil
	}
	id := domainfolder.ID(raw)
	return &id
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	name := r.Header.Get("X-Asset-Name")
	if name == "" {
		name = r.URL.Query().Get("name")
	}

	a, err := h.assets.Upload(r.Context(), appasset.UploadCommand{
		OwnerID:  ownerIDFrom(r),
		FolderID: folderIDFrom(r.URL.Query().Get("folder_id")),
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

func (h *Handler) getFolder(w http.ResponseWriter, r *http.Request) {
	id := domainfolder.ID(r.PathValue("id"))
	f, err := h.directory.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toFolderDTO(f))
}

func (h *Handler) listDirectory(w http.ResponseWriter, r *http.Request) {
	parentID := folderIDFrom(r.URL.Query().Get("folder_id"))

	listing, err := h.directory.List(r.Context(), ownerIDFrom(r), parentID)
	if err != nil {
		writeError(w, err)
		return
	}

	dto := directoryDTO{
		Folders: make([]folderDTO, 0, len(listing.Folders)),
		Assets:  make([]assetDTO, 0, len(listing.Assets)),
	}
	for _, f := range listing.Folders {
		dto.Folders = append(dto.Folders, toFolderDTO(f))
	}
	for _, a := range listing.Assets {
		dto.Assets = append(dto.Assets, toDTO(a))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto)
}

type mkdirRequest struct {
	FolderID string `json:"folder_id"`
	Name     string `json:"name"`
}

func (h *Handler) mkdir(w http.ResponseWriter, r *http.Request) {
	var req mkdirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	f, err := h.directory.Mkdir(r.Context(), ownerIDFrom(r), folderIDFrom(req.FolderID), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toFolderDTO(f))
}

type removeEntryRequest struct {
	FolderID string `json:"folder_id"`
	Name     string `json:"name"`
}

func (h *Handler) removeEntry(w http.ResponseWriter, r *http.Request) {
	var req removeEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.directory.Remove(r.Context(), ownerIDFrom(r), folderIDFrom(req.FolderID), req.Name); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
	ID        string   `json:"id"`
	OwnerID   string   `json:"owner_id"`
	Name      string   `json:"name"`
	MIMEType  string   `json:"mime_type"`
	Kind      string   `json:"kind"`
	Size      int64    `json:"size"`
	Checksum  string   `json:"checksum"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

func toDTO(a *domain.Asset) assetDTO {
	return assetDTO{
		ID:        string(a.ID),
		OwnerID:   a.OwnerID,
		Name:      a.Name,
		MIMEType:  string(a.MIMEType),
		Kind:      string(a.Kind()),
		Size:      a.Size,
		Checksum:  string(a.Checksum),
		Tags:      a.Tags,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
		UpdatedAt: a.UpdatedAt.Format(time.RFC3339),
	}
}

type folderDTO struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id,omitempty"`
	OwnerID   string `json:"owner_id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toFolderDTO(f *domainfolder.Folder) folderDTO {
	dto := folderDTO{
		ID:        string(f.ID),
		OwnerID:   f.OwnerID,
		Name:      f.Name,
		Kind:      "folder",
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
		UpdatedAt: f.UpdatedAt.Format(time.RFC3339),
	}
	if f.ParentID != nil {
		dto.ParentID = string(*f.ParentID)
	}
	return dto
}

type directoryDTO struct {
	Folders []folderDTO `json:"folders"`
	Assets  []assetDTO  `json:"assets"`
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domainfolder.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrEmptyBlob), errors.Is(err, domain.ErrInvalidRange),
		errors.Is(err, domainfolder.ErrInvalidName), errors.Is(err, domainfolder.ErrExists), errors.Is(err, domainfolder.ErrNotEmpty):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
