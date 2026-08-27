package onebot

// SSE (Server-Sent Events) 推送支持

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// SSEClient 表示一个 SSE 客户端连接
type SSEClient struct {
	ID     string
	Chan   chan []byte
	Cancel context.CancelFunc
}

// SSEManager 管理所有 SSE 客户端连接
type SSEManager struct {
	mu      sync.RWMutex
	clients map[string]*SSEClient
	logs    map[string][]string // account_id -> log buffer
}

// NewSSEManager 创建新的 SSE 管理器
func NewSSEManager() *SSEManager {
	return &SSEManager{
		clients: make(map[string]*SSEClient),
		logs:    make(map[string][]string),
	}
}

// Subscribe 订阅 SSE 事件
func (m *SSEManager) Subscribe(clientID string, ctx context.Context) (<-chan []byte, context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已存在，先关闭旧的
	if old, ok := m.clients[clientID]; ok {
		close(old.Chan)
		delete(m.clients, clientID)
	}

	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan []byte, 100) // 缓冲区大小 100

	m.clients[clientID] = &SSEClient{
		ID:     clientID,
		Chan:   ch,
		Cancel: cancel,
	}

	// 启动清理 goroutine
	go func() {
		<-ctx.Done()
		m.mu.Lock()
		defer m.mu.Unlock()
		if c, ok := m.clients[clientID]; ok {
			close(c.Chan)
			delete(m.clients, clientID)
		}
	}()

	return ch, cancel
}

// Unsubscribe 取消订阅
func (m *SSEManager) Unsubscribe(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[clientID]; ok {
		c.Cancel()
		close(c.Chan)
		delete(m.clients, clientID)
	}
}

// Broadcast 向所有客户端广播事件
func (m *SSEManager) Broadcast(eventType string, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event := SSEEvent{
		Type: eventType,
		Data: data,
	}
	payload, _ := json.Marshal(event)

	for _, client := range m.clients {
		select {
		case client.Chan <- payload:
		default:
			// 客户端消费太慢，跳过
			log.Printf("[SSE] 客户端 %s 消费过慢，跳过事件", client.ID)
		}
	}
}

// BroadcastTo 向特定客户端发送事件
func (m *SSEManager) BroadcastTo(clientID string, eventType string, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[clientID]
	if !ok {
		return
	}

	event := SSEEvent{
		Type: eventType,
		Data: data,
	}
	payload, _ := json.Marshal(event)

	select {
	case client.Chan <- payload:
	default:
		log.Printf("[SSE] 客户端 %s 消费过慢，跳过事件", clientID)
	}
}

// SSEEvent SSE 事件结构
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// handleSSE 处理 SSE 连接请求
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// 验证 token
	token := bearerToken(r)
	if !s.Auth.Validate(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

	// 生成客户端 ID
	clientID := generateClientID()

	// 订阅事件
	ch, cancel := s.SSE.Subscribe(clientID, r.Context())
	defer func() {
		cancel()
		s.SSE.Unsubscribe(clientID)
	}()

	// 发送初始连接成功事件
	initEvent := SSEEvent{
		Type: "connected",
		Data: map[string]interface{}{
			"client_id": clientID,
			"timestamp": time.Now().Unix(),
		},
	}
	initPayload, _ := json.Marshal(initEvent)
	w.Write([]byte("data: " + string(initPayload) + "\n\n"))
	w.(http.Flusher).Flush()

	log.Printf("[SSE] 客户端已连接: %s", clientID)

	// 保持连接并推送事件
	for {
		select {
		case <-r.Context().Done():
			log.Printf("[SSE] 客户端已断开: %s", clientID)
			return
		case payload, ok := <-ch:
			if !ok {
				return
			}
			w.Write([]byte("data: " + string(payload) + "\n\n"))
			w.(http.Flusher).Flush()
		}
	}
}

// generateClientID 生成客户端 ID
func generateClientID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(8)
}

// randomHex 生成随机十六进制字符串
func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1) // 确保随机性
	}
	return string(b)
}

// BroadcastAccountStatus 广播账号状态变更
func (s *Server) BroadcastAccountStatus(accountID string, status string) {
	s.SSE.Broadcast("account_status", map[string]interface{}{
		"account_id": accountID,
		"status":     status,
		"timestamp":  time.Now().Unix(),
	})
}

// BroadcastSDKReady 广播 SDK 就绪事件
func (s *Server) BroadcastSDKReady(accountID string) {
	s.SSE.Broadcast("sdk_ready", map[string]interface{}{
		"account_id": accountID,
		"timestamp":  time.Now().Unix(),
	})
}

// BroadcastMessage 广播新消息事件
func (s *Server) BroadcastMessage(accountID string, message interface{}) {
	s.SSE.Broadcast("message", map[string]interface{}{
		"account_id": accountID,
		"message":    message,
		"timestamp":  time.Now().Unix(),
	})
}

// BroadcastError 广播错误事件
func (s *Server) BroadcastError(accountID string, err error) {
	s.SSE.Broadcast("error", map[string]interface{}{
		"account_id": accountID,
		"error":      err.Error(),
		"timestamp":  time.Now().Unix(),
	})
}

// BroadcastLog 广播日志事件并缓存
func (m *SSEManager) BroadcastLog(accountID string, message string) {
	m.mu.Lock()
	if m.logs[accountID] == nil {
		m.logs[accountID] = make([]string, 0, 200)
	}
	m.logs[accountID] = append(m.logs[accountID], message)
	if len(m.logs[accountID]) > 200 {
		m.logs[accountID] = m.logs[accountID][len(m.logs[accountID])-200:]
	}
	m.mu.Unlock()

	m.Broadcast("log", map[string]interface{}{
		"account_id": accountID,
		"message":    message,
		"timestamp":  time.Now().Unix(),
	})
}

// GetLogs 获取账号的日志缓冲
func (m *SSEManager) GetLogs(accountID string, limit int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	logs := m.logs[accountID]
	if len(logs) == 0 {
		return []string{}
	}
	if limit > len(logs) {
		limit = len(logs)
	}
	result := make([]string, limit)
	copy(result, logs[len(logs)-limit:])
	return result
}
