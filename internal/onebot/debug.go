package onebot

// 账号维度的调试/控制 API（WebUI 令牌鉴权）：
// 截图(限速)、JS执行、点击、键盘输入、页面状态、HTML。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"mahiru-dybot/internal/browser"
	"github.com/go-rod/rod"
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

// resolvePage 解析账号并返回就绪的页面，未就绪时写错误响应。
func (s *Server) resolvePage(w http.ResponseWriter, r *http.Request) (*browser.Account, *browser.Instance, *rod.Page, bool) {
	acc, inst, ok := s.resolveDebugTarget(w, r)
	if !ok {
		return nil, nil, nil, false
	}
	page := inst.Page()
	if page == nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "页面未就绪，浏览器仍在初始化中"})
		return nil, nil, nil, false
	}
	return acc, inst, page, true
}

// handleDebugScreenshot GET /api/webui/accounts/{id}/screenshot → PNG。
func (s *Server) handleDebugScreenshot(w http.ResponseWriter, r *http.Request) {
	fps := s.RT.Get().ScreenshotMaxFPS
	if !s.limiter.allow(fps) {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": fmt.Sprintf("截图过于频繁 (>%dfps)，可调大 screenshot_max_fps", fps)})
		return
	}
	_, _, page, ok := s.resolvePage(w, r)
	if !ok {
		return
	}
	png := page.MustScreenshot()
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(png)
}

// handleDebugConsole GET /api/webui/accounts/{id}/console → 页面状态。
func (s *Server) handleDebugConsole(w http.ResponseWriter, r *http.Request) {
	acc, _, page, ok := s.resolvePage(w, r)
	if !ok {
		return
	}
	var title, url, wpAvail interface{}
	var loggedIn, bodyLen interface{}
	rod.Try(func() {
		title = page.MustEval(`() => document.title`).Val()
		url = page.MustEval(`() => location.href`).Val()
		loggedIn = page.MustEval(`() => !!(window.userInfoStore && window.userInfoStore.curLoginUserInfo)`).Val()
		bodyLen = page.MustEval(`() => document.body ? document.body.innerHTML.length : 0`).Val()
		wpAvail = page.MustEval(`() => typeof window.webpackChunkdouyin_web`).Val()
	})

	writeJSONRaw(w, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"account":   acc.Meta,
			"title":     title,
			"url":       url,
			"logged_in": loggedIn,
			"body_len":  bodyLen,
			"webpack":   wpAvail,
		},
	})
}

// handleDebugEval POST {js} → 执行结果。
func (s *Server) handleDebugEval(w http.ResponseWriter, r *http.Request) {
	_, _, page, ok := s.resolvePage(w, r)
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
	js := req.JS
	// 自动包装：如果用户代码不是以箭头函数/函数开头，则包装为箭头函数
	trimmed := strings.TrimSpace(js)
	if !strings.HasPrefix(trimmed, "()") &&
		!strings.HasPrefix(trimmed, "async") &&
		!strings.HasPrefix(trimmed, "(async") &&
		!strings.HasPrefix(trimmed, "(function") &&
		!strings.HasPrefix(trimmed, "function") {
		if strings.HasPrefix(trimmed, "return ") {
			// `return x` → `() => { return x }`
			js = "() => { " + js + " }"
		} else {
			js = "() => " + js
		}
	}
	res, err := page.Eval(js)
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	b, _ := json.Marshal(res.Value.Val())
	writeJSONRaw(w, map[string]interface{}{"ok": true, "data": json.RawMessage(b)})
}

// handleDebugClick POST {x,y} → 模拟真实点击（含前置移动轨迹）。
func (s *Server) handleDebugClick(w http.ResponseWriter, r *http.Request) {
	_, inst, page, ok := s.resolvePage(w, r)
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
	var elInfo interface{}
	rod.Try(func() {
		elInfo = page.MustEval(fmt.Sprintf(`() => {
			var el = document.elementFromPoint(%f, %f);
			if (!el) return {error: 'no element'};
			return {
				tag: el.tagName,
				id: el.id,
				className: el.className,
				text: (el.textContent || '').substring(0, 100),
				rect: el.getBoundingClientRect ? JSON.parse(JSON.stringify(el.getBoundingClientRect())) : null
			};
		}`, req.X, req.Y)).Val()
	})

	if err := inst.Click(req.X, req.Y); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "x": req.X, "y": req.Y, "element": elInfo})
}

// handleDebugDrag POST {from_x,from_y,to_x,to_y,[steps]} → 人手轨迹拖拽（滑块验证）。
func (s *Server) handleDebugDrag(w http.ResponseWriter, r *http.Request) {
	_, inst, _, ok := s.resolvePage(w, r)
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
	_, inst, _, ok := s.resolvePage(w, r)
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
	_, inst, _, ok := s.resolvePage(w, r)
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
	_, inst, _, ok := s.resolvePage(w, r)
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
	_, inst, _, ok := s.resolvePage(w, r)
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
	_, inst, _, ok := s.resolvePage(w, r)
	if !ok {
		return
	}
	w2, h2 := inst.ViewportSize()
	writeJSONRaw(w, map[string]interface{}{"ok": true, "width": w2, "height": h2})
}

// handleDebugHTML GET → 页面HTML(前50KB)。
func (s *Server) handleDebugHTML(w http.ResponseWriter, r *http.Request) {
	_, _, page, ok := s.resolvePage(w, r)
	if !ok {
		return
	}
	var htmlStr string
	rod.Try(func() {
		res := page.MustEval(`() => document.documentElement.outerHTML`)
		htmlStr = res.Str()
		if htmlStr == "" {
			b, _ := json.Marshal(res.Val())
			htmlStr = string(b)
		}
	})
	if htmlStr == "" {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "获取页面 HTML 失败"})
		return
	}
	if len(htmlStr) > 50000 {
		htmlStr = htmlStr[:50000] + "\n... (truncated)"
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "html": htmlStr})
}
