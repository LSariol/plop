//go:build windows

package wizard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	"github.com/lsariol/plop/desktop/config"
)

//go:embed setup.html
var setupHTML string

// Run shows the setup wizard in the user's default browser and blocks until
// the user completes pairing. Config and token files are written to exeDir.
func Run(exeDir string) error {
	tmpl, err := template.New("setup").Parse(setupHTML)
	if err != nil {
		return fmt.Errorf("parse setup template: %w", err)
	}

	// Pre-fill from existing config if present
	type tmplData struct{ ServerURL, MachineName string }
	var pre tmplData
	if cfg, err := config.Load(filepath.Join(exeDir, "config.toml")); err == nil {
		pre.ServerURL = cfg.ServerURL
		pre.MachineName = cfg.MachineName
	}

	done := make(chan struct{})

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, pre)
	})

	mux.HandleFunc("POST /setup", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ServerURL   string `json:"server_url"`
			MachineName string `json:"machine_name"`
			PairingCode string `json:"pairing_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeWizardError(w, "invalid request", http.StatusBadRequest)
			return
		}
		if body.ServerURL == "" || body.MachineName == "" || body.PairingCode == "" {
			writeWizardError(w, "all fields are required", http.StatusBadRequest)
			return
		}

		token, err := callPair(body.ServerURL, body.MachineName, body.PairingCode)
		if err != nil {
			writeWizardError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Write config.toml
		if err := writeConfig(exeDir, body.ServerURL, body.MachineName); err != nil {
			writeWizardError(w, "could not write config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Save desktop token
		tokenPath := filepath.Join(exeDir, ".desktop_token")
		if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
			writeWizardError(w, "could not save token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		// Signal completion after response is flushed
		go func() {
			time.Sleep(150 * time.Millisecond)
			close(done)
		}()
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start wizard server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	slog.Info("opening setup wizard", "url", url)
	exec.Command("cmd", "/C", "start", "", url).Start()

	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	return nil
}

func writeWizardError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func callPair(serverURL, machineName, pairingCode string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"pairing_code": strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(pairingCode)), " ", ""),
		"machine_name": machineName,
	})
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(serverURL+"/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("could not reach server — check Server URL")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct{ Error string `json:"error"` }
		json.NewDecoder(resp.Body).Decode(&e)
		return "", fmt.Errorf("%s", e.Error)
	}
	var result struct {
		DesktopToken string `json:"desktop_token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.DesktopToken == "" {
		return "", fmt.Errorf("server returned empty token")
	}
	return result.DesktopToken, nil
}

func writeConfig(exeDir, serverURL, machineName string) error {
	home, _ := os.UserHomeDir()
	defaultFolder := filepath.ToSlash(filepath.Join(home, "Desktop", "Plop"))
	content := fmt.Sprintf("server_url   = %q\nmachine_name = %q\n\n[tags]\ndefault_folder = %q\n",
		serverURL, machineName, defaultFolder)
	return os.WriteFile(filepath.Join(exeDir, "config.toml"), []byte(content), 0644)
}
