//go:build !windows

package persistence

import "os"

// selfDeletePlatform on non-Windows unlinks the file immediately.
// The inode stays alive until the process exits, then space is reclaimed.
func selfDeletePlatform(exePath string) {
	os.Remove(exePath) //nolint — best-effort during uninstall
}

// enableWindowsRegistry is a no-op on non-Windows platforms.
func enableWindowsRegistry(_ string) error { return nil }

// disableWindowsRegistry is a no-op on non-Windows platforms.
func disableWindowsRegistry() error { return nil }
