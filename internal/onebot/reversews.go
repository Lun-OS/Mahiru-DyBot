package onebot

// 反向 WebSocket：本服务作为客户端主动外连远端 OneBot 框架（如 NoneBot）。
// 通用模式：同一连接上既推送事件，也接收并处理 {action, params, echo} 请求帧。
// 每条配置独立连接、独立重连（5s）、独立开关；与正向 WS 可同时存在。

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"mahiru-dybot/internal/config"
	"mahiru-dybot/internal/eventbus"
)

const reverseRetryInterval = 5 * time.Second

// ReverseConn 单条反向连接。
type ReverseConn struct {
	cfg    config.ReverseWSConfig
	srv    *Server
	mu     sync.Mutex
	status string // disabled / connecting / open / retry
	conn   *websocket.Conn
	send   chan []byte
	stopCh chan struct{}
}

func (rc *ReverseConn) setStatus(st string) {
	rc.mu.Lock()
	if rc.status == st {
		rc.mu.Unlock()
		return
	}
	rc.status = st
	cfg := rc.cfg
	rc.mu.Unlock()
	log.Printf("[反WS] %s -> %s", cfg.URL, st)
	if rc.srv.Bus != nil {
		rc.srv.Bus.Publish(eventbus.TopicReverseWS, map[string]interface{}{
			"id": cfg.ID, "url": cfg.URL, "status": st,
		})
	}
}

// Stop 断开并退出重连循环。
func (rc *ReverseConn) Stop() {
	select {
	case <-rc.stopCh:
		return
	default:
	}
	close(rc.stopCh)
	rc.mu.Lock()
	if rc.conn != nil {
		_ = rc.conn.Close()
	}
	rc.mu.Unlock()
}

// run 连接主循环：拨号 → 收发 → 断线5s重连，直至 stopCh 关闭。
func (rc *ReverseConn) run() {
	for {
		select {
		case <-rc.stopCh:
			return
		default:
		}
		if err := rc.dialOnce(); err != nil {
			rc.setStatus("retry")
			log.Printf("[反WS] %s 连接失败: %v (%ds后重试)", rc.cfg.URL, err, int(reverseRetryInterval.Seconds()))
			select {
			case <-rc.stopCh:
				return
			case <-time.After(reverseRetryInterval):
			}
		}
	}
}

// dialURL 返回原始连接地址（Token 通过 Header 传输，不再拼接到 URL）。
func (rc *ReverseConn) dialURL() string {
	return rc.cfg.URL
}

func (rc *ReverseConn) dialOnce() error {
	rc.setStatus("connecting")
	// OneBot v11 规范：Token 通过 Authorization Header 传输
	headers := http.Header{}
	headers.Set("X-Self-ID", rc.cfg.ID)
	headers.Set("X-Client-Role", "Universal")
	headers.Set("User-Agent", "OneBot/11 (mahiru) Mahiru-DyBot/1.0")
	if rc.cfg.AccessToken != "" {
		headers.Set("Authorization", "Bearer "+rc.cfg.AccessToken)
	}
	conn, _, err := adapterDialer.Dial(rc.cfg.URL, headers)
	if err != nil {
		return err
	}
	rc.mu.Lock()
	rc.conn = conn
	rc.mu.Unlock()
	rc.setStatus("open")
	log.Printf("[反WS] 已连接 %s", rc.cfg.URL)

	// OneBot v11 规范：连接成功后发送 lifecycle/connect 生命周期事件
	selfID, _ := parseI64(rc.cfg.ID)
	connectEvent := map[string]interface{}{
		"time":            time.Now().Unix(),
		"self_id":         selfID,
		"post_type":       "meta_event",
		"meta_event_type": "lifecycle",
		"sub_type":        "connect",
	}
	if connectJSON, err := json.Marshal(connectEvent); err == nil {
		rc.trySend(connectJSON)
	}

	defer func() {
		rc.mu.Lock()
		if rc.conn != nil {
			_ = rc.conn.Close()
			rc.conn = nil
		}
		rc.mu.Unlock()
	}()

	stop := make(chan struct{})
	done := make(chan struct{})
	go writePump(conn, rc.send, stop)
	defer func() {
		close(stop)
		<-done
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// 读循环：处理远端下发的 action 请求
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			rc.setStatus("retry")
			return err
		}
		var frame WSActionFrame
		if err := json.Unmarshal(raw, &frame); err != nil || frame.Action == "" {
			resp, _ := json.Marshal(failResult(RetCodeBadRequest, "请求帧格式: {\"action\":..., \"params\":..., \"echo\":...}", nil))
			rc.trySend(resp)
			continue
		}
		result := rc.srv.Dispatch(frame.Action, frame.Params, frame.Echo)
		out, _ := json.Marshal(result)
		rc.trySend(out)
	}
}

