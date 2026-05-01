//go:build windows

package installer

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
	procShowWindow  = user32.NewProc("ShowWindow")
	procGetConsoleW = kernel32.NewProc("GetConsoleWindow")
)

const (
	SW_HIDE = 0
)

// HideConsole unconditionally hides the console window
func HideConsole() {
	hwnd, _, _ := procGetConsoleW.Call()
	if hwnd != 0 {
		procShowWindow.Call(hwnd, SW_HIDE)
	}
}

// Install checks if this binary is running from the installed location.
// If not (e.g. running from Downloads), it hides the console and lets
// the NSIS installer handle proper installation.
// Returns true only if the process should exit immediately.
func Install() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	exePath = filepath.Clean(exePath)

	// Installed path (matches what NSIS writes to)
	installedPath := filepath.Join(os.Getenv("PROGRAMFILES"), "Skreen", "skreen-agent.exe")

	// If already running from the installed location — we're good, just hide console
	if strings.EqualFold(exePath, installedPath) {
		HideConsole()
		return false
	}

	// Running from somewhere else (Downloads, Desktop, etc.)
	// The NSIS setup.exe handles installation — if someone runs the raw agent
	// binary directly, hide the console and run as-is (development/testing mode).
	HideConsole()
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
