package onebot

// Server：HTTP 路由总装。
//
// 路由分区：
//   /webui/*                     → React 静态前端（SPA）
//   /api/webui/*                 → WebUI 管理 API（webui 令牌鉴权）
//   /<action> (GET/POST 单段路径) → OneBot v11 标准 API（onebot access_token 鉴权）
//   /event                       → 正向 WebSocket（事件推送 + action 下发，全双工）
//   反向 WebSocket                → 由 ReverseManager 主动外连（配置驱动）

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"mahiru-dybot/internal/auth"
	"mahiru-dybot/internal/browser"
	"mahiru-dybot/internal/config"
	"mahiru-dybot/internal/eventbus"
)

// Server OneBot v11 + WebUI 管理服务。
type Server struct {
	Addr   string
	WsPath string
	UdpAddr string // 桌面客户端 UDP 网关地址（空=不启动）

	BM   *browser.AccountManager
	Auth *auth.Store
	RT   *config.Runtime
	Bus  *eventbus.Bus
	SSE  *SSEManager

	mu         sync.RWMutex
	hub        map[*wsClient]struct{}
	shortToUID sync.Map // "<accountID>|<convShortID>" -> 对端uid

	limiter        screenshotLimiter
	reverse        *ReverseManager
	adapterMgrs    map[string]*AccountAdapterManager // accountID -> AdapterManager
	loginMu        sync.Mutex                        // 登录流程串行化
	udp            *udpServer                        // 桌面客户端画面/控制网关
}

// NewServer 构建服务（不监听）。调用 Start 开始服务。
func NewServer(addr, udpAddr, wsPath string, bm *browser.AccountManager, authStore *auth.Store, rt *config.Runtime, bus *eventbus.Bus) *Server {
	s := &Server{
		Addr:    addr,
		UdpAddr: udpAddr,
		WsPath:  wsPath,
		BM:      bm,
		Auth:    authStore,
		RT:      rt,
		Bus:     bus,
		SSE:     NewSSEManager(),
		hub:     map[*wsClient]struct{}{},
		adapterMgrs: map[string]*AccountAdapterManager{},
	}
	return s
}

