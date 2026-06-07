// Package screenshare implements Phase 2 screen capture and WebRTC streaming.
//
// Design rules (mirrors phase2.txt):
//   - Uses pion/webrtc for P2P video (VP8 track)
//   - Uses kbinani/screenshot for frame capture
//   - Sends JPEG-encoded frames as "samples" — good enough for MVP, avoids CGO VP8 encoder dep
//   - Target: 8 FPS, 960x540 (half of 1080p)
//   - Frame-skip guard: never send if previous frame < 125ms ago
//   - All errors logged, never panic
package screenshare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/jpeg"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"scon/agent/internal/control"

	"github.com/kbinani/screenshot"
	"github.com/pion/webrtc/v3"
	"golang.org/x/image/draw"
)

const (
	targetWidth  = 960
	targetHeight = 540
	targetFPS    = 15                      // raised from 8 → 15 for smoother feel
	framePeriod  = time.Second / targetFPS // ~66ms between frames
	jpegQuality  = 50                      // good enough for remote viewing
)

// Session manages one screen-share WebRTC session.
type Session struct {
	pc *webrtc.PeerConnection
	dc *webrtc.DataChannel

	stopCh  chan struct{}
	stopped atomic.Bool
	once    sync.Once

	// Quality settings (mutable at runtime)
	qualityMu    sync.Mutex
	jpegQual     int
	targetW      int
	targetH      int
	framePeriod  time.Duration
	displayIndex int

	// Frame deduplication: skip sending if screen didn't change
	lastChecksum uint32

	// Callbacks
	OnICECandidate func(candidateJSON string)
	OnStateChange  func(state webrtc.ICEConnectionState)
	IsHidden       func() bool
}

// NewSession creates a new PeerConnection and video track.
func NewSession(
	iceServers []webrtc.ICEServer,
	onICECandidate func(candidateJSON string),
	onStateChange func(state webrtc.ICEConnectionState),
) (*Session, error) {
	if len(iceServers) == 0 {
		iceServers = []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		}
	}

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("new peer connection: %w", err)
	}

	// We send JPEG bytes over an unreliable DataChannel.
	// This avoids needing a complex VP8 CGO encoder while still benefiting from
	// WebRTC's UDP-based P2P traversal and low latency.
	ordered := false
	maxRetransmits := uint16(0)
	dc, err := pc.CreateDataChannel("screen", &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	})
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("create data channel: %w", err)
	}

	s := &Session{
		pc:             pc,
		dc:             dc,
		stopCh:         make(chan struct{}),
		OnICECandidate: onICECandidate,
		OnStateChange:  onStateChange,
		// Default: balanced quality on primary display
		jpegQual:    50,
		targetW:     960,
		targetH:     540,
		framePeriod: time.Second / 8,
		displayIndex: 0,
	}

	// Wire ICE candidate trickle
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return // gathering complete
		}
		if s.OnICECandidate == nil {
			return
		}
		candidateInit := c.ToJSON()
		j, err := json.Marshal(candidateInit)
		if err != nil {
			log.Printf("[screenshare] ICE marshal error: %v", err)
			return
		}
		s.OnICECandidate(string(j))
	})

	// Wire connection state
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[screenshare] ICE state: %s", state)
		if s.OnStateChange != nil {
			s.OnStateChange(state)
		}
		if state == webrtc.ICEConnectionStateFailed ||
			state == webrtc.ICEConnectionStateDisconnected {
			s.Stop()
		}
	})

	return s, nil
}

// CreateOffer generates the SDP offer to send to the controller.
func (s *Session) CreateOffer() (string, error) {
	offer, err := s.pc.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("create offer: %w", err)
	}

	// IMPORTANT: must set local description before starting ICE gathering
	if err = s.pc.SetLocalDescription(offer); err != nil {
		return "", fmt.Errorf("set local description: %w", err)
	}

	// Wait until ICE gathering is complete (simplifies trickle; works fine for LAN/local)
	gatherDone := webrtc.GatheringCompletePromise(s.pc)
	select {
	case <-gatherDone:
	case <-time.After(10 * time.Second):
		log.Println("[screenshare] ICE gather timeout; sending partial SDP")
	}

	return s.pc.LocalDescription().SDP, nil
}

// SetAnswer applies the controller's SDP answer.
func (s *Session) SetAnswer(sdp string) error {
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}
	if err := s.pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}
	return nil
}

// AddICECandidate feeds a remote ICE candidate received from the controller.
func (s *Session) AddICECandidate(candidateJSON string) error {
	var init webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidateJSON), &init); err != nil {
		return fmt.Errorf("unmarshal ICE candidate: %w", err)
	}
	return s.pc.AddICECandidate(init)
}

// StartCapture begins the screen capture → encode → RTP loop.
// Call this after SetAnswer so the connection is established.
func (s *Session) StartCapture() {
	var initialHidden bool
	if s.IsHidden != nil {
		initialHidden = s.IsHidden()
	}
	go s.runCaptureLoop(initialHidden, "")
}

// Stop gracefully tears down the session.
func (s *Session) Stop() {
	s.once.Do(func() {
		s.stopped.Store(true)
		close(s.stopCh)
		if err := s.pc.Close(); err != nil {
			log.Printf("[screenshare] pc.Close error: %v", err)
		}
		log.Println("[screenshare] session stopped")
	})
}

