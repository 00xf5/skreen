//go:build windows

package privilege

import (
	"syscall"
)

var (
	shell32         = syscall.NewLazyDLL("shell32.dll")
	isUserAnAdmin   = shell32.NewProc("IsUserAnAdmin")
)

// detectPlatform checks if running as administrator (Windows)
func detectPlatform() Level {
	ret, _, _ := isUserAnAdmin.Call()
	if ret != 0 {
		return LevelAdmin
	}
	return LevelUser
}
