//go:build !windows

package control

func getScreenSize() (int, int) {
	return 1920, 1080
}

func getMousePos() (int, int) {
	return 0, 0
}

func setMousePos(x, y int) {}

func mouseToggle(button, state string) {}

func keybdToggle(key, state string) {}

func setBlockInput(block bool) {}

func sendCAD() {}

func sendWinKey() {}

func mouseScroll(delta int) {}
