package handlers

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/services"
)

const userIDHeader = "X-User-ID"

type AvatarHandler struct {
	svc AvatarService
}

func NewAvatarHandler(svc AvatarService) *AvatarHandler {
	return &AvatarHandler{svc: svc}
}

type uploadResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type metadataResponse struct {
	ID         string            `json:"id"`
	UserID     string            `json:"user_id"`
	FileName   string            `json:"file_name"`
	MimeType   string            `json:"mime_type"`
	Size       int64             `json:"size"`
	Dimensions map[string]int    `json:"dimensions,omitempty"`
	Thumbnails []thumbnailItem   `json:"thumbnails"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Status     map[string]string `json:"status,omitempty"`
}

type thumbnailItem struct {
	Size string `json:"size"`
	URL  string `json:"url"`
}

type avatarListItem struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	URL       string    `json:"url"`
	FileName  string    `json:"file_name"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *AvatarHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		writeError(w, http.StatusBadRequest, "Missing X-User-ID header", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.svc.MaxSize()+1024)
	if err := r.ParseMultipartForm(h.svc.MaxSize()); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "File too large", map[string]any{
				"max_size": h.svc.MaxSize(),
			})
			return
		}
		writeError(w, http.StatusBadRequest, "Invalid multipart form", map[string]any{
			"details": err.Error(),
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "File is required", map[string]any{
			"details": `multipart field "file" is required`,
		})
		return
	}
	defer file.Close()

	result, err := h.svc.Upload(r.Context(), services.UploadInput{
		UserID:   userID,
		FileName: header.Filename,
		MimeType: header.Header.Get("Content-Type"),
		Content:  file,
		Size:     header.Size,
	})
	if err != nil {
		writeServiceError(w, err, h.svc.MaxSize())
		return
	}

	writeJSON(w, http.StatusCreated, uploadResponse{
		ID:        result.Avatar.ID,
		UserID:    result.Avatar.UserID,
		URL:       result.URL,
		Status:    result.Status,
		CreatedAt: result.Avatar.CreatedAt.UTC(),
	})
}

func (h *AvatarHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "avatar_id")
	size := r.URL.Query().Get("size")
	img, err := h.svc.GetImage(r.Context(), id, size)
	if err != nil {
		writeServiceError(w, err, h.svc.MaxSize())
		return
	}
	defer img.Body.Close()
	writeImage(w, img)
}

func (h *AvatarHandler) GetByUserID(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	size := r.URL.Query().Get("size")
	img, err := h.svc.GetUserImage(r.Context(), userID, size)
	if err != nil {
		writeServiceError(w, err, h.svc.MaxSize())
		return
	}
	defer img.Body.Close()
	writeImage(w, img)
}

func (h *AvatarHandler) Metadata(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "avatar_id")
	avatar, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err, h.svc.MaxSize())
		return
	}

	thumbs := make([]thumbnailItem, 0, len(avatar.ThumbnailS3Keys))
	for size := range avatar.ThumbnailS3Keys {
		thumbs = append(thumbs, thumbnailItem{
			Size: size,
			URL:  "/api/v1/avatars/" + avatar.ID + "?size=" + size,
		})
	}

	writeJSON(w, http.StatusOK, metadataResponse{
		ID:         avatar.ID,
		UserID:     avatar.UserID,
		FileName:   avatar.FileName,
		MimeType:   avatar.MimeType,
		Size:       avatar.SizeBytes,
		Thumbnails: thumbs,
		CreatedAt:  avatar.CreatedAt.UTC(),
		UpdatedAt:  avatar.UpdatedAt.UTC(),
		Status: map[string]string{
			"upload":     string(avatar.UploadStatus),
			"processing": string(avatar.ProcessingStatus),
		},
	})
}

func (h *AvatarHandler) ListByUserID(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	avatars, err := h.svc.ListByUserID(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err, h.svc.MaxSize())
		return
	}

	items := make([]avatarListItem, 0, len(avatars))
	for _, a := range avatars {
		items = append(items, avatarListItem{
			ID:        a.ID,
			UserID:    a.UserID,
			URL:       "/api/v1/avatars/" + a.ID,
			FileName:  a.FileName,
			MimeType:  a.MimeType,
			Size:      a.SizeBytes,
			Status:    string(a.ProcessingStatus),
			CreatedAt: a.CreatedAt.UTC(),
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *AvatarHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		writeError(w, http.StatusBadRequest, "Missing X-User-ID header", nil)
		return
	}

	id := chi.URLParam(r, "avatar_id")
	if err := h.svc.Delete(r.Context(), id, userID); err != nil {
		writeServiceError(w, err, h.svc.MaxSize())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AvatarHandler) DeleteUserAvatar(w http.ResponseWriter, r *http.Request) {
	headerUserID := r.Header.Get(userIDHeader)
	if headerUserID == "" {
		writeError(w, http.StatusBadRequest, "Missing X-User-ID header", nil)
		return
	}

	pathUserID := chi.URLParam(r, "user_id")
	if err := h.svc.DeleteUserAvatar(r.Context(), pathUserID, headerUserID); err != nil {
		writeServiceError(w, err, h.svc.MaxSize())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeImage(w http.ResponseWriter, img *services.ImageContent) {
	w.Header().Set("Content-Type", img.ContentType)
	w.Header().Set("Cache-Control", "max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, img.Body)
}

func writeServiceError(w http.ResponseWriter, err error, maxSize int64) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrAlreadyDeleted):
		writeError(w, http.StatusNotFound, "Avatar not found", nil)
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "Forbidden", map[string]any{
			"details": "You can only delete your own avatars",
		})
	case errors.Is(err, domain.ErrMissingUserID):
		writeError(w, http.StatusBadRequest, "Missing X-User-ID header", nil)
	case errors.Is(err, domain.ErrInvalidFile):
		writeError(w, http.StatusBadRequest, "Invalid file format", map[string]any{
			"details": "Supported formats: jpeg, png, webp",
		})
	case errors.Is(err, domain.ErrFileTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "File too large", map[string]any{
			"max_size": maxSize,
		})
	case errors.Is(err, domain.ErrEmptyFile):
		writeError(w, http.StatusBadRequest, "Empty file", nil)
	case errors.Is(err, domain.ErrThumbnailNotReady):
		writeError(w, http.StatusNotFound, "Thumbnail not ready", map[string]any{
			"details": "Avatar is still processing",
		})
	case errors.Is(err, domain.ErrInvalidSize):
		writeError(w, http.StatusBadRequest, "Invalid size", map[string]any{
			"details": `Supported sizes: "100x100", "300x300", "original"`,
		})
	default:
		writeError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}