// Start 启动事件桥、反向WS、UDP网关、心跳与 HTTP 监听（阻塞）。
func (s *Server) Start() error {
	s.reverse = NewReverseManager(s)
	s.startEventBridge()
	s.Heartbeat(30 * time.Second)

	if s.UdpAddr != "" {
		s.udp = &udpServer{srv: s, sessions: map[string]*udpSession{}}
		go func() {
			if err := s.StartUDP(s.UdpAddr); err != nil {
				log.Printf("[ERROR] UDP 网关退出: %v", err)
			}
		}()
	}

	mux := http.NewServeMux()

	// ---------- WebUI 前端静态资源 ----------
	mux.HandleFunc("GET /webui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/webui/", http.StatusFound)
	})
	mux.HandleFunc("GET /webui/", spaHandler("webui"))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/webui/", http.StatusFound)
	})

	// ---------- WebUI 认证 ----------
	mux.HandleFunc("GET /api/webui/me", s.handleWebUIMe)
	mux.HandleFunc("POST /api/webui/auth/setup", s.handleWebUISetup)
	mux.HandleFunc("POST /api/webui/auth/login", s.handleWebUILogin)
	mux.HandleFunc("POST /api/webui/auth/verify", s.handleWebUIVerify)
	mux.HandleFunc("POST /api/webui/auth/reset", s.webuiAuthed(s.handleWebUIReset))

	// ---------- 运行时设置 ----------
	mux.HandleFunc("GET /api/webui/settings", s.webuiAuthed(s.handleWebUISettingsGET))
	mux.HandleFunc("POST /api/webui/settings", s.webuiAuthed(s.handleWebUISettingsPOST))

	// ---------- 系统信息 ----------
	mux.HandleFunc("GET /api/webui/system/info", s.webuiAuthed(s.handleSystemInfo))

	// ---------- 账号管理 ----------
	mux.HandleFunc("GET /api/webui/accounts", s.webuiAuthed(s.handleAccountsList))
	mux.HandleFunc("POST /api/webui/accounts", s.webuiAuthed(s.handleAccountCreate))
	mux.HandleFunc("DELETE /api/webui/accounts/{id}", s.webuiAuthed(s.handleAccountDelete))
	mux.HandleFunc("GET /api/webui/accounts/{id}/info", s.webuiAuthed(s.handleAccountInfo))
	mux.HandleFunc("POST /api/webui/accounts/{id}/rename", s.webuiAuthed(s.handleAccountRename))
	mux.HandleFunc("POST /api/webui/accounts/{id}/start", s.webuiAuthed(s.handleAccountStart))
	mux.HandleFunc("POST /api/webui/accounts/{id}/stop", s.webuiAuthed(s.handleAccountStop))
	mux.HandleFunc("POST /api/webui/accounts/{id}/settings", s.webuiAuthed(s.handleAccountUpdateSettings))
	mux.HandleFunc("GET /api/webui/accounts/{id}/qrcode", s.webuiAuthed(s.handleAccountQRCode))
	mux.HandleFunc("GET /api/webui/accounts/{id}/wait-login", s.webuiAuthed(s.handleAccountWaitLogin))

	// ---------- 账号调试控制 ----------
	mux.HandleFunc("GET /api/webui/accounts/{id}/screenshot", s.webuiAuthed(s.handleDebugScreenshot))
	mux.HandleFunc("GET /api/webui/accounts/{id}/console", s.webuiAuthed(s.handleDebugConsole))
	mux.HandleFunc("GET /api/webui/accounts/{id}/viewport", s.webuiAuthed(s.handleDebugViewport))
	mux.HandleFunc("GET /api/webui/accounts/{id}/html", s.webuiAuthed(s.handleDebugHTML))
	mux.HandleFunc("POST /api/webui/accounts/{id}/eval", s.webuiAuthed(s.handleDebugEval))
	mux.HandleFunc("POST /api/webui/accounts/{id}/click", s.webuiAuthed(s.handleDebugClick))
	mux.HandleFunc("POST /api/webui/accounts/{id}/drag", s.webuiAuthed(s.handleDebugDrag))
	mux.HandleFunc("POST /api/webui/accounts/{id}/key", s.webuiAuthed(s.handleDebugKey))
	mux.HandleFunc("POST /api/webui/accounts/{id}/type", s.webuiAuthed(s.handleDebugType))
	mux.HandleFunc("POST /api/webui/accounts/{id}/scroll", s.webuiAuthed(s.handleDebugScroll))
	mux.HandleFunc("POST /api/webui/accounts/{id}/rightclick", s.webuiAuthed(s.handleDebugRightClick))

	// ---------- 每个账号独立的连接适配器 ----------
	mux.HandleFunc("GET /api/webui/accounts/{id}/adapters", s.webuiAuthed(s.handleAccountAdaptersList))
	mux.HandleFunc("POST /api/webui/accounts/{id}/adapters", s.webuiAuthed(s.handleAccountAdapterCreate))
	mux.HandleFunc("PUT /api/webui/accounts/{id}/adapters/{aid}", s.webuiAuthed(s.handleAccountAdapterUpdate))
	mux.HandleFunc("DELETE /api/webui/accounts/{id}/adapters/{aid}", s.webuiAuthed(s.handleAccountAdapterDelete))
	mux.HandleFunc("GET /api/webui/accounts/{id}/adapters/status", s.webuiAuthed(s.handleAccountAdaptersStatus))

	// ---------- 实时事件流 ----------
	mux.HandleFunc("GET /api/webui/events", s.handleSSE)

	// ---------- 账号日志 ----------
	mux.HandleFunc("GET /api/webui/accounts/{id}/logs", s.webuiAuthed(s.handleAccountLogs))

	// ---------- OneBot v11 标准 action ----------
	// 单段路径 GET/POST 统一分发到注册表（send_private_msg、get_login_info 等）
	actionHandler := s.httpAction
	mux.HandleFunc("POST /{action}", actionHandler)
	mux.HandleFunc("GET /{action}", actionHandler)

	// ---------- 内部消息回调（JS fetch → Go，无需鉴权） ----------
	mux.HandleFunc("POST /api/internal/msg/{id}", s.handleInternalMsg)

	// ---------- 正向 WebSocket ----------
	mux.HandleFunc("GET "+s.WsPath, s.handleWS)

	log.Printf("[API] 服务已启动 http://0.0.0.0%s  (WebUI: /webui/  WS: %s)", s.Addr, s.WsPath)
	return http.ListenAndServe(s.Addr, mux)
}

// httpAction 将 HTTP 请求转为 action 分发。响应为标准 {status, retcode, data, echo}。
func (s *Server) httpAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	body, err := readBody(r)
	if err != nil {
		writeActionResult(w, failResult(RetCodeBadRequest, err.Error(), nil))
		return
	}
	var params json.RawMessage
	if len(body) > 0 {
		params = json.RawMessage(body)
	}
	var echo interface{}
	var probe map[string]interface{}
	if json.Unmarshal(params, &probe) == nil {
		echo = probe["echo"]
	}
	// OneBot access_token 鉴权（运行时设置热生效；空 = 不鉴权）
	if !s.checkOnebotAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		writeActionResult(w, failResult(RetCodeUnauth, "access_token 无效", echo))
		return
	}
	result := s.Dispatch(action, params, echo)
	if result.RetCode == RetCodeNotFound {
		w.WriteHeader(http.StatusNotFound)
	}
	writeActionResult(w, result)
}

