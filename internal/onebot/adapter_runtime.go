package onebot

// 每账号适配器运行时管理
// 当账号启动时，加载并启动该账号的所有适配器连接
// 当账号停止时，关闭所有适配器连接

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 自定义 WebSocket 拨号器：跳过代理、支持域名解析、宽松 TLS
var adapterDialer = &websocket.Dialer{
	Proxy:            nil, // 不走系统代理
	HandshakeTimeout: 15 * time.Second,
	TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
	NetDialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
}

// logLine 格式化日志行
func logLine(msg string) string {
	return time.Now().Format("15:04:05") + " " + msg
}

// AdapterRuntime 单个适配器的运行时连接
type AdapterRuntime struct {
	adapter   *Adapter
	srv       *Server
	accountID string
	status    string // disabled / connecting / open / retry / stopped
	send      chan []byte
	stopCh    chan struct{}
	mu        sync.Mutex
	conn      *websocket.Conn // reverse WS only
}

func newAdapterRuntime(a *Adapter, srv *Server, accountID string) *AdapterRuntime {
	return &AdapterRuntime{
		adapter:   a,
		srv:       srv,
		accountID: accountID,
		status:    "disabled",
		send:      make(chan []byte, sendChanBuffer),
		stopCh:    make(chan struct{}),
	}
}

func (ar *AdapterRuntime) setStatus(st string) {
	ar.mu.Lock()
	old := ar.status
	ar.status = st
	ar.mu.Unlock()
	if old != st {
		log.Printf("[适配器] %s (%s) -> %s", ar.adapter.Name, ar.adapter.Type, st)
		if ar.srv.SSE != nil && ar.accountID != "" {
			ar.srv.SSE.BroadcastLog(ar.accountID, logLine("[适配器] "+ar.adapter.Name+" ("+string(ar.adapter.Type)+") -> "+st))
		}
	}
}

func (ar *AdapterRuntime) getStatus() string {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.status
}

func (ar *AdapterRuntime) Stop() {
	select {
	case <-ar.stopCh:
		return
	default:
	}
	close(ar.stopCh)
	ar.mu.Lock()
	if ar.conn != nil {
		_ = ar.conn.Close()
	}
	ar.mu.Unlock()
	ar.setStatus("stopped")
}

// Start 根据适配器类型启动连接
func (ar *AdapterRuntime) Start() {
	if !ar.adapter.Enabled {
		ar.setStatus("disabled")
		return
	}
	switch ar.adapter.Type {
	case AdapterReverseWS:
		go ar.runReverseWS()
	case AdapterHTTPServer:
		// HTTP Server 模式：接收外部请求，不做主动连接
		ar.setStatus("open")
	case AdapterForwardWS:
		// Forward WS：等待外部客户端连入，不需要主动连接
		ar.setStatus("open")
	}
}

// SendEvent 向适配器推送事件
func (ar *AdapterRuntime) SendEvent(data []byte) {
	if !ar.adapter.Enabled {
		return
	}
	switch ar.adapter.Type {
	case AdapterReverseWS:
		ar.trySend(data)
	case AdapterHTTPServer:
		go ar.postEvent(data)
	// Forward WS 由 Server.hub 管理，不走这里
	}
}

func (ar *AdapterRuntime) trySend(data []byte) {
	select {
	case ar.send <- data:
	default:
	}
}

// --- Reverse WS ---

