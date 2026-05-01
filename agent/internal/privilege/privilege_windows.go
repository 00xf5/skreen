//go:build windows

package privilege

import (
	"golang.org/x/sys/windows/registry"
)

// detectPlatform checks if running as administrator (Windows)
func detectPlatform() Level {
	// Try to open a registry key that requires admin rights
	// HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion`,
		registry.QUERY_VALUE)

	if err == nil {
		key.Close()
		return LevelAdmin
	}

	return LevelUser
}
