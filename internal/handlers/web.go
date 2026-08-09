package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/qutaq/GophProfile/internal/services"
)

type WebHandler struct {
	svc       *services.AvatarService
	templates *template.Template
}

func NewWebHandler(svc *services.AvatarService, webDir string) (*WebHandler, error) {
	pattern := filepath.Join(webDir, "templates", "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}
	return &WebHandler{svc: svc, templates: tmpl}, nil
}

type uploadPageData struct {
	UserID string
}

type galleryItem struct {
	ID       string
	FileName string
	Status   string
}

type galleryPageData struct {
	UserID     string
	UploadedID string
	Avatars    []galleryItem
}

func (h *WebHandler) UploadPage(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "demo-user"
	}
	h.render(w, "upload.html", uploadPageData{UserID: userID})
}

func (h *WebHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		userID = r.FormValue("user_id")
	}
	if strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "Missing user id", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.svc.MaxSize()+1024)
	if err := r.ParseMultipartForm(h.svc.MaxSize()); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid multipart form", map[string]any{"details": err.Error()})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "File is required", nil)
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
		// Reuse API error mapping via a temporary AvatarHandler.
		(&AvatarHandler{svc: h.svc}).writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      result.Avatar.ID,
		"user_id": result.Avatar.UserID,
		"url":     result.URL,
		"status":  result.Status,
	})
}

func (h *WebHandler) Gallery(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		http.Redirect(w, r, "/web/upload", http.StatusFound)
		return
	}

	avatars, err := h.svc.ListByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load gallery", nil)
		return
	}

	items := make([]galleryItem, 0, len(avatars))
	for _, a := range avatars {
		items = append(items, galleryItem{
			ID:       a.ID,
			FileName: a.FileName,
			Status:   string(a.ProcessingStatus),
		})
	}

	h.render(w, "gallery.html", galleryPageData{
		UserID:     userID,
		UploadedID: r.URL.Query().Get("uploaded"),
		Avatars:    items,
	})
}

func (h *WebHandler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
