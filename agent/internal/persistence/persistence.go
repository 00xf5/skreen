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

// Windows implementation
func (m *Manager) enableWindows(agentPath string) error {
	// Registry Run key: HKCU\Software\Microsoft\Windows\CurrentVersion\Run
	// Requires admin for HKLM, HKCU works for user
	m.isEnabled = true
	return fmt.Errorf("windows persistence: requires admin to modify registry")
}

func (m *Manager) disableWindows() error {
	m.isEnabled = false
	return nil
}

// Linux implementation
func (m *Manager) enableLinux(agentPath string) error {
	// systemd user service or crontab
	// For now, just mark as enabled
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