func (ar *AdapterRuntime) runReverseWS() {
	for {
		select {
		case <-ar.stopCh:
			return
		default:
		}
		if err := ar.dialReverseOnce(); err != nil {
			ar.setStatus("retry")
			log.Printf("[适配器] %s (%s) 连接失败: %v (5s后重试)", ar.adapter.Name, ar.adapter.URL, err)
			if ar.srv.SSE != nil && ar.accountID != "" {
				ar.srv.SSE.BroadcastLog(ar.accountID, logLine("[适配器] "+ar.adapter.Name+" 连接失败: "+err.Error()+" (5s后重试)"))
			}
			select {
			case <-ar.stopCh:
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (ar *AdapterRuntime) dialReverseOnce() error {
	ar.setStatus("connecting")
	targetURL := ar.adapter.URL
	log.Printf("[适配器] %s 正在连接 %s", ar.adapter.Name, targetURL)
	if ar.srv.SSE != nil && ar.accountID != "" {
		ar.srv.SSE.BroadcastLog(ar.accountID, logLine("[适配器] "+ar.adapter.Name+" 正在连接 "+targetURL))
	}
	// OneBot v11 规范：Token 通过 Authorization Header 传输
	headers := http.Header{}
	headers.Set("X-Self-ID", ar.accountID)
	headers.Set("X-Client-Role", "Universal")
	headers.Set("User-Agent", "OneBot/11 (mahiru) Mahiru-DyBot/1.0")
	if ar.adapter.Token != "" {
		headers.Set("Authorization", "Bearer "+ar.adapter.Token)
	}
	conn, _, err := adapterDialer.Dial(targetURL, headers)
	if err != nil {
		log.Printf("[适配器] %s 连接失败: %v", ar.adapter.Name, err)
		return err
	}
	ar.mu.Lock()
	ar.conn = conn
	ar.mu.Unlock()
	ar.setStatus("open")
	log.Printf("[适配器] %s 已连接 %s", ar.adapter.Name, ar.adapter.URL)
	if ar.srv.SSE != nil && ar.accountID != "" {
		ar.srv.SSE.BroadcastLog(ar.accountID, logLine("[适配器] "+ar.adapter.Name+" 已连接 "+ar.adapter.URL))
	}

	// OneBot v11 规范：连接成功后发送 lifecycle/connect 生命周期事件
	selfID, _ := parseI64(ar.accountID)
	connectEvent := map[string]interface{}{
		"time":            time.Now().Unix(),
		"self_id":         selfID,
		"post_type":       "meta_event",
		"meta_event_type": "lifecycle",
		"sub_type":        "connect",
	}
	if connectJSON, err := json.Marshal(connectEvent); err == nil {
		ar.trySend(connectJSON)
		log.Printf("[适配器] %s 已发送 lifecycle/connect 事件", ar.adapter.Name)
	}

	defer func() {
		ar.mu.Lock()
		if ar.conn != nil {
			_ = ar.conn.Close()
			ar.conn = nil
		}
		ar.mu.Unlock()
	}()

	stop := make(chan struct{})
	done := make(chan struct{})
	go writePump(conn, ar.send, stop)
	defer func() {
		close(stop)
		<-done
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			ar.setStatus("retry")
			return err
		}
		var frame WSActionFrame
		if err := json.Unmarshal(raw, &frame); err != nil || frame.Action == "" {
			resp, _ := json.Marshal(failResult(RetCodeBadRequest, "请求帧格式错误", nil))
			ar.trySend(resp)
			continue
		}
		result := ar.srv.Dispatch(frame.Action, frame.Params, frame.Echo)
		out, _ := json.Marshal(result)
		ar.trySend(out)
	}
}

// --- HTTP POST ---

func (ar *AdapterRuntime) postEvent(data []byte) {
	if ar.adapter.URL == "" {
		return
	}
	req, err := http.NewRequest("POST", ar.adapter.URL, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if ar.adapter.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ar.adapter.Token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[适配器] %s POST 失败: %v", ar.adapter.Name, err)
		return
	}
	_ = resp.Body.Close()
}

// --- Per-Account Adapter Manager ---

// AccountAdapterManager 管理单个账号的所有适配器运行时
type AccountAdapterManager struct {
	accountID string
	srv       *Server
	mu        sync.RWMutex
	adapters  map[string]*AdapterRuntime
	stopCh    chan struct{}
}

func NewAccountAdapterManager(accountID string, srv *Server) *AccountAdapterManager {
	return &AccountAdapterManager{
		accountID: accountID,
		srv:       srv,
		adapters:  make(map[string]*AdapterRuntime),
		stopCh:    make(chan struct{}),
	}
}

// Start 加载并启动该账号的所有适配器
func (am *AccountAdapterManager) Start(accountDir string) {
	am.loadAndStart(accountDir)
}

func (am *AccountAdapterManager) loadAndStart(accountDir string) {
	aa := NewAccountAdapters(accountDir)
	for _, a := range aa.List() {
		ar := newAdapterRuntime(a, am.srv, am.accountID)
		am.mu.Lock()
		am.adapters[a.ID] = ar
		am.mu.Unlock()
		ar.Start()
	}
}

// Stop 停止所有适配器
func (am *AccountAdapterManager) Stop() {
	am.mu.Lock()
	defer am.mu.Unlock()
	for id, ar := range am.adapters {
		ar.Stop()
		delete(am.adapters, id)
	}
}

// SendEvent 向该账号的所有适配器推送事件
func (am *AccountAdapterManager) SendEvent(data []byte) {
	am.mu.RLock()
	targets := make([]*AdapterRuntime, 0, len(am.adapters))
	for _, ar := range am.adapters {
		targets = append(targets, ar)
	}
	am.mu.RUnlock()
	for _, ar := range targets {
		ar.SendEvent(data)
	}
}

// Statuses 返回该账号所有适配器的状态
func (am *AccountAdapterManager) Statuses() []map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(am.adapters))
	for id, ar := range am.adapters {
		out = append(out, map[string]interface{}{
			"id":     id,
			"name":   ar.adapter.Name,
			"type":   ar.adapter.Type,
			"url":    ar.adapter.URL,
			"status": ar.getStatus(),
		})
	}
	return out
}

// Reload 重新加载适配器配置（热更新）
func (am *AccountAdapterManager) Reload(accountDir string) {
	am.Stop()
	am.loadAndStart(accountDir)
}
