//go:build windows

package clipboard

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	openClipboard   = user32.NewProc("OpenClipboard")
	closeClipboard  = user32.NewProc("CloseClipboard")
	getClipData     = user32.NewProc("GetClipboardData")
	setClipData     = user32.NewProc("SetClipboardData")
	emptyClipboard  = user32.NewProc("EmptyClipboard")
	globalAlloc     = kernel32.NewProc("GlobalAlloc")
	globalLock      = kernel32.NewProc("GlobalLock")
	globalUnlock    = kernel32.NewProc("GlobalUnlock")
	lstrcpyW        = kernel32.NewProc("lstrcpyW")
)

const (
	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0002
)

// Get reads the current clipboard text content.
func Get() (string, error) {
	r, _, _ := openClipboard.Call(0)
	if r == 0 {
		return "", nil
	}
	defer closeClipboard.Call()

	h, _, _ := getClipData.Call(CF_UNICODETEXT)
	if h == 0 {
		return "", nil
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		return "", nil
	}
	defer globalUnlock.Call(h)

	var chars []uint16
	for p := uintptr(ptr); ; p += 2 {
		c := *(*uint16)(unsafe.Pointer(p))
		if c == 0 {
			break
		}
		chars = append(chars, c)
	}
	return string(utf16.Decode(chars)), nil
}

// Set writes text to the clipboard.
func Set(text string) error {
	r, _, _ := openClipboard.Call(0)
	if r == 0 {
		return nil
	}
	defer closeClipboard.Call()

	emptyClipboard.Call()

	encoded := syscall.StringToUTF16(text)
	size := uintptr(len(encoded) * 2)

	hMem, _, _ := globalAlloc.Call(GMEM_MOVEABLE, size)
	if hMem == 0 {
		return nil
	}

	ptr, _, _ := globalLock.Call(hMem)
	if ptr == 0 {
		return nil
	}

	lstrcpyW.Call(ptr, uintptr(unsafe.Pointer(&encoded[0])))
	globalUnlock.Call(hMem)
	setClipData.Call(CF_UNICODETEXT, hMem)

	return nil
}
