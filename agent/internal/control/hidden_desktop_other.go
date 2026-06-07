//go:build !windows

package control

// Hidden desktop is a Windows-only concept.
// On other platforms these are no-ops.

func createHiddenDesktop() error                    { return nil }
func switchToHiddenDesktop() error                  { return nil }
func switchToOriginalDesktop() error                { return nil }
func destroyHiddenDesktop()                         {}
func SwitchThreadToDesktop(hidden bool) error       { return nil }
func GetInputDesktopName() string                    { return "Default" }
func SwitchThreadToInputDesktop() func()             { return nil }

