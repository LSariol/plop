//go:build windows

package notify

import (
	"os"
	"path/filepath"

	"gopkg.in/toast.v1"
)

func PushNotification(title, message, folderPath string) error {
	exePath, _ := os.Executable()
	icon := filepath.Join(filepath.Dir(exePath), "plop1.png")
	n := toast.Notification{
		AppID:               appID,
		Title:               title,
		Message:             message,
		Icon:                icon,
		ActivationType:      "protocol",
		ActivationArguments: "file:///" + filepath.ToSlash(folderPath),
	}
	return n.Push()
}
