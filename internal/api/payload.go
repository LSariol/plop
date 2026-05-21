package api

import (
	"archive/zip"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lsariol/plop/internal/store"
)

func (h *Handler) GetPayload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := desktopUserFromCtx(r)

	payload, err := h.store.GetPayload(r.Context(), id, userID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, "payload not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("get payload", "id", id, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="payload.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, f := range payload.Files {
		path := filepath.Join(h.cfg.PayloadDir, id, "files", f.Name)
		if err := addFileToZip(zw, f.Name, path); err != nil {
			slog.Error("zip file", "file", f.Name, "payload", id, "error", err)
			return
		}
	}
}

func (h *Handler) AckPayload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := desktopUserFromCtx(r)

	if err := h.store.AckPayload(r.Context(), id, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, "payload not found", http.StatusNotFound)
			return
		}
		slog.Error("ack payload", "id", id, "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("payload acknowledged", "id", id)
	w.WriteHeader(http.StatusOK)
}

func addFileToZip(zw *zip.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fw, err := zw.Create(name)
	if err != nil {
		return err
	}

	_, err = io.Copy(fw, f)
	return err
}
