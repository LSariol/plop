//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lsariol/plop/desktop/client"
	"github.com/lsariol/plop/desktop/config"
	"github.com/lsariol/plop/desktop/receiver"
	"github.com/lsariol/plop/desktop/tray"
	"github.com/lsariol/plop/desktop/wizard"
	notify "github.com/lsariol/plop/internal/toast"
)

func main() {
	exeDir := getExeDir()

	// Log to file next to the exe (stdout goes nowhere with -H windowsgui).
	logFile, err := os.OpenFile(filepath.Join(exeDir, "plop.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logFile = os.Stderr
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(
		io.MultiWriter(os.Stderr, logFile),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)))

	// ── Config ────────────────────────────────────────────────────────────────

	cfgPath := filepath.Join(exeDir, "config.toml")
	tokenPath := filepath.Join(exeDir, ".desktop_token")

	var desktopToken string

	// 1. Try to load an existing paired token.
	if data, err := os.ReadFile(tokenPath); err == nil {
		desktopToken = strings.TrimSpace(string(data))
		slog.Info("loaded desktop token")
	}

	// 2. No token — try auto-pairing with legacy pairing_code field.
	if desktopToken == "" {
		if cfg, err := config.Load(cfgPath); err == nil && cfg.PairingCode != "" {
			slog.Info("auto-pairing using pairing_code from config")
			token, err := doPairing(cfg.ServerURL, cfg.MachineName, cfg.PairingCode)
			if err != nil {
				slog.Warn("auto-pair failed, falling through to wizard", "error", err)
			} else {
				if writeErr := os.WriteFile(tokenPath, []byte(token), 0600); writeErr != nil {
					slog.Error("save token", "error", writeErr)
					os.Exit(1)
				}
				desktopToken = token
				slog.Info("auto-paired successfully")
			}
		}
	}

	// 3. Still no token — run the setup wizard.
	if desktopToken == "" {
		slog.Info("no desktop token — opening setup wizard")
		if err := wizard.Run(exeDir); err != nil {
			slog.Error("setup wizard failed", "error", err)
			os.Exit(1)
		}
		data, err := os.ReadFile(tokenPath)
		if err != nil {
			slog.Error("token not saved after wizard", "error", err)
			os.Exit(1)
		}
		desktopToken = strings.TrimSpace(string(data))
	}

	// ── Load final config ─────────────────────────────────────────────────────

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	// ── Toast registration ────────────────────────────────────────────────────

	iconPath := filepath.Join(exeDir, "plop1.png")
	if err := notify.RegisterApp("Plop", iconPath); err != nil {
		slog.Warn("toast registration failed", "error", err)
	}

	// ── Tray channels ─────────────────────────────────────────────────────────

	lastFolderCh := make(chan string, 4)
	statusCh := make(chan string, 4)

	// ── Start WS client in background ────────────────────────────────────────

	recv := receiver.New(cfg, desktopToken, func(folder string) {
		select {
		case lastFolderCh <- folder:
		default:
		}
	})
	go client.Run(cfg, desktopToken, recv, statusCh)

	// ── Run system tray (blocks on main goroutine) ────────────────────────────

	slog.Info("plop desktop starting", "server", cfg.ServerURL)
	tray.Run(&tray.State{
		LastFolderCh: lastFolderCh,
		StatusCh:     statusCh,
		ServerURL:    cfg.ServerURL,
	})
}

func getExeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

func doPairing(serverURL, machineName, pairingCode string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"pairing_code": strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(pairingCode)), " ", ""),
		"machine_name": machineName,
	})
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Post(serverURL+"/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("connect to server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct{ Error string `json:"error"` }
		json.NewDecoder(resp.Body).Decode(&e)
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, e.Error)
	}
	var result struct {
		DesktopToken string `json:"desktop_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.DesktopToken == "" {
		return "", fmt.Errorf("server returned empty token")
	}
	return result.DesktopToken, nil
}
