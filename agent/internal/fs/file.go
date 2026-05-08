package fs

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"`
}

// Transfer handles a single file upload/download session
type Transfer struct {
	ID         string
	Action     string // "upload" or "download"
	File       *os.File
	Size       int64
	ChunkCount int
	Current    int
}

type Manager struct {
	mu        sync.Mutex
	transfers map[string]*Transfer

	// Callback to send messages back to server
	SendMsg func(msg interface{})
}

func NewManager(sendMsg func(msg interface{})) *Manager {
	return &Manager{
		transfers: make(map[string]*Transfer),
		SendMsg:   sendMsg,
	}
}

// StartTransfer initializes a new transfer session
func (m *Manager) StartTransfer(id, action, path string, size int64, chunkCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.transfers[id]; exists {
		return fmt.Errorf("transfer %s already exists", id)
	}

	var f *os.File
	var err error

	cleanPath := filepath.Clean(path)

	if action == "upload" {
		// We are receiving a file from controller
		f, err = os.Create(cleanPath)
		if err != nil {
			return err
		}
	} else if action == "download" {
		// We are sending a file to controller
		f, err = os.Open(cleanPath)
		if err != nil {
			return err
		}
		// In a real scenario we'd stat the file to confirm size, but we trust the controller's request size/count or we determine it here.
		// Actually, if it's a download request, the agent should determine size/chunks and reply.
		// For simplicity, we assume the controller just sent Action: "download" and Path, and the agent fills the rest in the ack.
		stat, statErr := f.Stat()
		if statErr == nil {
			size = stat.Size()
			// 1MB chunks
			chunkSize := int64(1024 * 1024)
			chunkCount = int((size + chunkSize - 1) / chunkSize)
		}
	} else {
		return fmt.Errorf("unknown action: %s", action)
	}

	m.transfers[id] = &Transfer{
		ID:         id,
		Action:     action,
		File:       f,
		Size:       size,
		ChunkCount: chunkCount,
		Current:    0,
	}

	return nil
}

// HandleChunk processes an incoming chunk (for uploads)
func (m *Manager) HandleChunk(id string, index int, dataB64 string) error {
	m.mu.Lock()
	t, exists := m.transfers[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("transfer %s not found", id)
	}

	if t.Action != "upload" {
		return fmt.Errorf("transfer %s is not an upload", id)
	}

	decoded, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return err
	}

	// We don't strictly enforce sequential chunking in this MVP, we assume TCP WebSocket order is guaranteed
	_, err = t.File.Write(decoded)
	if err != nil {
		return err
	}

	m.mu.Lock()
	t.Current++
	done := t.Current >= t.ChunkCount
	m.mu.Unlock()

	if done {
		t.File.Close()
		m.mu.Lock()
		delete(m.transfers, id)
		m.mu.Unlock()
		log.Printf("[fs] Transfer %s completed successfully", id)
	}

	return nil
}

// SendNextChunk reads the next chunk and sends it (for downloads)
func (m *Manager) SendNextChunk(id string) error {
	m.mu.Lock()
	t, exists := m.transfers[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("transfer %s not found", id)
	}

	if t.Action != "download" {
		return fmt.Errorf("transfer %s is not a download", id)
	}

	// 1MB chunk
	buf := make([]byte, 1024*1024)
	n, err := t.File.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}

	if n > 0 {
		m.mu.Lock()
		currentIndex := t.Current
		t.Current++
		m.mu.Unlock()

		b64 := base64.StdEncoding.EncodeToString(buf[:n])
		
		// This uses the callback to send the message
		// Since we can't easily import the types package without circular dependencies sometimes,
		// we just pass an interface{} that the caller will cast.
		// Wait, we can import it in the manager.
		m.SendMsg(map[string]interface{}{
			"type": "file_chunk",
			"transfer_id": id,
			"chunk_index": currentIndex,
			"chunk_data": b64,
		})
	}

	if err == io.EOF || n == 0 || t.Current >= t.ChunkCount {
		t.File.Close()
		m.mu.Lock()
		delete(m.transfers, id)
		m.mu.Unlock()
		log.Printf("[fs] Download %s completed", id)
	}

	return nil
}

// CancelTransfer aborts and cleans up a transfer
func (m *Manager) CancelTransfer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t, exists := m.transfers[id]; exists {
		t.File.Close()
		delete(m.transfers, id)
		log.Printf("[fs] Transfer %s cancelled", id)
	}
}

// GetTransfer returns transfer info
func (m *Manager) GetTransfer(id string) *Transfer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.transfers[id]
}

// ListDir returns a list of files in the specified path
func (m *Manager) ListDir(path string) ([]FileInfo, error) {
	if path == "" || path == "/" {
		if runtime.GOOS == "windows" {
			return m.listWindowsDrives()
		}
		path = "/"
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		modTime := int64(0)
		if info != nil {
			size = info.Size()
			modTime = info.ModTime().Unix()
		}
		files = append(files, FileInfo{
			Name:    e.Name(),
			Size:    size,
			IsDir:   e.IsDir(),
			ModTime: modTime,
		})
	}
	return files, nil
}

func (m *Manager) listWindowsDrives() ([]FileInfo, error) {
	var drives []FileInfo
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		d := string(drive) + ":\\"
		if _, err := os.Stat(d); err == nil {
			drives = append(drives, FileInfo{
				Name:  d,
				IsDir: true,
			})
		}
	}
	return drives, nil
}

// FileOp performs file operations like delete/rename
func (m *Manager) FileOp(action, path, newPath string) error {
	switch action {
	case "delete":
		return os.RemoveAll(path)
	case "rename":
		if newPath == "" {
			return fmt.Errorf("new path required for rename")
		}
		return os.Rename(path, newPath)
	default:
		return fmt.Errorf("unknown file op: %s", action)
	}
}
