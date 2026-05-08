//go:build !windows

package installer

// HideConsole is a stub for non-Windows platforms.
func HideConsole() {}

// EnsureSingleInstance is a stub for non-Windows platforms.
func EnsureSingleInstance(name string) {}

// IsInstalled is a stub for non-Windows platforms.
func IsInstalled() bool {
	return false
}

// Install is a stub for non-Windows platforms.
func Install() bool {
	return false
}

// Uninstall is a stub for non-Windows platforms.
func Uninstall() error {
	return nil
}
