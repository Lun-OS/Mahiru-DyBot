package onebot

// 正向 WebSocket：客户端连入 /event。
// 全双工：服务端持续推送事件，客户端也可发送 {action, params, echo} 请求帧并接收回帧。

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"mahiru-dybot/internal/browser"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second // OneBot v11 标准：30s ping 间隔
	sendChanBuffer = 256
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsClient struct {
	conn   *websocket.Conn
	send   chan []byte
	remote string
}

func (c *wsClient) trySend(data []byte) {
	select {
	case c.send <- data:
	default: // 队列满丢弃，防止慢客户端阻塞广播方
	}
}

func (c *wsClient) closeSend() {
	select {
	case <-c.send:
		return
	default:
	}
	close(c.send)
}

// writePump 单写者协程：从 send 通道取帧写出，附带 ping 保活。
func writePump(conn *websocket.Conn, send <-chan []byte, stop <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer func() { ticker.Stop(); _ = conn.Close() }()
	for {
		select {
		case data, ok := <-send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-stop:
			return
		}
	}
}

// checkOnebotAuth 校验 OneBot 访问令牌（运行时设置热生效）。
func (s *Server) checkOnebotAuth(r *http.Request) bool {
	token := s.RT.Get().OneBotAccessToken
	if token == "" {
		return true
	}
	got := r.URL.Query().Get("access_token")
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		got = strings.TrimPrefix(a, "Bearer ")
	}
	if got == "" {
		got = r.Header.Get("access_token")
	}
	return got == token
}

// handleWS 正向 WebSocket 接入点。
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if !s.checkOnebotAuth(r) {
		http.Error(w, "access_token 无效", http.StatusForbidden)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(4 << 20)
	client := &wsClient{conn: conn, send: make(chan []byte, sendChanBuffer), remote: r.RemoteAddr}

	s.mu.Lock()
	s.hub[client] = struct{}{}
	s.mu.Unlock()
	log.Printf("[正WS] 客户端接入 %s", conn.RemoteAddr())

	stop := make(chan struct{})
	go writePump(conn, client.send, stop)

	// 连接生命周期事件：为每个在线账号发送 lifecycle/connect
	accounts := s.BM.List()
	if len(accounts) == 0 {
		// 没有账号时发送默认的 self_id=0 连接事件（兼容性）
		lc, _ := json.Marshal(map[string]interface{}{
			"time":            time.Now().Unix(),
			"self_id":         0,
			"post_type":       "meta_event",
			"meta_event_type": "lifecycle",
			"sub_type":        "connect",
			"ping_interval":   30000,
		})
		client.trySend(lc)
	} else {
		for _, accInfo := range accounts {
			uidInt := int64(0)
			// 尝试从运行中的实例获取 UID
			if acc, ok := s.BM.Get(accInfo.ID); ok {
				if inst := acc.Instance(); inst != nil {
					if uid := inst.SelfUID(); uid != "" {
						uidInt, _ = parseI64(uid)
					}
				}
			}
			// 回退到元数据中的 UID
			if uidInt == 0 && accInfo.UID != "" {
				uidInt, _ = parseI64(accInfo.UID)
			}
			lc, _ := json.Marshal(map[string]interface{}{
				"time":            time.Now().Unix(),
				"self_id":         uidInt,
				"post_type":       "meta_event",
				"meta_event_type": "lifecycle",
				"sub_type":        "connect",
				"ping_interval":   30000,
			})
			client.trySend(lc)
		}
	}

	defer func() {
		s.mu.Lock()
		delete(s.hub, client)
		s.mu.Unlock()
		close(stop)
		client.closeSend()
		_ = conn.Close()
		log.Printf("[正WS] 客户端断开 %s", conn.RemoteAddr())
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var frame WSActionFrame
		if err := json.Unmarshal(raw, &frame); err != nil || frame.Action == "" {
			resp, _ := json.Marshal(failResult(RetCodeBadRequest, "请求帧格式: {\"action\":..., \"params\":..., \"echo\":...}", nil))
			client.trySend(resp)
			continue
		}
		result := s.Dispatch(frame.Action, frame.Params, frame.Echo)
		out, _ := json.Marshal(result)
		client.trySend(out)
	}
}

// Broadcast 向所有事件通道（正向WS + 反向WS）推送原始 JSON 帧。
func (s *Server) Broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.broadcastRaw(data)
}

func (s *Server) broadcastRaw(data []byte) {
	s.mu.RLock()
	targets := make([]*wsClient, 0, len(s.hub))
	for c := range s.hub {
		targets = append(targets, c)
	}
	s.mu.RUnlock()
	for _, c := range targets {
		c.trySend(data)
	}
	if s.reverse != nil {
		s.reverse.broadcastRaw(data)
	}
}

