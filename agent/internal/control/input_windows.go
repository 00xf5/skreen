//go:build windows

package control

import (
	"syscall"
	"unsafe"
)

var (
	user32            = syscall.NewLazyDLL("user32.dll")
	procSendInput     = user32.NewProc("SendInput")
	procSetCursorPos  = user32.NewProc("SetCursorPos")
	procGetCursorPos  = user32.NewProc("GetCursorPos")
	procGetSysMetrics = user32.NewProc("GetSystemMetrics")
	procBlockInput    = user32.NewProc("BlockInput")
)

const (
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	INPUT_MOUSE    = 0
	INPUT_KEYBOARD = 1

	MOUSEEVENTF_MOVE       = 0x0001
	MOUSEEVENTF_LEFTDOWN   = 0x0002
	MOUSEEVENTF_LEFTUP     = 0x0004
	MOUSEEVENTF_RIGHTDOWN  = 0x0008
	MOUSEEVENTF_RIGHTUP    = 0x0010
	MOUSEEVENTF_MIDDLEDOWN = 0x0020
	MOUSEEVENTF_MIDDLEUP   = 0x0040
	MOUSEEVENTF_WHEEL      = 0x0800
	MOUSEEVENTF_ABSOLUTE   = 0x8000

	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_UNICODE     = 0x0004
)

// MOUSEINPUT mirrors the Win32 MOUSEINPUT struct exactly.
type MOUSEINPUT struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// KEYBDINPUT mirrors the Win32 KEYBDINPUT struct exactly.
type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// INPUT is the union type passed to SendInput. We embed both structs with
// padding to match the Win32 ABI (the union is the size of the largest member).
type INPUT struct {
	Type uint32
	// Padding to fit the union: MOUSEINPUT is 28 bytes, KEYBDINPUT is 16 bytes.
	// We use MOUSEINPUT as the backing array since it's larger.
	Data [28]byte
}

func sendMouseInput(flags uint32, dx, dy int32, data uint32) {
	mi := MOUSEINPUT{Dx: dx, Dy: dy, MouseData: data, DwFlags: flags}
	inp := INPUT{Type: INPUT_MOUSE}
	copy(inp.Data[:], (*(*[28]byte)(unsafe.Pointer(&mi)))[:])
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func sendKeyInput(vk uint16, flags uint32) {
	ki := KEYBDINPUT{WVk: vk, DwFlags: flags}
	inp := INPUT{Type: INPUT_KEYBOARD}
	copy(inp.Data[:], (*(*[16]byte)(unsafe.Pointer(&ki)))[:])
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

type POINT struct{ X, Y int32 }

func getScreenSize() (int, int) {
	w, _, _ := procGetSysMetrics.Call(uintptr(SM_CXSCREEN))
	h, _, _ := procGetSysMetrics.Call(uintptr(SM_CYSCREEN))
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
	down := state == "down"
	var flags uint32
	switch button {
	case "left":
		if down { flags = MOUSEEVENTF_LEFTDOWN } else { flags = MOUSEEVENTF_LEFTUP }
	case "right":
		if down { flags = MOUSEEVENTF_RIGHTDOWN } else { flags = MOUSEEVENTF_RIGHTUP }
	case "center":
		if down { flags = MOUSEEVENTF_MIDDLEDOWN } else { flags = MOUSEEVENTF_MIDDLEUP }
	}
	if flags != 0 {
		sendMouseInput(flags, 0, 0, 0)
	}
}

func mouseScroll(delta int) {
	// Positive = scroll up, negative = scroll down. Windows WHEEL_DELTA is 120 per notch.
	sendMouseInput(MOUSEEVENTF_WHEEL, 0, 0, uint32(int32(delta*120)))
}

// virtualKeyCode maps key names to Windows Virtual Key codes.
func virtualKeyCode(key string) uint16 {
	if len(key) == 1 {
		k := key[0]
		if k >= 'a' && k <= 'z' { return uint16(k - 32) }
		if k >= 'A' && k <= 'Z' { return uint16(k) }
		if k >= '0' && k <= '9' { return uint16(k) }
	}
	switch key {
	case "backspace":  return 0x08
	case "tab":        return 0x09
	case "enter":      return 0x0D
	case "shift":      return 0x10
	case "ctrl":       return 0x11
	case "alt":        return 0x12
	case "pause":      return 0x13
	case "capslock":   return 0x14
	case "escape":     return 0x1B
	case "space":      return 0x20
	case "pageup":     return 0x21
	case "pagedown":   return 0x22
	case "end":        return 0x23
	case "home":       return 0x24
	case "left":       return 0x25
	case "up":         return 0x26
	case "right":      return 0x27
	case "down":       return 0x28
	case "insert":     return 0x2D
	case "delete":     return 0x2E
	// Function keys
	case "f1":  return 0x70
	case "f2":  return 0x71
	case "f3":  return 0x72
	case "f4":  return 0x73
	case "f5":  return 0x74
	case "f6":  return 0x75
	case "f7":  return 0x76
	case "f8":  return 0x77
	case "f9":  return 0x78
	case "f10": return 0x79
	case "f11": return 0x7A
	case "f12": return 0x7B
	// Numpad
	case "numpad0": return 0x60
	case "numpad1": return 0x61
	case "numpad2": return 0x62
	case "numpad3": return 0x63
	case "numpad4": return 0x64
	case "numpad5": return 0x65
	case "numpad6": return 0x66
	case "numpad7": return 0x67
	case "numpad8": return 0x68
	case "numpad9": return 0x69
	// Special punctuation
	case ";":  return 0xBA
	case "=":  return 0xBB
	case ",":  return 0xBC
	case "-":  return 0xBD
	case ".":  return 0xBE
	case "/":  return 0xBF
	case "`":  return 0xC0
	case "[":  return 0xDB
	case "\\":  return 0xDC
	case "]":  return 0xDD
	case "'":  return 0xDE
	// Windows / meta key
	case "command", "meta", "win": return 0x5B
	}
	return 0
}

func keybdToggle(key, state string) {
	vk := virtualKeyCode(key)
	if vk == 0 {
		return
	}
	var flags uint32
	if state == "up" {
		flags = KEYEVENTF_KEYUP
	}
	sendKeyInput(vk, flags)
}

func setBlockInput(block bool) {
	b := uintptr(0)
	if block {
		b = 1
	}
	procBlockInput.Call(b)
}

func sendCAD() {
	// Ctrl+Alt+Del is intercepted by the Windows kernel (SAS — Secure Attention Sequence).
	// User-mode code cannot synthesise it. This is a hard OS limitation.
	// Left intentionally as no-op; the feature is not available without a kernel driver.
}

func sendWinKey() {
	sendKeyInput(0x5B, 0)               // Left Windows key down
	sendKeyInput(0x5B, KEYEVENTF_KEYUP)  // Left Windows key up
}