// SetQuality adjusts capture quality at runtime without restarting the session.
func (s *Session) SetQuality(q string) {
	s.qualityMu.Lock()
	defer s.qualityMu.Unlock()
	switch q {
	case "low":
		s.jpegQual = 30
		s.targetW = 640
		s.targetH = 360
		s.framePeriod = time.Second / 8
	case "high":
		s.jpegQual = 75
		s.targetW = 1280
		s.targetH = 720
		s.framePeriod = time.Second / 20
	default: // balanced
		s.jpegQual = 50
		s.targetW = 960
		s.targetH = 540
		s.framePeriod = time.Second / 15
	}
	log.Printf("[screenshare] quality set to %s", q)
}

// SetDisplay changes the active display being captured.
func (s *Session) SetDisplay(index int) {
	s.qualityMu.Lock()
	defer s.qualityMu.Unlock()
	max := screenshot.NumActiveDisplays() - 1
	if index >= 0 && index <= max {
		s.displayIndex = index
		log.Printf("[screenshare] display set to %d", index)
	}
}

// NumDisplays returns the total number of connected monitors.
func NumDisplays() int {
	return screenshot.NumActiveDisplays()
}

// runCaptureLoop is the heart of Phase 2.
func (s *Session) runCaptureLoop(hidden bool, activeDesktop string) {
	runtime.LockOSThread()
	// DO NOT call runtime.UnlockOSThread() so that the thread is destroyed by Go runtime on exit.

	var hDesk uintptr
	var deskName string
	if hidden {
		deskName = "SkreenHiddenDesktop"
		hDesk = control.OpenDesktopByName(deskName)
	} else {
		if activeDesktop == "" {
			activeDesktop = control.GetInputDesktopName()
			if activeDesktop == "" {
				activeDesktop = "Default"
			}
		}
		deskName = activeDesktop
		hDesk = control.OpenDesktopByName(deskName)
	}

	if hDesk != 0 {
		if err := control.SetThreadDesktopHandle(hDesk); err != nil {
			log.Printf("[screenshare] SetThreadDesktopHandle to %s failed: %v", deskName, err)
		} else {
			log.Printf("[screenshare] Capture thread attached to desktop: %s", deskName)
		}
		defer control.CloseDesktopHandle(hDesk)
	} else {
		log.Printf("[screenshare] Failed to open desktop: %s", deskName)
	}

	log.Printf("[screenshare] Capture loop started (hidden=%v, desktop=%s)", hidden, deskName)

	n := screenshot.NumActiveDisplays()
	if n < 1 {
		log.Println("[screenshare] no displays detected")
		return
	}

	lastFrame := time.Time{}

	for {
		select {
		case <-s.stopCh:
			log.Printf("[screenshare] Capture loop exiting because stopCh closed")
			return
		default:
		}

		// Check if desktop context has changed
		var currentHidden bool
		if s.IsHidden != nil {
			currentHidden = s.IsHidden()
		}
		if currentHidden != hidden {
			log.Printf("[screenshare] Hidden mode changed from %v to %v. Restarting capture loop thread...", hidden, currentHidden)
			go s.runCaptureLoop(currentHidden, "")
			return
		}

		if !hidden {
			inputDesk := control.GetInputDesktopName()
			if inputDesk != "" && inputDesk != deskName {
				log.Printf("[screenshare] Input desktop changed from %s to %s. Restarting capture loop thread...", deskName, inputDesk)
				go s.runCaptureLoop(false, inputDesk)
				return
			}
		}

		// Read quality settings atomically
		s.qualityMu.Lock()
		fp := s.framePeriod
		tw := s.targetW
		th := s.targetH
		jq := s.jpegQual
		idx := s.displayIndex
		s.qualityMu.Unlock()

		// Frame-skip guard
		now := time.Now()
		if now.Sub(lastFrame) < fp {
			time.Sleep(fp - now.Sub(lastFrame))
			continue
		}
		lastFrame = now

		// Re-fetch bounds in case of display changes
		maxDisplays := screenshot.NumActiveDisplays()
		if idx >= maxDisplays {
			idx = 0 // fallback
		}
		bounds := screenshot.GetDisplayBounds(idx)

		// Capture screen
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			log.Printf("[screenshare] capture error: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// ── Frame deduplication ──────────────────────────────────────────────────────
		checksum := crc32.ChecksumIEEE(img.Pix)
		if checksum == s.lastChecksum {
			continue // Screen unchanged — skip encode + send entirely
		}
		s.lastChecksum = checksum

		// Resize to current target resolution
		dst := image.NewRGBA(image.Rect(0, 0, tw, th))
		scaler := draw.Scaler(draw.BiLinear)
		if jq <= 30 {
			scaler = draw.NearestNeighbor // faster for low-quality mode
		}
		scaler.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

		// JPEG encode
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jq}); err != nil {
			log.Printf("[screenshare] encode error: %v", err)
			continue
		}

		// Push into DataChannel
		if s.dc.ReadyState() == webrtc.DataChannelStateOpen {
			if err := s.dc.Send(buf.Bytes()); err != nil {
				log.Printf("[screenshare] datachannel send error: %v", err)
			}
		}
	}
}