func writeActionResult(w http.ResponseWriter, r *ActionResult) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(r)
}

// startEventBridge 订阅事件总线：
//   - message → 转 OneBot 消息事件广播（正向+反向）
//   - account → 账号状态通知广播（调试友好）+ SSE 推送
func (s *Server) startEventBridge() {
	sub := s.Bus.Subscribe(eventbus.TopicMessage, eventbus.TopicAccount)
	go func() {
		for ev := range sub.Ch() {
			switch payload := ev.Payload.(type) {
			case browser.MessageEvent:
				log.Printf("[EventBridge] 收到消息事件 account=%s len=%d", payload.AccountID, len(payload.Raw))
				incoming, err := browser.ParseIncoming(payload.Raw)
				if err != nil {
					log.Printf("[WARN] 解析浏览器消息失败: %v", err)
					continue
				}
				info, ok := s.BM.Info(payload.AccountID)
				if !ok {
					log.Printf("[WARN] 找不到账号信息 account=%s", payload.AccountID)
					continue
				}
				incoming.AccountID = payload.AccountID
				s.Broadcast(s.buildIncomingEvent(info, incoming))
				// SSE 推送消息
				s.BroadcastMessage(payload.AccountID, incoming)
			case browser.AccountStateEvent:
				s.Broadcast(EventAccount{
					Time:          time.Now().Unix(),
					PostType:      "meta_event",
					MetaEventType: "account_state",
					Account: map[string]interface{}{
						"id":     payload.AccountID,
						"name":   payload.Name,
						"prev":   payload.PrevState,
						"state":  payload.State,
						"error":  payload.Error,
					},
				})
				// SSE 推送账号状态
				s.BroadcastAccountStatus(payload.AccountID, payload.State)
			}
		}
	}()
}

// --- Per-Account Adapter Manager Lifecycle ---

// StartAccountAdapters 启动指定账号的适配器
func (s *Server) StartAccountAdapters(accountID, accountDir string) {
	s.mu.Lock()
	if existing, ok := s.adapterMgrs[accountID]; ok {
		existing.Stop()
	}
	mgr := NewAccountAdapterManager(accountID, s)
	s.adapterMgrs[accountID] = mgr
	s.mu.Unlock()
	mgr.Start(accountDir)
	log.Printf("[适配器] 账号 %s 适配器已启动", accountID)
}

// StopAccountAdapters 停止指定账号的适配器
func (s *Server) StopAccountAdapters(accountID string) {
	s.mu.Lock()
	mgr, ok := s.adapterMgrs[accountID]
	if ok {
		delete(s.adapterMgrs, accountID)
	}
	s.mu.Unlock()
	if ok {
		mgr.Stop()
		log.Printf("[适配器] 账号 %s 适配器已停止", accountID)
	}
}

// ReloadAccountAdapters 重载指定账号的适配器配置
func (s *Server) ReloadAccountAdapters(accountID, accountDir string) {
	s.mu.RLock()
	mgr, ok := s.adapterMgrs[accountID]
	s.mu.RUnlock()
	if ok {
		mgr.Reload(accountDir)
		return
	}
	// 账号可能在运行但 manager 未创建（例如 BM.Start 失败后用户手动创建适配器）
	// 此时创建 manager 并加载适配器
	mgr = NewAccountAdapterManager(accountID, s)
	s.mu.Lock()
	s.adapterMgrs[accountID] = mgr
	s.mu.Unlock()
	mgr.Start(accountDir)
}

// BroadcastToAccountAdapters 向指定账号的所有适配器推送事件
func (s *Server) BroadcastToAccountAdapters(accountID string, data []byte) {
	s.mu.RLock()
	mgr, ok := s.adapterMgrs[accountID]
	s.mu.RUnlock()
	if ok {
		mgr.SendEvent(data)
	}
}

// handleInternalMsg 接收 JS 端 fetch 回调的消息数据（无需鉴权，仅限本机）。
func (s *Server) handleInternalMsg(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("id")
	if accountID == "" {
		http.Error(w, "missing account id", 400)
		return
	}
	// CORS：允许来自任何来源的 fetch（about:blank 页面需要）
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	body, err := readBody(r)
	if err != nil || len(body) == 0 {
		http.Error(w, "empty body", 400)
		return
	}
	log.Printf("[InternalMsg] account=%s len=%d", accountID, len(body))
	s.Bus.Publish(eventbus.TopicMessage, browser.MessageEvent{AccountID: accountID, Raw: string(body)})
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
