//go:build windows

package tray

import (
	_ "embed"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconBytes []byte

// State carries channels that other packages write to for real-time tray updates.
type State struct {
	LastFolderCh <-chan string // receiver sends the folder path after each delivery
	StatusCh     <-chan string // ws client sends "Connected" / "Disconnected"
	ServerURL    string        // used to open Settings in the browser
}

// Run starts the system tray. It must be called from the main goroutine and blocks
// until the user clicks Quit.
func Run(s *State) {
	systray.Run(func() { onReady(s) }, func() {
		slog.Info("tray exit")
	})
}

func onReady(s *State) {
	systray.SetIcon(iconBytes)
	systray.SetTooltip("Plop — file transfer")

	mStatus := systray.AddMenuItem("Connecting…", "Connection status")
	mStatus.Disable()
	systray.AddSeparator()

	mOpenFolder := systray.AddMenuItem("Open last folder", "Open the most recently received folder")
	mOpenFolder.Disable()

	mSettings := systray.AddMenuItem("Settings", "Open Plop settings in browser")

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit Plop", "Stop the Plop desktop client")

	var mu sync.Mutex
	var lastFolder string

	// Update status label from ws client
	go func() {
		for status := range s.StatusCh {
			mStatus.SetTitle(status)
		}
	}()

	// Track last received folder from receiver
	go func() {
		for folder := range s.LastFolderCh {
			mu.Lock()
			lastFolder = folder
			mu.Unlock()
			mOpenFolder.SetTitle("Open: " + filepath.Base(folder))
			mOpenFolder.Enable()
		}
	}()

	// Handle menu clicks
	go func() {
		for range mOpenFolder.ClickedCh {
			mu.Lock()
			f := lastFolder
			mu.Unlock()
			if f != "" {
				exec.Command("explorer.exe", f).Start()
			}
		}
	}()

	go func() {
		for range mSettings.ClickedCh {
			if s.ServerURL != "" {
				exec.Command("cmd", "/C", "start", "", s.ServerURL+"/settings").Start()
			}
		}
	}()

	for range mQuit.ClickedCh {
		systray.Quit()
		return
	}
}
