package debug

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var (
	enabled atomic.Bool
	mu      sync.Mutex
	file    *os.File
)

type record struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`
	Data      any       `json:"data,omitempty"`
}

func Enable() {
	enabled.Store(true)
}

func Enabled() bool {
	return enabled.Load()
}

func StartSession(pwd, sessionID string) error {
	if !enabled.Load() {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
	}
	path := filepath.Join(pwd, "dbg-"+sessionID)
	opened, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		file = nil
		return err
	}
	file = opened
	return writeLocked(record{
		Time:      time.Now(),
		SessionID: sessionID,
		Type:      "session_start",
		Data:      map[string]any{"pwd": pwd},
	})
}

func Record(kind string, data any) {
	if !enabled.Load() {
		return
	}

	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return
	}
	_ = writeLocked(record{
		Time:      time.Now(),
		SessionID: currentSessionID(),
		Type:      kind,
		Data:      data,
	})
}

func writeLocked(value record) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = file.Write(encoded)
	return err
}

func currentSessionID() string {
	if file == nil {
		return ""
	}
	return filepath.Base(file.Name())[len("dbg-"):]
}
