package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lsariol/plop/desktop/config"
	"github.com/lsariol/plop/desktop/receiver"
	"github.com/lsariol/plop/internal/notify"
	"github.com/lsariol/plop/internal/store"
)

// Run blocks forever, reconnecting to the server whenever the connection drops.
// statusCh receives human-readable status strings ("Connected", "Disconnected")
// for display in the system tray; pass nil if not needed.
func Run(cfg config.Config, desktopToken string, recv *receiver.Receiver, statusCh chan<- string) {
	backoff := 2 * time.Second
	for {
		connected, err := connect(cfg, desktopToken, recv, statusCh)
		if connected {
			backoff = 2 * time.Second
		}
		sendStatus(statusCh, "Disconnected — reconnecting…")
		log.Printf("ws: %v — reconnecting in %s", err, backoff)
		time.Sleep(backoff)
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

func connect(cfg config.Config, desktopToken string, recv *receiver.Receiver, statusCh chan<- string) (connected bool, err error) {
	wsURL, err := toWSURL(cfg.ServerURL)
	if err != nil {
		return false, err
	}
	wsURL += "/ws/client"

	header := http.Header{}
	header.Set("Authorization", "Bearer "+desktopToken)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return false, fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer conn.Close()
	connected = true

	if err := conn.WriteJSON(notify.WSMessage{Type: "hello"}); err != nil {
		return connected, fmt.Errorf("send hello: %w", err)
	}
	sendStatus(statusCh, "Connected")
	log.Printf("connected to %s", cfg.ServerURL)

	for {
		var msg notify.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return connected, err
		}

		switch msg.Type {
		case "pending":
			var pm store.PendingMsg
			if err := json.Unmarshal(msg.Payload, &pm); err != nil {
				log.Printf("bad pending message: %v", err)
				continue
			}
			for _, p := range pm.Payloads {
				sendAck(conn, p.ID, recv.Receive(p))
			}

		case "payload_ready":
			var p store.PayloadReadyMsg
			if err := json.Unmarshal(msg.Payload, &p); err != nil {
				log.Printf("bad payload_ready message: %v", err)
				continue
			}
			sendAck(conn, p.ID, recv.Receive(p))
		}
	}
}

func sendAck(conn *websocket.Conn, id string, err error) {
	ack := store.AckMsg{ID: id, Success: err == nil}
	if err != nil {
		ack.Error = err.Error()
		log.Printf("payload %s failed: %v", id[:8], err)
	}
	ackJSON, _ := json.Marshal(ack)
	if writeErr := conn.WriteJSON(notify.WSMessage{
		Type:    "ack",
		Payload: ackJSON,
	}); writeErr != nil {
		log.Printf("send ack for %s: %v", id[:8], writeErr)
	}
}

func sendStatus(ch chan<- string, status string) {
	if ch == nil {
		return
	}
	select {
	case ch <- status:
	default:
	}
}

func toWSURL(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("parse server_url %q: %w", serverURL, err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme %q in server_url (use http or https)", u.Scheme)
	}
	return u.String(), nil
}
