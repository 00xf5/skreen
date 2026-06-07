//go:build windows

package connection

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/pion/webrtc/v3"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procProcessIdToSessionId = kernel32.NewProc("ProcessIdToSessionId")
	procGetCurrentProcessId  = kernel32.NewProc("GetCurrentProcessId")

	wtsapi32                         = syscall.NewLazyDLL("wtsapi32.dll")
	procWTSGetActiveConsoleSessionId = kernel32.NewProc("WTSGetActiveConsoleSessionId")
	procWTSQueryUserToken            = wtsapi32.NewProc("WTSQueryUserToken")

	userenv                     = syscall.NewLazyDLL("userenv.dll")
	procCreateEnvironmentBlock  = userenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock = userenv.NewProc("DestroyEnvironmentBlock")
)

func IsSession0() bool {
	pid, _, _ := procGetCurrentProcessId.Call()
	var sessionId uint32
	ret, _, _ := procProcessIdToSessionId.Call(pid, uintptr(unsafe.Pointer(&sessionId)))
	if ret == 0 {
		return false
	}
	return sessionId == 0
}

func GetActiveSessionID() (uint32, error) {
	sid, _, _ := procWTSGetActiveConsoleSessionId.Call()
	if sid == 0xFFFFFFFF {
		return 0, fmt.Errorf("no active console session")
	}
	return uint32(sid), nil
}

func GetSessionUserToken(sessionID uint32) (syscall.Token, error) {
	var token syscall.Token
	ret, _, err := procWTSQueryUserToken.Call(uintptr(sessionID), uintptr(unsafe.Pointer(&token)))
	if ret == 0 {
		return 0, fmt.Errorf("WTSQueryUserToken failed: %w", err)
	}
	return token, nil
}

func GetUserEnvironment(token syscall.Token) ([]string, error) {
	var envBlock uintptr
	ret, _, err := procCreateEnvironmentBlock.Call(uintptr(unsafe.Pointer(&envBlock)), uintptr(token), 0)
	if ret == 0 {
		return nil, fmt.Errorf("CreateEnvironmentBlock failed: %w", err)
	}
	defer procDestroyEnvironmentBlock.Call(envBlock)

	var env []string
	p := envBlock
	for {
		length := 0
		for {
			val := *(*uint16)(unsafe.Pointer(p + uintptr(length*2)))
			if val == 0 {
				break
			}
			length++
		}
		if length == 0 {
			break
		}

		utf16Slice := unsafe.Slice((*uint16)(unsafe.Pointer(p)), length)
		env = append(env, string(utf16.Decode(utf16Slice)))

		p += uintptr((length + 1) * 2)
	}
	return env, nil
}

func SpawnHelperProcess(token syscall.Token, env []string) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, nil, nil, err
	}

	cmd := exec.Command(exePath, "-session-helper")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token:         token,
		CreationFlags: syscall.CREATE_UNICODE_ENVIRONMENT,
	}
	cmd.Env = env
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, err
	}

	return cmd, stdin, stdout, nil
}

func (m *Manager) StartHelperProcess(iceServers []webrtc.ICEServer) error {
	m.helperMu.Lock()
	defer m.helperMu.Unlock()

	if m.helperCmd != nil {
		return nil // Already running
	}

	sid, err := GetActiveSessionID()
	if err != nil {
		return fmt.Errorf("get active session ID: %w", err)
	}

	token, err := GetSessionUserToken(sid)
	if err != nil {
		return fmt.Errorf("get session user token: %w", err)
	}
	defer syscall.CloseHandle(syscall.Handle(token))

	env, err := GetUserEnvironment(token)
	if err != nil {
		log.Printf("[manager] Warning: failed to load user environment (using fallback): %v", err)
		env = nil
	}

	cmd, stdin, stdout, err := SpawnHelperProcess(token, env)
	if err != nil {
		return fmt.Errorf("spawn helper process: %w", err)
	}

	m.helperCmd = cmd
	m.helperStdin = stdin
	m.helperStdout = stdout

	// Start reading stdout in a background thread
	go m.readHelperStdout(stdout)

	// Send start stream message
	startMsg := map[string]interface{}{
		"type":        "start_stream",
		"ice_servers": iceServers,
	}
	m.writeToHelper(startMsg)

	log.Printf("[manager] Helper process started in session %d (PID %d)", sid, cmd.Process.Pid)
	return nil
}

func (m *Manager) StopHelperProcess() {
	m.helperMu.Lock()
	defer m.helperMu.Unlock()

	if m.helperCmd == nil {
		return
	}

	log.Println("[manager] Stopping session helper process...")
	
	// Send stop message first
	m.writeToHelper(map[string]interface{}{"type": "stop_stream"})
	
	// Close pipes
	if m.helperStdin != nil {
		m.helperStdin.Close()
	}
	
	// Wait or kill
	done := make(chan error, 1)
	go func() {
		done <- m.helperCmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		log.Println("[manager] Helper process did not exit, killing...")
		m.helperCmd.Process.Kill()
	}

	m.helperCmd = nil
	m.helperStdin = nil
	m.helperStdout = nil
}

func (m *Manager) SendToHelper(msg interface{}) bool {
	m.helperMu.Lock()
	defer m.helperMu.Unlock()

	if m.helperCmd == nil {
		return false
	}
	m.writeToHelper(msg)
	return true
}

func (m *Manager) writeToHelper(msg interface{}) {
	if m.helperStdin == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	m.helperStdin.Write(append(data, '\n'))
}

func (m *Manager) readHelperStdout(stdout io.ReadCloser) {
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Printf("[manager] Helper stdout EOF/error: %v", err)
			break
		}

		// Forward or handle message from helper
		var msg struct {
			Type      string             `json:"type"`
			SDP       string             `json:"sdp,omitempty"`
			Candidate string             `json:"candidate,omitempty"`
			Displays  int                `json:"displays,omitempty"`
			Error     string             `json:"error,omitempty"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("[manager] Failed to unmarshal helper output: %v", err)
			continue
		}

		switch msg.Type {
		case "webrtc_offer":
			m.Send(Message{
				Type:    MsgWebRTCOffer,
				AgentID: m.config.Agent.ID,
				SDP:     msg.SDP,
			})
		case "ice_candidate":
			m.Send(Message{
				Type:      MsgICECandidate,
				AgentID:   m.config.Agent.ID,
				Candidate: msg.Candidate,
			})
		case "stream_ready":
			m.Send(Message{
				Type:    MsgStreamReady,
				AgentID: m.config.Agent.ID,
				Data: map[string]interface{}{
					"displays": msg.Displays,
				},
			})
		case "stream_stopped":
			m.Send(Message{
				Type:    MsgStreamStopped,
				AgentID: m.config.Agent.ID,
			})
			m.StopHelperProcess()
		case "error":
			m.Send(Message{
				Type:    MsgError,
				AgentID: m.config.Agent.ID,
				Error:   msg.Error,
			})
		}
	}
}
