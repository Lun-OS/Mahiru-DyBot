package onebot

// 每个账号独立的连接适配器管理

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AdapterType string

const (
	AdapterForwardWS  AdapterType = "forward_ws"
	AdapterReverseWS  AdapterType = "reverse_ws"
	AdapterHTTPServer AdapterType = "http_server"
)

type Adapter struct {
	ID        string      `json:"id"`
	Type      AdapterType `json:"type"`
	Name      string      `json:"name"`
	URL       string      `json:"url"`
	Token     string      `json:"token"`
	Port      int         `json:"port,omitempty"`
	Enabled   bool        `json:"enabled"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// AccountAdapters 管理单个账号的适配器
type AccountAdapters struct {
	mu       sync.RWMutex
	adapters map[string]*Adapter
	dir      string // 账号目录路径
}

func NewAccountAdapters(accountDir string) *AccountAdapters {
	s := &AccountAdapters{
		adapters: make(map[string]*Adapter),
		dir:      accountDir,
	}
	s.load()
	return s
}

func (s *AccountAdapters) filePath() string {
	return filepath.Join(s.dir, "adapters.json")
}

func (s *AccountAdapters) load() {
	data, err := os.ReadFile(s.filePath())
	if err != nil {
		return
	}
	var list []*Adapter
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}
	for _, a := range list {
		s.adapters[a.ID] = a
	}
}

func (s *AccountAdapters) save() error {
	list := make([]*Adapter, 0, len(s.adapters))
	for _, a := range s.adapters {
		list = append(list, a)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(s.dir, 0755)
	return os.WriteFile(s.filePath(), data, 0644)
}

func (s *AccountAdapters) List() []*Adapter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Adapter, 0, len(s.adapters))
	for _, a := range s.adapters {
		result = append(result, a)
	}
	return result
}

func (s *AccountAdapters) Get(id string) (*Adapter, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.adapters[id]
	return a, ok
}

func (s *AccountAdapters) Create(a *Adapter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == "" {
		a.ID = time.Now().Format("0102150405") + "-" + randomHex(4)
	}
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	s.adapters[a.ID] = a
	return s.save()
}

func (s *AccountAdapters) Update(id string, update func(*Adapter)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.adapters[id]
	if !ok {
		return errAdapterNotFound
	}
	update(a)
	a.UpdatedAt = time.Now()
	return s.save()
}

func (s *AccountAdapters) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.adapters[id]; !ok {
		return errAdapterNotFound
	}
	delete(s.adapters, id)
	return s.save()
}

var errAdapterNotFound = &adapterError{"adapter not found"}

type adapterError struct{ msg string }

func (e *adapterError) Error() string { return e.msg }