func (rc *ReverseConn) trySend(data []byte) {
	select {
	case rc.send <- data:
	default:
	}
}

func (rc *ReverseConn) isOpen() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.status == "open"
}

// ReverseManager 反向连接集合管理。
type ReverseManager struct {
	srv   *Server
	mu    sync.Mutex
	conns map[string]*ReverseConn
}

func newReverseConn(cfg config.ReverseWSConfig, srv *Server) *ReverseConn {
	return &ReverseConn{
		cfg:    cfg,
		srv:    srv,
		status: "disabled",
		send:   make(chan []byte, sendChanBuffer),
		stopCh: make(chan struct{}),
	}
}

// NewReverseManager 构建并根据当前设置启动全部启用的反向连接；
// 同时注册设置观察者实现热更新（增删改即时生效）。
func NewReverseManager(srv *Server) *ReverseManager {
	rm := &ReverseManager{srv: srv, conns: map[string]*ReverseConn{}}
	srv.RT.On("reverse_ws", func(*config.RuntimeSettings) { rm.SyncFromSettings() })
	srv.RT.On("all", func(*config.RuntimeSettings) { rm.SyncFromSettings() })
	rm.SyncFromSettings()
	return rm
}

// SyncFromSettings 对齐运行中连接与配置期望状态。
func (rm *ReverseManager) SyncFromSettings() {
	desired := map[string]config.ReverseWSConfig{}
	for _, c := range rm.srv.RT.Get().ReverseWS {
		if c.Enabled && c.URL != "" {
			desired[c.ID] = c
		}
	}

	rm.mu.Lock()
	var toStart []config.ReverseWSConfig
	for id, rc := range rm.conns {
		cfg, keep := desired[id]
		if !keep || rc.cfg != cfg { // 移除/停用/配置变更 → 停旧
			rc.Stop()
			delete(rm.conns, id)
			if keep {
				toStart = append(toStart, cfg)
			}
		}
	}
	for id, cfg := range desired {
		if _, exists := rm.conns[id]; !exists {
			toStart = append(toStart, cfg)
		}
	}
	for _, cfg := range toStart {
		rc := newReverseConn(cfg, rm.srv)
		rm.conns[cfg.ID] = rc
		go rc.run()
	}
	rm.mu.Unlock()
}

// Statuses 返回各连接实时状态。
func (rm *ReverseManager) Statuses() []map[string]interface{} {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	out := make([]map[string]interface{}, 0, len(rm.conns))
	for id, rc := range rm.conns {
		rc.mu.Lock()
		st := rc.status
		url := rc.cfg.URL
		rc.mu.Unlock()
		out = append(out, map[string]interface{}{"id": id, "url": url, "status": st})
	}
	// 已配置但未运行的条目也要展示
	for _, c := range rm.srv.RT.Get().ReverseWS {
		found := false
		for _, st := range out {
			if st["id"] == c.ID {
				found = true
				break
			}
		}
		if !found {
			status := "disabled"
			if c.Enabled {
				status = "connecting"
			}
			out = append(out, map[string]interface{}{"id": c.ID, "url": c.URL, "status": status})
		}
	}
	return out
}

func (rm *ReverseManager) broadcastRaw(data []byte) {
	rm.mu.Lock()
	targets := make([]*ReverseConn, 0, len(rm.conns))
	for _, rc := range rm.conns {
		targets = append(targets, rc)
	}
	rm.mu.Unlock()
	for _, rc := range targets {
		if rc.isOpen() {
			rc.trySend(data)
		}
	}
}
