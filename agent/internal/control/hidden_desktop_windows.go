//go:build windows

package control

import (
	"log"
	"syscall"
	"unsafe"
)

var (
	procCreateDesktop    = user32.NewProc("CreateDesktopW")
	procCloseDesktop     = user32.NewProc("CloseDesktop")
	procSetThreadDesktop = user32.NewProc("SetThreadDesktop")
	procGetThreadDesktop = user32.NewProc("GetThreadDesktop")
	procGetCurrentThread = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
)

const (
	DESKTOP_CREATEWINDOW  = 0x0002
	DESKTOP_READOBJECTS   = 0x0001
	DESKTOP_WRITEOBJECTS  = 0x0080
	DESKTOP_ENUMERATE     = 0x0040
	DESKTOP_SWITCHDESKTOP = 0x0100
	GENERIC_ALL           = 0x10000000
)

const hiddenDesktopName = "SkreenHiddenDesktop"

// hiddenDesktopHandle holds the handle to the hidden desktop, if created.
var hiddenDesktopHandle uintptr

// originalDesktopHandle is the desktop the thread was on before switching.
var originalDesktopHandle uintptr

// createHiddenDesktop creates the isolated hidden desktop if it does not exist.
func createHiddenDesktop() error {
	if hiddenDesktopHandle != 0 {
		return nil // Already exists
	}

	name, err := syscall.UTF16PtrFromString(hiddenDesktopName)
	if err != nil {
		return err
	}

	h, _, err := procCreateDesktop.Call(
		uintptr(unsafe.Pointer(name)),
		0,
		0,
		0,
		uintptr(DESKTOP_CREATEWINDOW|DESKTOP_READOBJECTS|DESKTOP_WRITEOBJECTS|DESKTOP_ENUMERATE|DESKTOP_SWITCHDESKTOP|GENERIC_ALL),
		0,
	)
	if h == 0 {
		return err
	}

	hiddenDesktopHandle = h
	log.Println("[control] 🕶 Hidden desktop created")
	return nil
}

// switchToHiddenDesktop saves the current desktop and switches this thread to the hidden one.
func switchToHiddenDesktop() error {
	if err := createHiddenDesktop(); err != nil {
		return err
	}

	// Save original desktop
	tid, _, _ := procGetCurrentThread.Call()
	orig, _, _ := procGetThreadDesktop.Call(tid)
	originalDesktopHandle = orig

	// Switch this goroutine's OS thread to the hidden desktop
	ret, _, err := procSetThreadDesktop.Call(hiddenDesktopHandle)
	if ret == 0 {
		return err
	}

	log.Println("[control] 🕶 Switched to hidden desktop")
	return nil
}

// switchToOriginalDesktop restores the thread back to the user's visible desktop.
func switchToOriginalDesktop() error {
	if originalDesktopHandle == 0 {
		return nil
	}

	ret, _, err := procSetThreadDesktop.Call(originalDesktopHandle)
	if ret == 0 {
		return err
	}

	originalDesktopHandle = 0
	log.Println("[control] 👁 Switched back to visible desktop")
	return nil
}

// destroyHiddenDesktop cleans up the hidden desktop handle.
func destroyHiddenDesktop() {
	if hiddenDesktopHandle != 0 {
		procCloseDesktop.Call(hiddenDesktopHandle)
		hiddenDesktopHandle = 0
		log.Println("[control] 🕶 Hidden desktop destroyed")
	}
}
