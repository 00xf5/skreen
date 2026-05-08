//go:build windows

// Package sysinfo exposes lightweight system metadata used to populate the
// per-agent info row in the Skreen dashboard (hostname, username, idle time).
package sysinfo

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// FetchPublicIP retrieves the agent's public IP from a remote service.
func FetchPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://icanhazip.com")
	if err != nil {
		return "Unknown"
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Unknown"
	}
	return strings.TrimSpace(string(body))
}

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	getLastInputInfo     = user32.NewProc("GetLastInputInfo")
	getTickCount         = kernel32.NewProc("GetTickCount")
	getTickCount64       = kernel32.NewProc("GetTickCount64")
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	getDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
	regOpenKeyExW        = advapi32.NewProc("RegOpenKeyExW")
	regQueryValueExW     = advapi32.NewProc("RegQueryValueExW")
	regCloseKey          = advapi32.NewProc("RegCloseKey")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// GetIdleSeconds returns the number of seconds since the last keyboard/mouse input.
func GetIdleSeconds() int64 {
	var info lastInputInfo
	info.cbSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := getLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0
	}

	tick, _, _ := getTickCount.Call()
	idleMs := int64(uint32(tick)) - int64(info.dwTime)
	if idleMs < 0 {
		idleMs = 0
	}
	return idleMs / 1000
}

// GetUsername returns the currently logged-in username.
func GetUsername() string {
	u, err := user.Current()
	if err == nil && u.Username != "" {
		// Strip domain prefix if present (e.g. "MACHINE\User" -> "User")
		parts := strings.Split(u.Username, "\\")
		return parts[len(parts)-1]
	}
	return "unknown"
}

var (
	cachedPublicIP    string
	lastPublicIPFetch time.Time
)

func getCachedPublicIP() string {
	if cachedPublicIP != "" && time.Since(lastPublicIPFetch) < 10*time.Minute {
		return cachedPublicIP
	}
	cachedPublicIP = FetchPublicIP()
	lastPublicIPFetch = time.Now()
	return cachedPublicIP
}

// GetSystemStats gathers hardware telemetry.
func GetSystemStats() SystemStats {
	stats := SystemStats{
		CPU:          getCPUName(),
		Uptime:       getUptime(),
		LocalIP:      getLocalIP(),
		PublicIP:     getCachedPublicIP(),
	}

	stats.RAMTotal, stats.RAMUsed = getRAMUsage()
	stats.DiskTotal, stats.DiskFree = getDiskUsage()

	return stats
}

func getCPUName() string {
	var hKey syscall.Handle
	path, _ := syscall.UTF16PtrFromString(`HARDWARE\DESCRIPTION\System\CentralProcessor\0`)
	ret, _, _ := regOpenKeyExW.Call(
		uintptr(syscall.HKEY_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(path)),
		0,
		uintptr(0x20019), // KEY_READ
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return "Unknown CPU"
	}
	defer regCloseKey.Call(uintptr(hKey))

	name, _ := syscall.UTF16PtrFromString("ProcessorNameString")
	var buf [256]uint16
	var size uint32 = uint32(len(buf) * 2)
	ret, _, _ = regQueryValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(name)),
		0,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret != 0 {
		return "Unknown CPU"
	}
	return syscall.UTF16ToString(buf[:])
}

func getRAMUsage() (total, used uint64) {
	var mem memoryStatusEx
	mem.dwLength = uint32(unsafe.Sizeof(mem))
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if ret == 0 {
		return 0, 0
	}
	return mem.ullTotalPhys, mem.ullTotalPhys - mem.ullAvailPhys
}

func getDiskUsage() (total, free uint64) {
	root, _ := syscall.UTF16PtrFromString(os.Getenv("SystemDrive") + "\\")
	var freeBytes, totalBytes, totalFreeBytes uint64
	ret, _, _ := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return 0, 0
	}
	return totalBytes, freeBytes
}

func getUptime() uint64 {
	ret, _, _ := getTickCount64.Call()
	return uint64(ret) / 1000
}

func getLocalIP() string {
	// Simplified local IP detection
	conn, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return "127.0.0.1"
	}
	defer syscall.Closesocket(conn)

	// We don't actually connect, just use it to find the best local interface
	addr := syscall.SockaddrInet4{Port: 80, Addr: [4]byte{8, 8, 8, 8}}
	if err := syscall.Connect(conn, &addr); err != nil {
		return "127.0.0.1"
	}

	sa, err := syscall.Getsockname(conn)
	if err != nil {
		return "127.0.0.1"
	}

	if sa4, ok := sa.(*syscall.SockaddrInet4); ok {
		return fmt.Sprintf("%d.%d.%d.%d", sa4.Addr[0], sa4.Addr[1], sa4.Addr[2], sa4.Addr[3])
	}
	return "127.0.0.1"
}
