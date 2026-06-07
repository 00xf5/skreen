//go:build windows

package control

import (
	"log"
	"sync"
	"syscall"
	"unsafe"
)

var (
	procCreateDesktop             = user32.NewProc("CreateDesktopW")
	procOpenDesktop               = user32.NewProc("OpenDesktopW")
	procCloseDesktop              = user32.NewProc("CloseDesktop")
	procSetThreadDesktop          = user32.NewProc("SetThreadDesktop")
	procGetThreadDesktop          = user32.NewProc("GetThreadDesktop")
	procGetCurrentThread          = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
	procOpenInputDesktop          = user32.NewProc("OpenInputDesktop")
	procGetUserObjectInformationW = user32.NewProc("GetUserObjectInformationW")
)

const UOI_NAME = 2

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

var desktopMgr = struct {
	mu           sync.Mutex
	origDesktops map[uint32]uintptr
}{
	origDesktops: make(map[uint32]uintptr),
}

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

	// Spawn interactive shell so there is something to interact with
	spawnShellOnHiddenDesktop()
	return nil
}

func startProcessOnDesktop(cmdStr string, desktopName string) error {
	cmdPtr, err := syscall.UTF16PtrFromString(cmdStr)
	if err != nil {
		return err
	}

	desktopPtr, err := syscall.UTF16PtrFromString(desktopName)
	if err != nil {
		return err
	}

	var si syscall.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Desktop = desktopPtr

	var pi syscall.ProcessInformation

	err = syscall.CreateProcess(
		nil,
		cmdPtr,
		nil,
		nil,
		false,
		0,
		nil,
		nil,
		&si,
		&pi,
	)
	if err != nil {
		return err
	}

	// Close handles to prevent leaks
	syscall.CloseHandle(pi.Process)
	syscall.CloseHandle(pi.Thread)
	return nil
}

func spawnShellOnHiddenDesktop() {
	// Start cmd.exe /c start powershell.exe on the hidden desktop
	err := startProcessOnDesktop("cmd.exe /c start powershell.exe", hiddenDesktopName)
	if err != nil {
		log.Printf("[control] Failed to spawn powershell on hidden desktop: %v", err)
	} else {
		log.Println("[control] Spawned interactive shell on hidden desktop")
	}
}

// SwitchThreadToDesktop switches the calling OS thread to the hidden desktop (if true)
// or back to the thread's original desktop (if false).
func SwitchThreadToDesktop(hidden bool) error {
	tid, _, _ := procGetCurrentThread.Call()

	if hidden {
		if err := createHiddenDesktop(); err != nil {
			return err
		}

		desktopMgr.mu.Lock()
		if _, exists := desktopMgr.origDesktops[uint32(tid)]; !exists {
			orig, _, _ := procGetThreadDesktop.Call(tid)
			desktopMgr.origDesktops[uint32(tid)] = orig
		}
		desktopMgr.mu.Unlock()

		ret, _, err := procSetThreadDesktop.Call(hiddenDesktopHandle)
		if ret == 0 {
			return err
		}
	} else {
		desktopMgr.mu.Lock()
		orig := desktopMgr.origDesktops[uint32(tid)]
		desktopMgr.mu.Unlock()

		if orig != 0 {
			ret, _, err := procSetThreadDesktop.Call(orig)
			if ret == 0 {
				return err
			}
		}
	}
	return nil
}

// switchToHiddenDesktop saves the current desktop and switches this thread to the hidden one.
func switchToHiddenDesktop() error {
	return SwitchThreadToDesktop(true)
}

// switchToOriginalDesktop restores the thread back to the user's visible desktop.
func switchToOriginalDesktop() error {
	return SwitchThreadToDesktop(false)
}

// destroyHiddenDesktop cleans up the hidden desktop handle.
func destroyHiddenDesktop() {
	if hiddenDesktopHandle != 0 {
		procCloseDesktop.Call(hiddenDesktopHandle)
		hiddenDesktopHandle = 0
		log.Println("[control] 🕶 Hidden desktop destroyed")
	}
}

// GetInputDesktopName returns the name of the desktop that currently has
// keyboard/mouse input focus (e.g. "Default", "Winlogon", "SkreenHiddenDesktop").
// Returns empty string on failure.
func GetInputDesktopName() string {
	hDesk, _, _ := procOpenInputDesktop.Call(0, 0, uintptr(DESKTOP_READOBJECTS|DESKTOP_ENUMERATE))
	if hDesk == 0 {
		return ""
	}
	defer procCloseDesktop.Call(hDesk)

	var buf [256]uint16
	var size uint32 = uint32(len(buf) * 2)
	ret, _, _ := procGetUserObjectInformationW.Call(
		hDesk,
		UOI_NAME,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:])
}

// SwitchThreadToInputDesktop moves the calling OS thread to whichever
// desktop currently has physical input focus. This keeps the capture loop
// correctly attached through UAC prompts and lock screens.
// Returns the cleanup func to call when capture is done.
// Only call from a runtime.LockOSThread goroutine.
func SwitchThreadToInputDesktop() func() {
	hDesk, _, _ := procOpenInputDesktop.Call(0, 0,
		uintptr(DESKTOP_CREATEWINDOW|DESKTOP_READOBJECTS|DESKTOP_WRITEOBJECTS|DESKTOP_ENUMERATE))
	if hDesk == 0 {
		return nil
	}

	tid, _, _ := procGetCurrentThread.Call()

	// Save original desktop for this thread
	orig, _, _ := procGetThreadDesktop.Call(tid)

	ret, _, _ := procSetThreadDesktop.Call(hDesk)
	if ret == 0 {
		procCloseDesktop.Call(hDesk)
		return nil
	}

	return func() {
		if orig != 0 {
			procSetThreadDesktop.Call(orig)
		}
		procCloseDesktop.Call(hDesk)
	}
}

// OpenDesktopByName opens an existing desktop by name.
func OpenDesktopByName(name string) uintptr {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	h, _, _ := procOpenDesktop.Call(
		uintptr(unsafe.Pointer(namePtr)),
		0,
		0,
		uintptr(DESKTOP_CREATEWINDOW|DESKTOP_READOBJECTS|DESKTOP_WRITEOBJECTS|DESKTOP_ENUMERATE|DESKTOP_SWITCHDESKTOP),
	)
	return h
}

// SetThreadDesktopHandle sets the current thread's desktop to the given handle.
func SetThreadDesktopHandle(hDesk uintptr) error {
	ret, _, err := procSetThreadDesktop.Call(hDesk)
	if ret == 0 {
		if err != nil {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}

// CloseDesktopHandle closes a desktop handle.
func CloseDesktopHandle(hDesk uintptr) {
	if hDesk != 0 {
		procCloseDesktop.Call(hDesk)
	}
}

