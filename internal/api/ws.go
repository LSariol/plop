package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lsariol/plop/internal/notify"
	"github.com/lsariol/plop/internal/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") == "https://plop.mobasity.com"
	},
}

func (h *Handler) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade", "error", err)
		return
	}

	clientID := uuid.New().String()
	userID := desktopUserFromCtx(r)
	h.hub.Register(clientID, userID, conn)
	slog.Info("desktop connected", "client_id", clientID, "user", userID)
	defer func() {
		h.hub.Unregister(clientID)
		slog.Info("desktop disconnected", "client_id", clientID, "user", userID)
	}()

	// Pings are sent by writePump's ticker. Reset the read deadline on each pong.
	conn.SetReadDeadline(time.Now().Add(40 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(40 * time.Second))
		return nil
	})

	for {
		var msg notify.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				slog.Warn("ws unexpected close", "client_id", clientID, "error", err)
			}
			return
		}

		switch msg.Type {
		case "hello":
			payloads, err := h.store.PendingPayloads(r.Context(), userID)
			if err != nil {
				slog.Error("pending payloads", "client_id", clientID, "error", err)
				continue
			}
			pendingJSON, _ := json.Marshal(store.PendingMsg{Payloads: payloads})
			h.hub.Send(clientID, notify.WSMessage{
				Type:    "pending",
				Payload: pendingJSON,
			})
			if len(payloads) > 0 {
				slog.Info("sent pending payloads", "client_id", clientID, "count", len(payloads))
			}

		case "ack":
			var ack store.AckMsg
			if err := json.Unmarshal(msg.Payload, &ack); err != nil {
				slog.Warn("bad ack", "client_id", clientID, "error", err)
				continue
			}
			if ack.Success {
				if err := h.store.AckPayload(r.Context(), ack.ID, userID); err != nil {
					slog.Error("ack payload", "id", ack.ID, "error", err)
				} else {
					slog.Info("payload delivered", "id", ack.ID, "user", userID)
					sseData, _ := json.Marshal(map[string]string{"id": ack.ID})
					h.sseHub.Publish(userID, string(sseData))
				}
			} else {
				slog.Warn("payload nacked", "id", ack.ID, "client_id", clientID, "reason", ack.Error)
			}
		}
	}
}
