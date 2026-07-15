package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	authapp "commissary/internal/application/auth"
	channelapp "commissary/internal/application/channel"
	"commissary/internal/application/port"
	videoapp "commissary/internal/application/video"
	"commissary/internal/domain/account"
	domainvideo "commissary/internal/domain/video"
)

const sessionCookie = "session"

type Options struct {
	AllowedOrigin  string
	SecureCookies  bool
	MaxUploadBytes int64
	Health         func(ctx context.Context) error
	IsUnavailable  func(error) bool
	Idempotency    port.IdempotencyStore
}

type Handler struct {
	auth     *authapp.Service
	videos   *videoapp.Service
	channels *channelapp.Service
	opts     Options
}

func NewHandler(auth *authapp.Service, videos *videoapp.Service, channels *channelapp.Service, opts Options) *Handler {
	if opts.Health == nil {
		opts.Health = func(context.Context) error { return nil }
	}
	if opts.IsUnavailable == nil {
		opts.IsUnavailable = func(error) bool { return false }
	}
	if opts.MaxUploadBytes <= 0 {
		opts.MaxUploadBytes = 2 << 30
	}
	return &Handler{auth: auth, videos: videos, channels: channels, opts: opts}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)

	mux.HandleFunc("POST /auth/signup", h.signup)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /auth/me", h.me)

	mux.HandleFunc("POST /videos", h.upload)
	mux.HandleFunc("GET /videos", h.feed)
	mux.HandleFunc("GET /videos/{id}", h.getVideo)
	mux.HandleFunc("GET /videos/{id}/stream", h.stream)
	mux.HandleFunc("GET /videos/{id}/thumbnail", h.thumbnail)

	mux.HandleFunc("GET /channels/{id}", h.channelPage)

	return recoverMW(logMW(h.corsMW(h.idempotencyMW(mux))))
}

func (h *Handler) idempotencyMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if h.opts.Idempotency == nil || r.Method != http.MethodPost || key == "" {
			next.ServeHTTP(w, r)
			return
		}

		scope := idempotencyScope(r)
		rec, found, err := h.opts.Idempotency.Begin(r.Context(), key, scope)
		if err != nil {
			if h.opts.IsUnavailable(err) {
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			} else {
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}

		if found {
			if rec.Scope != scope {
				http.Error(w, "idempotency key already used", http.StatusConflict)
				return
			}
			if !rec.Completed {
				http.Error(w, "a request with this idempotency key is still in progress", http.StatusConflict)
				return
			}
			w.Header().Set("Idempotent-Replayed", "true")
			if len(rec.Body) > 0 {
				w.Header().Set("Content-Type", "application/json")
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(rec.Body)))
			w.WriteHeader(rec.Status)
			_, _ = w.Write(rec.Body)
			return
		}

		cap := &captureWriter{ResponseWriter: w}
		next.ServeHTTP(cap, r)
		status := cap.status
		if status == 0 {
			status = http.StatusOK
		}
		if status >= 500 {
			_ = h.opts.Idempotency.Discard(r.Context(), key)
		} else {
			_ = h.opts.Idempotency.Complete(r.Context(), key, status, cap.body.Bytes())
		}
		w.Header().Set("Content-Length", strconv.Itoa(cap.body.Len()))
		w.WriteHeader(status)
		_, _ = w.Write(cap.body.Bytes())
	})
}

func idempotencyScope(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil {
		sum := sha256.Sum256([]byte(c.Value))
		return hex.EncodeToString(sum[:])
	}
	return "anon"
}

type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	wrote  bool
}

func (c *captureWriter) WriteHeader(code int) {
	if !c.wrote {
		c.status = code
		c.wrote = true
	}
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if !c.wrote {
		c.status = http.StatusOK
		c.wrote = true
	}
	return c.body.Write(b)
}

func recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic recovered", "err", v, "path", r.URL.Path)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method, "path", r.URL.Path,
			"status", rec.status, "dur", time.Since(start).String())
	})
}

func (h *Handler) corsMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", h.opts.AllowedOrigin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	if err := h.opts.Health(r.Context()); err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) requireUser(w http.ResponseWriter, r *http.Request) (*account.User, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return nil, false
	}
	user, err := h.auth.Authenticate(r.Context(), c.Value)
	if err != nil {
		if h.opts.IsUnavailable(err) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return nil, false
		}
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return nil, false
	}
	return user, true
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.opts.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.opts.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

type signupRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	ChannelName string `json:"channel_name"`
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	user, channel, token, err := h.auth.Signup(r.Context(), authapp.SignupCommand{
		Email:       req.Email,
		Password:    req.Password,
		ChannelName: req.ChannelName,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusCreated, sessionDTO(user, channel))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.writeError(w, err)
		return
	}
	user, err := h.auth.Authenticate(r.Context(), token)
	if err != nil {
		h.writeError(w, err)
		return
	}
	channel, err := h.channels.GetByUser(r.Context(), user.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, sessionDTO(user, channel))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = h.auth.Logout(r.Context(), c.Value)
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	channel, err := h.channels.GetByUser(r.Context(), user.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionDTO(user, channel))
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	channel, err := h.channels.GetByUser(r.Context(), user.ID)
	if err != nil {
		h.writeError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.opts.MaxUploadBytes)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "upload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		http.Error(w, "video file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(mimeType, "video/") {
		http.Error(w, "the video must be a video/* file", http.StatusUnsupportedMediaType)
		return
	}

	cmd := videoapp.UploadCommand{
		ChannelID:   string(channel.ID),
		Title:       strings.TrimSpace(r.FormValue("title")),
		Description: strings.TrimSpace(r.FormValue("description")),
		MIMEType:    mimeType,
		Size:        header.Size,
		Content:     file,
	}

	if thumb, thumbHeader, err := r.FormFile("thumbnail"); err == nil {
		defer thumb.Close()
		thumbMIME := thumbHeader.Header.Get("Content-Type")
		if !strings.HasPrefix(thumbMIME, "image/") {
			http.Error(w, "the thumbnail must be an image/* file", http.StatusUnsupportedMediaType)
			return
		}
		cmd.Thumbnail = thumb
		cmd.ThumbnailMIME = thumbMIME
	}

	v, err := h.videos.Upload(r.Context(), cmd)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.videoDTO(v, channel))
}

func (h *Handler) feed(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	videos, err := h.videos.Feed(r.Context(), limit)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.videoListDTO(r, videos))
}

func (h *Handler) getVideo(w http.ResponseWriter, r *http.Request) {
	id := domainvideo.ID(r.PathValue("id"))
	v, err := h.videos.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	if err := h.videos.RecordView(r.Context(), id); err == nil {
		v.Views++
	}
	channel := h.lookupChannel(r, v.ChannelID)
	writeJSON(w, http.StatusOK, h.videoDTO(v, channel))
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	id := domainvideo.ID(r.PathValue("id"))
	rng, err := parseRange(r.Header.Get("Range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
		return
	}
	result, err := h.videos.Stream(r.Context(), id, rng)
	if err != nil {
		if errors.Is(err, domainvideo.ErrInvalidRange) {
			http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		h.writeError(w, err)
		return
	}
	defer result.Content.Close()

	w.Header().Set("Accept-Ranges", "bytes")
	if ct := result.Video.MIMEType; ct != "" {
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

func (h *Handler) thumbnail(w http.ResponseWriter, r *http.Request) {
	id := domainvideo.ID(r.PathValue("id"))
	v, err := h.videos.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	reader, mime, err := h.videos.OpenThumbnail(r.Context(), v)
	if err != nil {
		http.Error(w, "no thumbnail", http.StatusNotFound)
		return
	}
	defer reader.Close()
	if mime == "" {
		mime = "image/jpeg"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, reader)
}

func (h *Handler) channelPage(w http.ResponseWriter, r *http.Request) {
	id := account.ChannelID(r.PathValue("id"))
	c, err := h.channels.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	videos, err := h.videos.ByChannel(r.Context(), string(c.ID))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, channelPageDTO{
		Channel: channelDTO(c),
		Videos:  h.videoListDTO(r, videos),
	})
}

func (h *Handler) lookupChannel(r *http.Request, channelID string) *account.Channel {
	c, err := h.channels.Get(r.Context(), account.ChannelID(channelID))
	if err != nil {
		return nil
	}
	return c
}

func (h *Handler) videoListDTO(r *http.Request, videos []*domainvideo.Video) []videoResponse {
	cache := make(map[string]*account.Channel)
	out := make([]videoResponse, 0, len(videos))
	for _, v := range videos {
		c, ok := cache[v.ChannelID]
		if !ok {
			c = h.lookupChannel(r, v.ChannelID)
			cache[v.ChannelID] = c
		}
		out = append(out, h.videoDTO(v, c))
	}
	return out
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, account.ErrUserNotFound), errors.Is(err, account.ErrChannelNotFound), errors.Is(err, domainvideo.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, account.ErrInvalidLogin):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, account.ErrEmailTaken):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, account.ErrInvalidEmail), errors.Is(err, account.ErrWeakPassword),
		errors.Is(err, account.ErrInvalidName), errors.Is(err, domainvideo.ErrInvalidTitle),
		errors.Is(err, domainvideo.ErrEmptyContent), errors.Is(err, domainvideo.ErrInvalidRange):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case h.opts.IsUnavailable(err):
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
