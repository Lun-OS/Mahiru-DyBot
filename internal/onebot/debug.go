package onebot

// 账号维度的调试/控制 API（WebUI 令牌鉴权）：
// 截图(限速)、JS执行、点击、键盘输入、页面状态、HTML。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"mahiru-dybot/internal/browser"
)

// screenshotLimiter 截图限速器（fps 运行时读取，可热更）。
type screenshotLimiter struct {
	mu          sync.Mutex
	lastCapture time.Time
}

func (l *screenshotLimiter) allow(fps int) bool {
	if fps <= 0 {
		fps = 10
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.lastCapture) < time.Second/time.Duration(fps) {
		return false
	}
	l.lastCapture = now
	return true
}

// resolveDebugTarget 按路径参数解析账号与在线实例。
// 返回 nil, nil, false 表示不存在或未启动（已写响应）。
func (s *Server) resolveDebugTarget(w http.ResponseWriter, r *http.Request) (*browser.Account, *browser.Instance, bool) {
	acc, ok := s.BM.Get(r.PathValue("id"))
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return nil, nil, false
	}
	inst := acc.Instance()
	if inst == nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "浏览器未启动，请先调用 start"})
		return nil, nil, false
	}
	return acc, inst, true
}

// handleDebugScreenshot GET /api/webui/accounts/{id}/screenshot → PNG。
func (s *Server) handleDebugScreenshot(w http.ResponseWriter, r *http.Request) {
	fps := s.RT.Get().ScreenshotMaxFPS
	if !s.limiter.allow(fps) {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": fmt.Sprintf("截图过于频繁 (>%dfps)，可调大 screenshot_max_fps", fps)})
		return
	}
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	png, err := inst.Page().Screenshot()
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(png)
}

// handleDebugConsole GET /api/webui/accounts/{id}/console → 页面状态。
func (s *Server) handleDebugConsole(w http.ResponseWriter, r *http.Request) {
	acc, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	page := inst.Page()
	title, _ := page.Evaluate("document.title")
	url, _ := page.Evaluate("location.href")
	loggedIn, _ := page.Evaluate(`!!(window.userInfoStore && window.userInfoStore.curLoginUserInfo)`)
	bodyLen, _ := page.Evaluate(`document.body ? document.body.innerHTML.length : 0`)
	wpAvail, _ := page.Evaluate(`typeof window.webpackChunkdouyin_web`)

	writeJSONRaw(w, map[string]interface{}{
		"ok":        true,
		"account":   acc.Meta,
		"title":     title,
		"url":       url,
		"logged_in": loggedIn,
		"body_len":  bodyLen,
		"webpack":   wpAvail,
	})
}

// handleDebugEval POST {js} → 执行结果。
func (s *Server) handleDebugEval(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		JS string `json:"js"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil || req.JS == "" {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"js\": \"...\"}"})
		return
	}
	res, err := inst.Page().Evaluate(req.JS)
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	b, _ := json.Marshal(res)
	writeJSONRaw(w, map[string]interface{}{"ok": true, "result": json.RawMessage(b)})
}

// handleDebugClick POST {x,y} → 模拟真实点击（含前置移动轨迹）。
func (s *Server) handleDebugClick(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"x\": number, \"y\": number}"})
		return
	}
	elInfo, _ := inst.Page().Evaluate(fmt.Sprintf(`(function() {
		var el = document.elementFromPoint(%f, %f);
		if (!el) return {error: 'no element'};
		return {
			tag: el.tagName,
			id: el.id,
			className: el.className,
			text: (el.textContent || '').substring(0, 100),
			rect: el.getBoundingClientRect ? JSON.parse(JSON.stringify(el.getBoundingClientRect())) : null
		};
	})()`, req.X, req.Y))

	if err := inst.Click(req.X, req.Y); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "x": req.X, "y": req.Y, "element": elInfo})
}

// handleDebugDrag POST {from_x,from_y,to_x,to_y,[steps]} → 人手轨迹拖拽（滑块验证）。
func (s *Server) handleDebugDrag(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		FromX float64 `json:"from_x"`
		FromY float64 `json:"from_y"`
		ToX   float64 `json:"to_x"`
		ToY   float64 `json:"to_y"`
		Steps int     `json:"steps"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"from_x\":..,\"from_y\":..,\"to_x\":..,\"to_y\":..,\"steps\"?:..}"})
		return
	}
	if req.Steps <= 0 {
		dist := absF(req.ToX-req.FromX) + absF(req.ToY-req.FromY)
		req.Steps = int(dist/15) + 8
	}
	if err := inst.Drag(req.FromX, req.FromY, req.ToX, req.ToY, req.Steps); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true,
		"from": map[string]float64{"x": req.FromX, "y": req.FromY},
		"to":   map[string]float64{"x": req.ToX, "y": req.ToY},
		"steps": req.Steps,
	})
}

// handleDebugKey POST {key} → 模拟按键（Enter/Escape/Tab/Backspace 等）。
func (s *Server) handleDebugKey(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil || req.Key == "" {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"key\": \"Enter\"}"})
		return
	}
	if err := inst.KeyPress(req.Key); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "key": req.Key})
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// handleDebugType POST {x,y,text} → 点击后键入。
func (s *Server) handleDebugType(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Text string  `json:"text"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"x\": number, \"y\": number, \"text\": \"...\"}"})
		return
	}
	if err := inst.TypeAt(req.X, req.Y, req.Text); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true})
}

// handleDebugScroll POST {x,y,delta_x,delta_y} → 鼠标滚轮。
func (s *Server) handleDebugScroll(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		DeltaX int     `json:"delta_x"`
		DeltaY int     `json:"delta_y"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"x\": number, \"y\": number, \"delta_x\": number, \"delta_y\": number}"})
		return
	}
	if err := inst.SessionInput().MouseScroll(req.X, req.Y, req.DeltaX, req.DeltaY); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true})
}

// handleDebugRightClick POST {x,y} → 右键点击。
func (s *Server) handleDebugRightClick(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	var req struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	body, _ := readBody(r)
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "需要 {\"x\": number, \"y\": number}"})
		return
	}
	if err := inst.RightClick(req.X, req.Y); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true})
}

// handleDebugViewport GET → 视口大小。
func (s *Server) handleDebugViewport(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	w2, h2 := inst.ViewportSize()
	writeJSONRaw(w, map[string]interface{}{"ok": true, "width": w2, "height": h2})
}

// handleDebugHTML GET → 页面HTML(前50KB)。
func (s *Server) handleDebugHTML(w http.ResponseWriter, r *http.Request) {
	_, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return
	}
	html, err := inst.Page().Evaluate(`document.documentElement.outerHTML`)
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	htmlStr := fmt.Sprintf("%v", html)
	if len(htmlStr) > 50000 {
		htmlStr = htmlStr[:50000] + "\n... (truncated)"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlStr))
}
