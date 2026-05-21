//go:build !windows

package notify

func PushNotification(title, message, folderPath string) error { return nil }
