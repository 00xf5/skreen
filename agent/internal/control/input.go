package control

import (
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Manager handles remote input execution and permission locking.
type Manager struct {
	mu           sync.Mutex
	active       atomic.Bool
	isHidden     atomic.Bool
	controllerID string

	// For dropping outdated mouse moves if we lag
	lastMove time.Time

	// hiddenDone signals the hidden-desktop goroutine to exit
	hiddenDone chan struct{}
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) RequestControl(controllerID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active.Load() {
		m.active.Store(true)
		m.controllerID = controllerID
		log.Println("[control] ⚠️ Remote control ACTIVE by", controllerID)
		return true
	}

	if m.controllerID == controllerID {
		return true
	}

	return false
}

func (m *Manager) StopControl(controllerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active.Load() && (controllerID == "" || m.controllerID == controllerID) {
		m.active.Store(false)
		m.controllerID = ""
		log.Println("[control] 🛑 Remote control STOPPED")
		// Always clean up hidden mode on stop
		if m.isHidden.Load() {
			if m.hiddenDone != nil {
				close(m.hiddenDone)
				m.hiddenDone = nil
			}
			m.isHidden.Store(false)
			destroyHiddenDesktop()
		}
	}
}

// IsHidden returns true when the agent is operating on the hidden desktop.
func (m *Manager) IsHidden() bool {
	return m.isHidden.Load()
}

// SetHiddenMode switches the input thread between the visible and hidden desktop.
// In hidden mode the local user's session is completely undisturbed.
func (m *Manager) SetHiddenMode(hidden bool) {
	if hidden == m.isHidden.Load() {
		return
	}

	if hidden {
		if err := createHiddenDesktop(); err != nil {
			log.Printf("[control] ⚠️ Could not create hidden desktop: %v", err)
			return
		}
		done := make(chan struct{})
		m.hiddenDone = done
		m.isHidden.Store(true)

		// Run a dedicated OS thread that stays on the hidden desktop.
		// SendInput is thread-affine on desktop context.
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			if err := switchToHiddenDesktop(); err != nil {
				log.Printf("[control] ⚠️ Could not switch desktop: %v", err)
				m.isHidden.Store(false)
				return
			}
			log.Println("[control] 🕶 Hidden desktop thread active")
			<-done // Block until StopControl or another SetHiddenMode(false)
			switchToOriginalDesktop()
		}()
	} else {
		if m.hiddenDone != nil {
			close(m.hiddenDone)
			m.hiddenDone = nil
		}
		m.isHidden.Store(false)
		log.Println("[control] 👁 Returned to visible desktop")
	}
}

func (m *Manager) IsActive(controllerID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active.Load() && m.controllerID == controllerID
}

func (m *Manager) HandleMouse(event string, x, y float64, button, state string) {
	if !m.active.Load() {
		return
	}

	switch event {
	case "move":
		m.mu.Lock()
		now := time.Now()
		if now.Sub(m.lastMove) < 20*time.Millisecond {
			m.mu.Unlock()
			return
		}
		m.lastMove = now
		m.mu.Unlock()

		width, height := getScreenSize()
		realX := int(x * float64(width))
		realY := int(y * float64(height))

		currentX, currentY := getMousePos()
		go smoothMove(currentX, currentY, realX, realY)

	case "click":
		log.Printf("[control] click %s %s", button, state)
		mouseToggle(button, state)

	case "scroll":
		// y holds the scroll delta: positive = up, negative = down
		delta := int(y)
		if delta != 0 {
			mouseScroll(delta)
		}
	}
}

func smoothMove(fromX, fromY, toX, toY int) {
	steps := 3 
	for i := 1; i <= steps; i++ {
		x := fromX + (toX-fromX)*i/steps
		y := fromY + (toY-fromY)*i/steps
		setMousePos(x, y)
		time.Sleep(5 * time.Millisecond)
	}
}

func (m *Manager) HandleKeyboard(key, state string) {
	if !m.active.Load() {
		return
	}
	log.Printf("[control] key %s %s", key, state)
	keybdToggle(mapKey(key), state)
}

func mapKey(key string) string {
	k := strings.ToLower(key)
	switch k {
	case "meta", "os":
		return "command"
	case "control":
		return "ctrl"
	case "arrowup":
		return "up"
	case "arrowdown":
		return "down"
	case "arrowleft":
		return "left"
	case "arrowright":
		return "right"
	case " ":
		return "space"
	}
	return k
}

func (m *Manager) HandleSpecialKey(key string) {
	if !m.active.Load() {
		return
	}
	log.Printf("[control] special key %s", key)
	switch strings.ToLower(key) {
	case "cad":
		sendCAD()
	case "win":
		sendWinKey()
	}
}

func (m *Manager) SetBlockInput(block bool) {
	if !m.active.Load() {
		return
	}
	log.Printf("[control] block input: %v", block)
	setBlockInput(block)
}
