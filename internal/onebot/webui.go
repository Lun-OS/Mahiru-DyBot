package onebot

// WebUI 管理 API：认证（设密/登录/令牌）、运行时设置、账号生命周期管理。
// 除 setup/login/me 外均需 Bearer 令牌（2小时有效）。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mahiru-dybot/internal/browser"
	"mahiru-dybot/internal/config"
)

// ---------- 中间件 ----------

func bearerToken(r *http.Request) string {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return strings.TrimPrefix(a, "Bearer ")
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return r.Header.Get("X-WebUI-Token")
}

// webuiAuthed 校验 WebUI 令牌。
func (s *Server) webuiAuthed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Auth.Validate(bearerToken(r)) {
			w.WriteHeader(http.StatusUnauthorized)
			writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "未登录或令牌已过期"})
			return
		}
		h(w, r)
	}
}

// writeJSONRaw 输出任意 JSON。
func writeJSONRaw(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// readBody 读取请求体。
func readBody(r *http.Request) ([]byte, error) {
	defer func() { _, _ = io.Copy(io.Discard, r.Body) }()
	return io.ReadAll(io.LimitReader(r.Body, 8<<20))
}

// ---------- 认证 ----------

// handleWebUIMe GET /api/webui/me → 初始化状态 + 当前令牌有效性。
func (s *Server) handleWebUIMe(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	writeJSONRaw(w, map[string]interface{}{
		"ok":            true,
		"initialized":   s.Auth.IsInitialized(),
		"authenticated": token != "" && s.Auth.Validate(token),
	})
}

// handleWebUISetup POST /api/webui/auth/setup {password} → 首次设置密码并自动登录。
func (s *Server) handleWebUISetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil || req.Password == "" {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"password\": \"...\"} (至少6位)"})
		return
	}
	if err := s.Auth.Setup(req.Password); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	token, lerr := s.Auth.Login(req.Password)
	if lerr != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": lerr.Error()})
		return
	}
	log.Printf("[WebUI] 密码已初始化")
	writeJSONRaw(w, map[string]interface{}{"ok": true, "token": token, "expires_in": int((2 * time.Hour).Seconds())})
}

// handleWebUILogin POST /api/webui/auth/login {password} → {token}。
func (s *Server) handleWebUILogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"password\": \"...\"}"})
		return
	}
	token, err := s.Auth.Login(req.Password)
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "token": token, "expires_in": int((2 * time.Hour).Seconds())})
}

// handleWebUIVerify POST /api/webui/auth/verify → 验证当前令牌是否有效。
func (s *Server) handleWebUIVerify(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	valid := token != "" && s.Auth.Validate(token)
	writeJSONRaw(w, map[string]interface{}{"ok": valid})
}

// handleWebUIReset POST /api/webui/auth/reset → 重置密码（需已登录）。
func (s *Server) handleWebUIReset(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.Reset(); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true})
}

// ---------- 运行时设置 ----------

// handleWebUISettingsGET GET /api/webui/settings。
func (s *Server) handleWebUISettingsGET(w http.ResponseWriter, r *http.Request) {
	st := s.RT.Get()
	resp := map[string]interface{}{
		"ok":                  true,
		"onebot_access_token": st.OneBotAccessToken,
		"screenshot_max_fps":  st.ScreenshotMaxFPS,
		"jpeg_quality":        st.JpegQuality,
		"reverse_ws":          st.ReverseWS,
		"ws_connections":      s.reverseStatuses(),
		"actions":             RegisteredActions(),
	}
	writeJSONRaw(w, resp)
}

