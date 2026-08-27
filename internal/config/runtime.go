package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ReverseWSConfig 单条反向 WebSocket 连接配置。
type ReverseWSConfig struct {
	ID          string `json:"id"`
	URL         string `json:"url"`                    // 例 ws://127.0.0.1:8080/onebot/v11/ws
	AccessToken string `json:"access_token,omitempty"` // 拨号时追加 ?access_token=
	Enabled     bool   `json:"enabled"`
}

// RuntimeSettings 运行时可热更新的设置（storage/runtime_settings.json）。
type RuntimeSettings struct {
	Version           int               `json:"version"`
	OneBotAccessToken string            `json:"onebot_access_token"`
	ScreenshotMaxFPS  int               `json:"screenshot_max_fps"`
	JpegQuality       int               `json:"jpeg_quality"` // UDP 画面流 JPEG 质量(1-95)，默认60
	ReverseWS         []ReverseWSConfig `json:"reverse_ws"`
}

func defaultRuntimeSettings() *RuntimeSettings {
	return &RuntimeSettings{
		Version:          1,
		ScreenshotMaxFPS: 10,
		JpegQuality:      60,
	}
}

// Watcher 配置变更回调。key 为变更关注点："onebot" / "reverse_ws" / "all"。
type Watcher func(s *RuntimeSettings)

// Runtime 线程安全的运行时设置容器，带观察者回调。
type Runtime struct {
	mu       sync.RWMutex
	path     string
	settings *RuntimeSettings
	watchers map[string][]Watcher
}

// LoadRuntime 从 path 加载运行时设置；文件不存在则用默认值。
func LoadRuntime(path string) (*Runtime, error) {
	rt := &Runtime{path: path, settings: defaultRuntimeSettings(), watchers: map[string][]Watcher{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rt, nil
		}
		return nil, fmt.Errorf("读取运行时设置失败: %w", err)
	}
	st := defaultRuntimeSettings()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("解析运行时设置失败: %w", err)
	}
	if st.ScreenshotMaxFPS <= 0 {
		st.ScreenshotMaxFPS = 10
	}
	if st.JpegQuality <= 0 || st.JpegQuality > 95 {
		st.JpegQuality = 60
	}
	rt.settings = st
	return rt, nil
}

// Get 返回设置快照。
func (r *Runtime) Get() *RuntimeSettings {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := *r.settings
	ws := make([]ReverseWSConfig, len(r.settings.ReverseWS))
	copy(ws, r.settings.ReverseWS)
	cp.ReverseWS = ws
	return &cp
}

// Update 原子修改并持久化，随后触发所有注册的 watcher（含 "all"）。
// 注意：不能在持锁状态下调用 Get()（RWMutex 不可重入），此处手动拷贝快照。
func (r *Runtime) Update(mutate func(*RuntimeSettings)) error {
	r.mu.Lock()
	snapshot := *r.settings
	ws := make([]ReverseWSConfig, len(r.settings.ReverseWS))
	copy(ws, r.settings.ReverseWS)
	snapshot.ReverseWS = ws
	mutate(&snapshot)
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	if snapshot.ScreenshotMaxFPS <= 0 {
		snapshot.ScreenshotMaxFPS = 10
	}
	if snapshot.JpegQuality <= 0 || snapshot.JpegQuality > 95 {
		snapshot.JpegQuality = 60
	}
	data, err := json.MarshalIndent(&snapshot, "", "  ")
	if err != nil {
		r.mu.Unlock()
		return err
	}
	_ = os.MkdirAll(filepath.Dir(r.path), 0o755)
	if err := os.WriteFile(r.path, data, 0o644); err != nil {
		r.mu.Unlock()
		return fmt.Errorf("写入运行时设置失败: %w", err)
	}
	stored := snapshot
	r.settings = &stored
	watchers := make([]Watcher, 0)
	for _, list := range r.watchers {
		watchers = append(watchers, list...)
	}
	r.mu.Unlock()
	for _, w := range watchers {
		w(&stored)
	}
	return nil
}

// On 注册变更观察者。key: "onebot"/"reverse_ws"/"screenshot"/"all"。
func (r *Runtime) On(key string, cb Watcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.watchers[key] = append(r.watchers[key], cb)
}

// NewID 生成带前缀的随机 ID（用于反向WS条目、账号等）。
func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// NowUnix 当前秒级时间戳。
func NowUnix() int64 { return time.Now().Unix() }
