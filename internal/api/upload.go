package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lsariol/plop/internal/notify"
	"github.com/lsariol/plop/internal/store"
)

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadBytes)
	if err := r.ParseMultipartForm(h.cfg.MaxUploadBytes); err != nil {
		var maxBytesErr *http.MaxBytesError
		var netErr net.Error
		switch {
		case errors.As(err, &maxBytesErr):
			writeError(w, "payload too large", http.StatusRequestEntityTooLarge)
		case errors.As(err, &netErr) && netErr.Timeout():
			writeError(w, "upload timed out", http.StatusRequestTimeout)
		default:
			slog.Warn("parse multipart form", "error", err)
			writeError(w, "invalid upload", http.StatusBadRequest)
		}
		return
	}
	defer r.MultipartForm.RemoveAll()

	id := uuid.New().String()
	text := r.FormValue("text")
	tags := splitTags(r.FormValue("tags"))

	var uploads []store.UploadedFile
	for _, fh := range r.MultipartForm.File["files"] {
		// Normalize cross-platform: strip Windows backslashes then take base component.
		name := path.Base(strings.ReplaceAll(fh.Filename, "\\", "/"))
		if name == "." || name == "" || name == "/" {
			name = "upload"
		}
		f, err := fh.Open()
		if err != nil {
			writeError(w, "could not read uploaded file", http.StatusBadRequest)
			return
		}
		defer f.Close()
		uploads = append(uploads, store.UploadedFile{
			Name:     name,
			MimeType: fh.Header.Get("Content-Type"),
			Reader:   f,
		})
	}

	userID := usernameFromCtx(r)
	payload := store.Payload{
		ID:        id,
		ExpiresAt: time.Now().Add(h.cfg.PayloadTTL),
		UserID:    userID,
		Text:      text,
		Tags:      tags,
	}
	if err := h.store.SavePayload(r.Context(), payload, uploads); err != nil {
		slog.Error("save payload", "id", id, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("upload", "user", userID, "files", len(uploads), "tags", tags, "ip", h.clientIP(r))

	readyJSON, _ := json.Marshal(store.PayloadReadyMsg{
		ID:        payload.ID,
		Text:      payload.Text,
		Tags:      payload.Tags,
		FileCount: len(uploads),
	})
	h.hub.SendToUser(userID, notify.WSMessage{
		Type:    "payload_ready",
		Payload: readyJSON,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"id":                id,
		"desktop_connected": h.hub.HasConnectedClients(userID),
	})
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}
