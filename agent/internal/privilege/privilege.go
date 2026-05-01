package privilege

// Level indicates the current privilege level
type Level string

const (
	LevelUser   Level = "user"
	LevelAdmin  Level = "admin"
	LevelSystem Level = "system"
)

// Detect determines the current privilege level
// Platform-specific implementation in privilege_*.go files
func Detect() Level {
	return detectPlatform()
}

// IsElevated returns true if running with admin/root privileges
func IsElevated() bool {
	lvl := Detect()
	return lvl == LevelAdmin || lvl == LevelSystem
}