// handleWebUISettingsPOST POST /api/webui/settings → 局部更新，热生效。
func (s *Server) handleWebUISettingsPOST(w http.ResponseWriter, r *http.Request) {
	body, _ := readBody(r)
	var req struct {
		OneBotAccessToken *string                  `json:"onebot_access_token"`
		ScreenshotMaxFPS  *int                     `json:"screenshot_max_fps"`
		JpegQuality       *int                     `json:"jpeg_quality"`
		ReverseWS         []config.ReverseWSConfig `json:"reverse_ws"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "JSON解析失败: " + err.Error()})
		return
	}
	err := s.RT.Update(func(st *config.RuntimeSettings) {
		if req.OneBotAccessToken != nil {
			st.OneBotAccessToken = strings.TrimSpace(*req.OneBotAccessToken)
		}
		if req.ScreenshotMaxFPS != nil && *req.ScreenshotMaxFPS > 0 {
			st.ScreenshotMaxFPS = *req.ScreenshotMaxFPS
		}
		if req.JpegQuality != nil {
			q := *req.JpegQuality
			if q < 1 { q = 1 }
			if q > 95 { q = 95 }
			st.JpegQuality = q
		}
		if req.ReverseWS != nil {
			for i := range req.ReverseWS {
				if req.ReverseWS[i].ID == "" {
					req.ReverseWS[i].ID = config.NewID("rw")
				}
				req.ReverseWS[i].URL = strings.TrimSpace(req.ReverseWS[i].URL)
			}
			st.ReverseWS = req.ReverseWS
		}
	})
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	s.handleWebUISettingsGET(w, r)
}

// reverseStatuses 反向连接实时状态（reverse 未初始化时返回空）。
func (s *Server) reverseStatuses() []map[string]interface{} {
	if s.reverse == nil {
		return []map[string]interface{}{}
	}
	return s.reverse.Statuses()
}

// ---------- 账号管理 ----------

// handleAccountsList GET /api/webui/accounts。
func (s *Server) handleAccountsList(w http.ResponseWriter, r *http.Request) {
	writeJSONRaw(w, map[string]interface{}{"ok": true, "accounts": s.BM.List()})
}

// handleAccountCreate POST /api/webui/accounts {name}。
func (s *Server) handleAccountCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	body, _ := readBody(r)
	_ = json.Unmarshal(body, &req)
	meta, err := s.BM.Create(req.Name)
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "account": meta})
}

// handleAccountDelete DELETE /api/webui/accounts/{id}。
func (s *Server) handleAccountDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, _ := s.BM.Info(id)
	if err := s.BM.Delete(id); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	log.Printf("[WebUI] 账号已删除: %v", info)
	writeJSONRaw(w, map[string]interface{}{"ok": true})
}

// handleAccountInfo GET /api/webui/accounts/{id}/info。
func (s *Server) handleAccountInfo(w http.ResponseWriter, r *http.Request) {
	info, ok := s.BM.Info(r.PathValue("id"))
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "account": info})
}

// handleAccountStart POST /api/webui/accounts/{id}/start。
// 同步执行（浏览器拉起约需10-30s），完成后返回最新状态。
func (s *Server) handleAccountStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.BM.Start(id)
	info, _ := s.BM.Info(id)
	// 无论浏览器启动是否成功，都尝试启动适配器
	if dir, ok := s.BM.InstanceDir(id); ok {
		s.StartAccountAdapters(id, dir)
	}
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error(), "account": info})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "account": info})
}

// handleAccountStop POST /api/webui/accounts/{id}/stop。
func (s *Server) handleAccountStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// 先停止适配器
	s.StopAccountAdapters(id)
	if err := s.BM.Stop(id); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	info, _ := s.BM.Info(id)
	writeJSONRaw(w, map[string]interface{}{"ok": true, "account": info})
}

// handleAccountRename POST /api/webui/accounts/{id}/rename {name}。
func (s *Server) handleAccountRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil || req.Name == "" {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"name\": \"...\"}"})
		return
	}
	if err := s.BM.Rename(r.PathValue("id"), req.Name); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true})
}

// handleAccountQRCode GET /api/webui/accounts/{id}/qrcode → 二维码 base64。
func (s *Server) handleAccountQRCode(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.BM.Get(r.PathValue("id"))
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}
	inst := acc.Instance()
	if inst == nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "浏览器未启动，请先调用 start"})
		return
	}
	if acc.State() == browser.StateOnline {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "该账号已在线，无需扫码"})
		return
	}
	b64, err := inst.GotoQRLogin(r.Context())
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "image_base64": b64, "token": inst.QRToken(), "state": acc.State()})
}

// handleAccountWaitLogin GET /api/webui/accounts/{id}/wait-login?timeout=180
// 长轮询：驱动扫码确认→SDK初始化→置online 全流程。
func (s *Server) handleAccountWaitLogin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acc, ok := s.BM.Get(id)
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}
	inst := acc.Instance()
	if inst == nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "浏览器未启动"})
		return
	}
	timeoutSec := 180
	if v := r.URL.Query().Get("timeout"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 600 {
			timeoutSec = n
		}
	}
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	// 登录流程串行化（管理员单用户场景足够）
	s.loginMu.Lock()
	defer s.loginMu.Unlock()

	for time.Now().Before(deadline) {
		if r.Context().Err() != nil {
			return // 客户端断开
		}
		st := acc.State()
		switch st {
		case browser.StateOnline:
			info, _ := s.BM.Info(id)
			writeJSONRaw(w, map[string]interface{}{"ok": true, "logged_in": true, "account": info})
			return
		case browser.StateError:
			writeJSONRaw(w, map[string]interface{}{"ok": false, "logged_in": false, "error": acc.LastError()})
			return
		case browser.StateQRPending:
			remain := time.Until(deadline)
			ctx, cancel := context.WithTimeout(r.Context(), remain)
			werr := inst.WaitLoginSuccess(ctx, 2*time.Second)
			cancel()
			switch {
			case werr == nil:
				if ferr := s.BM.FinalizeLogin(id); ferr != nil {
					writeJSONRaw(w, map[string]interface{}{"ok": false, "logged_in": false, "error": ferr.Error()})
					return
				}
				continue // 下一轮循环报告 online
			case errors.Is(werr, context.DeadlineExceeded), errors.Is(werr, context.Canceled):
				// 二维码超时未扫/客户端断开 → 继续外层循环直至总超时
				continue
			default:
				if strings.Contains(werr.Error(), "过期") {
					writeJSONRaw(w, map[string]interface{}{"ok": false, "logged_in": false, "expired": true, "error": "二维码已过期，请重新获取"})
					return
				}
				log.Printf("[WebUI] wait-login 轮询异常: %v", werr)
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(time.Until(minTime(deadline, time.Now().Add(2*time.Second)))):
		}
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "logged_in": false, "timeout": true})
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
