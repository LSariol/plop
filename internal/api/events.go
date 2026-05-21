package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	userID := usernameFromCtx(r)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if behind proxy

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Disable the server's global WriteTimeout for this long-lived connection.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.Warn("could not clear SSE write deadline", "error", err)
	}

	// Send an initial comment to establish the connection and flush headers.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	events, unsubscribe := h.sseHub.Subscribe(userID)
	defer unsubscribe()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-events:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: delivered\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
