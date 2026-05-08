//go:build windows

package persistence

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	moveFileExW     = kernel32.NewProc("MoveFileExW")
)

const MOVEFILE_DELAY_UNTIL_REBOOT = 0x4

// selfDeletePlatform on Windows uses MoveFileExW with DELAY_UNTIL_REBOOT.
// The OS marks the file for deletion and removes it on the next boot,
// which is the only reliable way to remove a running executable on Windows.
func selfDeletePlatform(exePath string) {
	from, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return
	}
	// Pass nil as destination — this tells Windows to delete the file on reboot
	moveFileExW.Call(
		uintptr(unsafe.Pointer(from)),
		0,
		uintptr(MOVEFILE_DELAY_UNTIL_REBOOT),
	)
}

// enableWindowsRegistry writes the Run key for HKCU persistence.
// This is called from the platform-agnostic Enable() in persistence.go.
func enableWindowsRegistry(agentPath string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("AgentPersistence", agentPath)
}

// disableWindowsRegistry removes the Run key, fully clearing persistence.
func disableWindowsRegistry() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		// If key doesn't exist, nothing to clean up — that's fine
		return nil
	}
	defer k.Close()
	k.DeleteValue("AgentPersistence") // Ignore error if value doesn't exist
	return nil
}
