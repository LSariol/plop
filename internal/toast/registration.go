//go:build windows

package notify

import "golang.org/x/sys/windows/registry"

const appID = "plop"

func RegisterApp(appName string, iconPath string) error {
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`SOFTWARE\Classes\AppUserModelId\`+appID,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetStringValue("DisplayName", appName); err != nil {
		return err
	}

	if err := key.SetStringValue("IconUri", iconPath); err != nil {
		return err
	}

	return nil
}

func UnregisterApp() error {
	return registry.DeleteKey(
		registry.CURRENT_USER,
		`SOFTWARE\Classes\AppUserModelId\`+appID,
	)
}