// disconnectAllWSClients 断开所有 WS 客户端连接。
// 当账号 UID 变化时调用，强制客户端重新连接以获取新的 self_id。
func (s *Server) disconnectAllWSClients() {
	s.mu.Lock()
	clients := make([]*wsClient, 0, len(s.hub))
	for c := range s.hub {
		clients = append(clients, c)
	}
	// 清空 hub，这样后续的 Broadcast 不会发送到旧连接
	s.hub = map[*wsClient]struct{}{}
	s.mu.Unlock()

	for _, c := range clients {
		// 发送关闭帧
		c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "UID changed, please reconnect"),
			time.Now().Add(time.Second))
		c.conn.Close()
	}
	log.Printf("[WS] 已断开 %d 个客户端连接（UID 变化）", len(clients))
}

// rememberPrivateConv 记录 会话shortId -> 对端uid 映射（按账号隔离）。
func (s *Server) rememberPrivateConv(accountID, shortID, uid string) {
	if accountID == "" || shortID == "" || uid == "" {
		return
	}
	s.shortToUID.Store(accountID+"|"+shortID, uid)
}

// buildIncomingEvent 将浏览器上报消息转换为 OneBot 消息事件。
func (s *Server) buildIncomingEvent(acc *browser.AccountInfo, incoming *browser.IncomingMessage) *EventMessage {
	segments := buildSegments(incoming)

	uidStr := ""
	if v, ok := s.shortToUID.Load(acc.UID + "|" + incoming.ConversationShortID); ok {
		uidStr, _ = v.(string)
	}
	if uidStr == "" && incoming.Sender != "" && !incoming.IsFromMe {
		uidStr = incoming.Sender
		s.rememberPrivateConv(acc.UID, incoming.ConversationShortID, uidStr)
	}
	uid, _ := parseI64(uidStr)
	selfID, _ := parseI64(acc.UID)

	msgType, subType := "private", "friend"
	groupID := int64(0)
	if incoming.ConversationType == browser.ConversationTypeGroup {
		msgType, subType = "group", "normal"
		groupID, _ = parseI64(incoming.ConversationShortID)
	}

	postType := "message"
	senderID := uid
	senderNickname := incoming.SenderNickname
	if incoming.IsFromMe {
		postType = "message_sent"
		senderID = selfID
		if senderNickname == "" {
			senderNickname = acc.Nickname
		}
	}
	if senderNickname == "" {
		senderNickname = strconv.FormatInt(uid, 10)
	}

	rawMsg := formatCQCode(segments)
	msgID := hashStringID(incoming.ClientID)
	ev := &EventMessage{
		Time:        incoming.CreatedAt,
		SelfID:      selfID,
		PostType:    postType,
		MessageType: msgType,
		SubType:     subType,
		UserID:      senderID,
		GroupID:     groupID,
		Message:     rawMsg,
		RawMessage:  rawMsg,
		Sender: EventSender{
			UserID:   senderID,
			Nickname: senderNickname,
		},
		MessageID:  msgID,
		RealID:     msgID,
		MessageSeq: 1,
		Image:      incoming.Image,
		Sticker:    incoming.Sticker,
		RawContent: incoming.RawContent,
	}
	if incoming.IsFromMe {
		ev.MessageSentType = "self"
	}
	return ev
}

// hashStringID 将字符串 client_id 转换为 int64 消息ID
func hashStringID(s string) int64 {
	var h int64
	for i := 0; i < len(s); i++ {
		h = h*31 + int64(s[i])
	}
	if h < 0 {
		h = -h
	}
	if h == 0 {
		h = 1
	}
	return h
}

// Heartbeat 周期性心跳事件（为每个在线账号发送心跳）。
func (s *Server) Heartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for t := range ticker.C {
			accounts := s.BM.List()
			if len(accounts) == 0 {
				// 没有账号时发送默认心跳（兼容性）
				ev := EventMeta{
					Time:          t.Unix(),
					SelfID:        0,
					PostType:      "meta_event",
					MetaEventType: "heartbeat",
					Interval:      int(interval.Seconds()),
				}
				s.Broadcast(ev)
			} else {
				for _, accInfo := range accounts {
					uidInt := int64(0)
					// 尝试从运行中的实例获取 UID
					if acc, ok := s.BM.Get(accInfo.ID); ok {
						if inst := acc.Instance(); inst != nil {
							if uid := inst.SelfUID(); uid != "" {
								uidInt, _ = parseI64(uid)
							}
						}
					}
					// 回退到元数据中的 UID
					if uidInt == 0 && accInfo.UID != "" {
						uidInt, _ = parseI64(accInfo.UID)
					}
					ev := EventMeta{
						Time:          t.Unix(),
						SelfID:        uidInt,
						PostType:      "meta_event",
						MetaEventType: "heartbeat",
						Interval:      int(interval.Seconds()),
					}
					s.Broadcast(ev)
				}
			}
		}
	}()
}

// parseI64 宽松解析 int64。
func parseI64(s string) (int64, bool) {
	n, err := strconvParse(s)
	return n, err
}
