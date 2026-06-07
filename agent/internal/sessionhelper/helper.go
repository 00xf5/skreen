package sessionhelper

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"scon/agent/internal/control"
	"scon/agent/internal/screenshare"

	"github.com/pion/webrtc/v3"
)

type HelperInput struct {
	Type       string             `json:"type"`
	ICEServers []webrtc.ICEServer `json:"ice_servers,omitempty"`
	SDP        string             `json:"sdp,omitempty"`
	Candidate  string             `json:"candidate,omitempty"`
	Event      string             `json:"event,omitempty"` // mouse event
	X          float64            `json:"x,omitempty"`
	Y          float64            `json:"y,omitempty"`
	Button     string             `json:"button,omitempty"`
	KeyState   string             `json:"key_state,omitempty"`
	Key        string             `json:"key,omitempty"`
	Data       interface{}        `json:"data,omitempty"`
}

type HelperOutput struct {
	Type      string `json:"type"`
	SDP       string `json:"sdp,omitempty"`
	Candidate string `json:"candidate,omitempty"`
	Displays  int    `json:"displays,omitempty"`
	Error     string `json:"error,omitempty"`
}

func sendOutput(msg HelperOutput) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	fmt.Println(string(data))
}

func RunSessionHelper() {
	// Log to stderr so we don't pollute stdout (which is used for JSON IPC)
	log.SetOutput(os.Stderr)
	log.Println("[helper] Helper process starting...")

	controlMgr := control.NewManager()
	defer controlMgr.Close()

	var activeSess *screenshare.Session

	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				log.Println("[helper] Stdin closed, exiting...")
				break
			}
			log.Printf("[helper] Read error: %v", err)
			break
		}

		var input HelperInput
		if err := json.Unmarshal(line, &input); err != nil {
			log.Printf("[helper] Failed to unmarshal input: %v", err)
			continue
		}

		switch input.Type {
		case "start_stream":
			log.Println("[helper] Starting WebRTC stream session...")
			if activeSess != nil {
				activeSess.Stop()
			}

			sess, err := screenshare.NewSession(
				input.ICEServers,
				func(candidateJSON string) {
					sendOutput(HelperOutput{
						Type:      "ice_candidate",
						Candidate: candidateJSON,
					})
				},
				func(state webrtc.ICEConnectionState) {
					if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
						sendOutput(HelperOutput{Type: "stream_stopped"})
					}
				},
			)
			if err != nil {
				log.Printf("[helper] Failed to create screenshare session: %v", err)
				sendOutput(HelperOutput{Type: "error", Error: err.Error()})
				continue
			}

			// Capture loop switches to the hidden desktop context if set
			sess.IsHidden = controlMgr.IsHidden

			activeSess = sess

			sdp, err := sess.CreateOffer()
			if err != nil {
				log.Printf("[helper] Failed to create offer: %v", err)
				sendOutput(HelperOutput{Type: "error", Error: err.Error()})
				sess.Stop()
				activeSess = nil
				continue
			}

			sendOutput(HelperOutput{
				Type: "webrtc_offer",
				SDP:  sdp,
			})

		case "webrtc_answer":
			if activeSess != nil {
				if err := activeSess.SetAnswer(input.SDP); err != nil {
					log.Printf("[helper] SetAnswer failed: %v", err)
					sendOutput(HelperOutput{Type: "error", Error: err.Error()})
				} else {
					activeSess.StartCapture()
					sendOutput(HelperOutput{
						Type:     "stream_ready",
						Displays: screenshare.NumDisplays(),
					})
				}
			}

		case "ice_candidate":
			if activeSess != nil && input.Candidate != "" {
				activeSess.AddICECandidate(input.Candidate)
			}

		case "input_mouse":
			controlMgr.HandleMouse(input.Event, input.X, input.Y, input.Button, input.KeyState)

		case "input_keyboard":
			controlMgr.HandleKeyboard(input.Key, input.KeyState)

		case "input_special":
			controlMgr.HandleSpecialKey(input.Key)

		case "block_input":
			if block, ok := input.Data.(bool); ok {
				controlMgr.SetBlockInput(block)
			}

		case "set_display":
			if activeSess != nil {
				if idx, ok := input.Data.(float64); ok {
					activeSess.SetDisplay(int(idx))
				}
			}

		case "set_hidden_mode":
			if hidden, ok := input.Data.(bool); ok {
				controlMgr.SetHiddenMode(hidden)
			}

		case "stop_stream":
			log.Println("[helper] Stopping WebRTC session...")
			if activeSess != nil {
				activeSess.Stop()
				activeSess = nil
			}
			sendOutput(HelperOutput{Type: "stream_stopped"})
		}
	}

	if activeSess != nil {
		activeSess.Stop()
	}
	log.Println("[helper] Helper process exiting.")
}
