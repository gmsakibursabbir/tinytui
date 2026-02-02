package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DirName     = "tinytui"
	FileName    = "history.json"
	PermDir     = 0755 // Config is 0700 but history can be 755 usually, but let's stick to user privacy if needed. standard state is 700 or 755.
	PermFile    = 0644
)

type Record struct {
	Timestamp      time.Time `json:"timestamp"`
	File           string    `json:"file"`
	BeforeSize     int64     `json:"before_size"`
	AfterSize      int64     `json:"after_size"`
	SavedBytes     int64     `json:"saved_bytes"`
	SavedPercent   float64   `json:"saved_percent"`
	Status         string    `json:"status"` // "success", "failed"
	Error          string    `json:"error,omitempty"`
}

type Manager struct {
	records []*Record
	mu      sync.RWMutex
	path    string
}

func New() (*Manager, error) {
	// os.UserStateDir was added in Go 1.21, but if environment is older or issue exists:
	// Use manual construction: ~/.local/state
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	stateDir := filepath.Join(home, ".local", "state")
	
	path := filepath.Join(stateDir, DirName, FileName)
	m := &Manager{
		path: path,
	}
	if err := m.Load(); err != nil {
		// Only return error if it's NOT just file missing
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return m, nil
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.records)
}

func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	
	data, err := json.MarshalIndent(m.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, PermFile)
}

func (m *Manager) Add(r *Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, r)
	// We might want to auto-save or flush periodically.
	// For simplicity, let's auto-save on Add? Or caller manages Save().
	// Prompt says "Save history to: ~/.local/state/tinytui/history.json".
	// Let's autosave for CLI usage safety.
	// Launch goroutine to save implementation detail? No, keep simple. 
	// Just ignore error in Add for now or log it?
	go m.Save()
}

func (m *Manager) All() []*Record {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*Record, len(m.records))
	copy(res, m.records)
	return res
}

func (m *Manager) ExportCSV(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	
	// Write Header
	// File, Before, After, Saved, %, Status, Time
	fmt.Fprintln(f, "File,Before_Size,After_Size,Saved_Bytes,Saved_Percent,Status,Timestamp,Error")
	
	for _, r := range m.records {
		fmt.Fprintf(f, "%q,%d,%d,%d,%.2f,%s,%s,%q\n",
			r.File, r.BeforeSize, r.AfterSize, r.SavedBytes, r.SavedPercent, r.Status, r.Timestamp.Format(time.RFC3339), r.Error)
	}
	return nil
}

func (m *Manager) ExportJSON(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, err := json.MarshalIndent(m.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
