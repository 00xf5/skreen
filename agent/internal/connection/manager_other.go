//go:build !windows

package connection

import (
	"fmt"

	"github.com/pion/webrtc/v3"
)

func IsSession0() bool {
	return false
}

func (m *Manager) StartHelperProcess(iceServers []webrtc.ICEServer) error {
	return fmt.Errorf("session helper not supported on this platform")
}

func (m *Manager) StopHelperProcess() {}

func (m *Manager) SendToHelper(msg interface{}) bool {
	return false
}
