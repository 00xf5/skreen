//go:build windows

package control

import (
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSetCursorPos     = user32.NewProc("SetCursorPos")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procmouse_event      = user32.NewProc("mouse_event")
	prockeybd_event      = user32.NewProc("keybd_event")
)

const (
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	MOUSEEVENTF_LEFTDOWN   = 0x0002
	MOUSEEVENTF_LEFTUP     = 0x0004
	MOUSEEVENTF_RIGHTDOWN  = 0x0008
	MOUSEEVENTF_RIGHTUP    = 0x0010
	MOUSEEVENTF_MIDDLEDOWN = 0x0020
	MOUSEEVENTF_MIDDLEUP   = 0x0040

	KEYEVENTF_KEYUP = 0x0002
)

type POINT struct {
	X int32
	Y int32
}

func getScreenSize() (int, int) {
	w, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXSCREEN))
	h, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYSCREEN))
	return int(w), int(h)
}

func getMousePos() (int, int) {
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return int(pt.X), int(pt.Y)
}

func setMousePos(x, y int) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
}

func mouseToggle(button, state string) {
	var flags uintptr
	down := state == "down"

	switch button {
	case "left":
		if down {
			flags = MOUSEEVENTF_LEFTDOWN
		} else {
			flags = MOUSEEVENTF_LEFTUP
		}
	case "right":
		if down {
			flags = MOUSEEVENTF_RIGHTDOWN
		} else {
			flags = MOUSEEVENTF_RIGHTUP
		}
	case "center":
		if down {
			flags = MOUSEEVENTF_MIDDLEDOWN
		} else {
			flags = MOUSEEVENTF_MIDDLEUP
		}
	}

	if flags != 0 {
		procmouse_event.Call(flags, 0, 0, 0, 0)
	}
}

// virtualKeyCode maps basic keys to Windows virtual key codes (VK)
func virtualKeyCode(key string) byte {
	if len(key) == 1 {
		k := key[0]
		if k >= 'a' && k <= 'z' {
			return k - 32 // uppercase
		}
		if k >= 'A' && k <= 'Z' {
			return k
		}
		if k >= '0' && k <= '9' {
			return k
		}
	}

	switch key {
	case "backspace": return 0x08
	case "tab": return 0x09
	case "enter": return 0x0D
	case "shift": return 0x10
	case "ctrl": return 0x11
	case "alt": return 0x12
	case "pause": return 0x13
	case "capslock": return 0x14
	case "escape": return 0x1B
	case "space": return 0x20
	case "pageup": return 0x21
	case "pagedown": return 0x22
	case "end": return 0x23
	case "home": return 0x24
	case "left": return 0x25
	case "up": return 0x26
	case "right": return 0x27
	case "down": return 0x28
	case "insert": return 0x2D
	case "delete": return 0x2E
	case "command": return 0x5B // Left Windows key
	}
	return 0
}

func keybdToggle(key, state string) {
	vk := virtualKeyCode(key)
	if vk == 0 {
		return
	}

	var flags uintptr
	if state == "up" {
		flags = KEYEVENTF_KEYUP
	}

	prockeybd_event.Call(uintptr(vk), 0, flags, 0)
}
