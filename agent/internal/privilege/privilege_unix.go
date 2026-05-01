//go:build linux || darwin

package privilege

import (
	"os"
)

// detectPlatform checks if running as root (Unix)
func detectPlatform() Level {
	if os.Getuid() == 0 {
		return LevelSystem // root
	}
	// Check if in sudo group (simplified)
	return LevelUser
}
