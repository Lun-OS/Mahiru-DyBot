package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Instance 单个抖音账号的浏览器实例（Rod 管理的无头 Chromium + 独立存储）。
type Instance struct {
	ID         string // 所属账号 ID
	StorageDir string // 实例私有目录（state.json/mod.json 所在）
	AttachURL  string // 可选：连接已有 Chrome 的 CDP 地址（高级用法），为空则 Rod 原生 Launch

	mu        sync.Mutex
	browser   *rod.Browser
	page      *rod.Page
	launcher  *launcher.Launcher // Chrome 进程管理器（用于确保进程关闭）
	userAgent string
	qrToken   string

	selfUID  string
	nickname string
	onNewMsgs func(jsonRaw string)

	bindingReady         bool          // ExposeBinding 是否已注册（实例级）
	inputSession        *InputSession // 桌面客户端实时输入通道（懒创建）
	healthMonitorRunning bool          // 健康监控是否已启动（防止重复）

	viewportWidth  int // 浏览器视口宽度
	viewportHeight int // 浏览器视口高度

	// 会话列表缓存
	convCache     interface{} // 缓存的会话列表
	convCacheTime time.Time   // 缓存时间

	// SDK 重初始化互斥锁（防止并发 reinit 导致 page.MustEval 竞争）
	reinitMu sync.Mutex
	sdkReady bool // SDK 就绪缓存标志（避免快速路径 page.MustEval 阻塞）

	_pageReady atomic.Bool // 页面完全就绪（Launch 完成后才为 true，防 WebUI 在初始化期间调用 panic）

	callbackURL string // JS → Go 消息回调地址（http://127.0.0.1:port/api/internal/msg/{id}）
}

// NewInstance 创建账号浏览器实例（不启动浏览器，调用 Launch 启动）。
func NewInstance(id, storageDir, customUA string, vpW, vpH int) (*Instance, error) {
	ua := resolveUA(customUA)
	log.Printf("[%s] User-Agent: %s", id, ua)
	if vpW <= 0 {
		vpW = 1280
	}
	if vpH <= 0 {
		vpH = 720
	}
	return &Instance{
		ID:              id,
		StorageDir:      storageDir,
		userAgent:       ua,
		viewportWidth:   vpW,
		viewportHeight:  vpH,
	}, nil
}

// Launch 启动该实例的浏览器并进入聊天页（幂等：已启动则跳过）。
func (in *Instance) Launch() error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page != nil {
		return nil
	}

	if in.AttachURL != "" {
		return in.connectCDP(in.AttachURL)
	}

	// 确保存储目录存在
	if err := os.MkdirAll(in.StorageDir, 0o755); err != nil {
		return fmt.Errorf("创建存储目录失败: %w", err)
	}

	// Rod 原生 Launch：headless Chromium，进程由 launcher 管理
	// 每个账号使用独立的 user-data 目录，确保登录态隔离
	log.Printf("[%s] Rod 原生 Launch (headless=true) storageDir=%s", in.ID, in.StorageDir)
	page, browser, l, err := LaunchBrowser(in.StorageDir, in.viewportWidth, in.viewportHeight)
	if err != nil {
		return fmt.Errorf("Rod Launch 失败: %w", err)
	}
	in.browser = browser
	in.page = page
	in.launcher = l

	// 额外反检测注入（LaunchBrowser 已通过 stealth 注入基础反检测）
	page.MustEvalOnNewDocument(`() => {
		Object.defineProperty(navigator, 'languages', {get: () => ['zh-CN','zh','en']});
		Object.defineProperty(navigator, 'plugins', {get: () => [1,2,3,4,5]});
		window.chrome = {runtime: {}, loadTimes: function(){}, csi: function(){}, app: {}};
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({state: Notification.permission}) :
				originalQuery(parameters)
		);
	}`)

	// 通过 CDP 阻断无用资源（节省内存）
	_ = proto.NetworkSetBlockedURLs{
		Urls: []string{
			"*.woff", "*.woff2", "*.ttf", "*.otf", "*.eot",
			"*.mp4", "*.webm", "*.ogg", "*.mp3", "*.wav",
			"*.svg",
		},
	}.Call(page)

	// 导航到 /chat（建立 origin，用于 sessionStorage 注入）
	log.Printf("[%s] 导航到 /chat", in.ID)
	if err := in.gotoWithRetry(page, "https://www.douyin.com/chat"); err != nil {
		log.Printf("[%s] 首次导航失败: %v", in.ID, err)
	}

	// 恢复 sessionStorage（Rod 不包含 sessionStorage 持久化）
	if in.HasSavedState() {
		log.Printf("[%s] 从 state.json 恢复 cookies + localStorage", in.ID)
		in.restoreSessionStorage()
		// sessionStorage 注入后需要刷新才能生效
		if err := in.gotoWithRetry(page, "https://www.douyin.com/chat"); err != nil {
			log.Printf("[%s] 刷新 /chat 失败: %v", in.ID, err)
		}
	}

	// 等待页面 JS 初始化，最多等20秒
	loggedIn := false
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		res, evalErr := page.Eval(`() => !!(window.userInfoStore && window.userInfoStore.curLoginUserInfo)`)
		if evalErr != nil {
			log.Printf("[%s] 登录检测 eval 失败: %v", in.ID, evalErr)
			continue
		}
		if res.Value.Bool() {
			loggedIn = true
			break
		}
	}
	if !loggedIn {
		log.Printf("[%s] 未登录，再试 reload...", in.ID)
		rod.Try(func() {
			page.MustReload().MustWaitLoad()
		})
		time.Sleep(10 * time.Second)
	}

	rod.Try(func() {
		title := page.MustEval(`() => document.title`).Str()
		url := page.MustEval(`() => location.href`).Str()
		log.Printf("[%s] 页面状态 title=%v url=%v", in.ID, title, url)
		wpAvail := page.MustEval(`() => typeof window.webpackChunkdouyin_web`)
		log.Printf("[%s] webpack可用: %v", in.ID, wpAvail)
	})

	in._pageReady.Store(true)
	return nil
}

// connectCDP 通过 CDP 连接到已有 Chrome（AttachURL 模式）。
func (in *Instance) connectCDP(cdpURL string) error {
	page, browser, err := ConnectChrome(cdpURL)
	if err != nil {
		return fmt.Errorf("CDP 连接失败 %s: %w", cdpURL, err)
	}
	in.browser = browser
	in.page = page

	if err := in.gotoWithRetry(page, "https://www.douyin.com/chat"); err != nil {
		log.Printf("[%s] 打开 /chat 失败: %v", in.ID, err)
	}
	return nil
}

func (in *Instance) gotoWithRetry(page *rod.Page, url string) error {
	var lastErr error
	for i := 0; i < 3; i++ {
		err := rod.Try(func() {
			page.MustNavigate(url).MustWaitLoad()
		})
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return lastErr
}

// Page 返回当前页面。
func (in *Instance) Page() *rod.Page {
	if !in._pageReady.Load() {
		return nil
	}
	return in.page
}

// restoreSessionStorage 从 state.json 恢复 sessionStorage 到当前页面。
// 必须在页面已导航到对应域名后调用。
func (in *Instance) restoreSessionStorage() {
	path := filepath.Join(in.StorageDir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state struct {
		SessionStorage map[string][]sessionStorageItem `json:"session_storage"`
	}
	if err := json.Unmarshal(data, &state); err != nil || len(state.SessionStorage) == 0 {
		return
	}
	rod.Try(func() {
		pageOrigin := in.page.MustEval(`() => location.origin`).Str()
		items, ok := state.SessionStorage[pageOrigin]
		if !ok || len(items) == 0 {
			return
		}
		for _, item := range items {
			script := fmt.Sprintf(`() => sessionStorage.setItem(%s, %s)`,
				safeJSStr(item.Name), safeJSStr(item.Value))
			in.page.MustEval(script)
		}
		log.Printf("[%s] 已恢复 %d 个 sessionStorage 项 (origin=%s)", in.ID, len(items), pageOrigin)
	})
}

// safeJSStr 生成安全的 JS 字符串字面量（用 JSON 编码）。
func safeJSStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Click 在页面指定坐标处模拟真实点击：
// 先注入 mousemove（许多组件会忽略无移动轨迹的点击），再按下→抬起。
func (in *Instance) Click(x, y float64) error {
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	// 前置移动轨迹（两段），让目标组件先经历 hover 状态
	page.Mouse.MustMoveTo(x-30, y+8)
	time.Sleep(40 * time.Millisecond)
	page.Mouse.MustMoveTo(x, y)
	time.Sleep(60 * time.Millisecond)
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	time.Sleep(50 * time.Millisecond)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	return nil
}

// RightClick 在页面指定坐标处模拟右键点击。
func (in *Instance) RightClick(x, y float64) error {
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	page.Mouse.MustMoveTo(x-30, y+8)
	time.Sleep(40 * time.Millisecond)
	page.Mouse.MustMoveTo(x, y)
	time.Sleep(60 * time.Millisecond)
	page.Mouse.MustDown(proto.InputMouseButtonRight)
	time.Sleep(50 * time.Millisecond)
	page.Mouse.MustUp(proto.InputMouseButtonRight)
	return nil
}

// Drag 模拟人手拖拽轨迹（滑块验证等）：
// 移动到起点 → 按下 → 分段插值移动(带随机抖动) → 到达终点 → 抬起。
func (in *Instance) Drag(fromX, fromY, toX, toY float64, steps int) error {
	if steps < 5 {
		steps = 5
	}
	if steps > 120 {
		steps = 120
	}
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	page.Mouse.MustMoveTo(fromX, fromY)
	time.Sleep(80 * time.Millisecond)
	page.Mouse.MustDown(proto.InputMouseButtonLeft)

	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		// easeOutQuadratic：起步快后段慢，更接近真人拖滑块
		ease := 1 - (1-t)*(1-t)
		nx := fromX + (toX-fromX)*ease
		ny := fromY + (toY-fromY)*ease
		// 轻微抖动模拟手抖（终点前收敛）
		jx, jy := 0.0, 0.0
		if i < steps {
			jx = float64(jitterN(3) - 1)
			jy = float64(jitterN(3) - 1)
		}
		page.Mouse.MustMoveTo(nx+jx, ny+jy)
		time.Sleep(time.Duration(10+jitterN(18)) * time.Millisecond)
	}
	// 终点精确落点并短暂停顿后再抬起
	page.Mouse.MustMoveTo(toX, toY)
	time.Sleep(90 * time.Millisecond)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	return nil
}

// jitterRand 拖拽轨迹随机源。
var jitterRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// jitterN 返回 [0,n) 的随机整数。
func jitterN(n int) int {
	if n <= 0 {
		return 0
	}
	return jitterRand.Intn(n)
}

// parseKey 将 Playwright 风格的字符串按键名转换为 Rod input.Key。
func parseKey(key string) input.Key {
	switch key {
	case "Enter":
		return input.Enter
	case "Escape", "Esc":
		return input.Escape
	case "Backspace":
		return input.Backspace
	case "Tab":
		return input.Tab
	case "Space":
		return input.Space
	case "Delete":
		return input.Delete
	case "ArrowUp", "Up":
		return input.ArrowUp
	case "ArrowDown", "Down":
		return input.ArrowDown
	case "ArrowLeft", "Left":
		return input.ArrowLeft
	case "ArrowRight", "Right":
		return input.ArrowRight
	case "Home":
		return input.Home
	case "End":
		return input.End
	case "PageUp":
		return input.PageUp
	case "PageDown":
		return input.PageDown
	case "Control", "Ctrl":
		return input.ControlLeft
	case "Shift":
		return input.ShiftLeft
	case "Alt":
		return input.AltLeft
	case "Meta", "Command":
		return input.MetaLeft
	case "F1":
		return input.F1
	case "F2":
		return input.F2
	case "F3":
		return input.F3
	case "F4":
		return input.F4
	case "F5":
		return input.F5
	case "F6":
		return input.F6
	case "F7":
		return input.F7
	case "F8":
		return input.F8
	case "F9":
		return input.F9
	case "F10":
		return input.F10
	case "F11":
		return input.F11
	case "F12":
		return input.F12
	default:
		if len(key) == 1 {
			return input.AddKey(key, "", key, int(key[0]), 0)
		}
		return input.AddKey(key, "", key, 0, 0)
	}
}

// TypeAt 在页面指定坐标处模拟键盘输入。
func (in *Instance) TypeAt(x, y float64, text string) error {
	if err := in.Click(x, y); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page == nil {
		return fmt.Errorf("页面未就绪")
	}
	in.page.MustInsertText(text)
	return nil
}

// KeyPress 模拟按键（Enter/Escape/Tab/Backspace 等）。
func (in *Instance) KeyPress(key string) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page == nil {
		return fmt.Errorf("页面未就绪")
	}
	in.page.Keyboard.MustType(parseKey(key))
	return nil
}

// ViewportSize 返回页面实际客户区尺寸（innerWidth/innerHeight，
// 含滚动条修正，供截图坐标换算使用）。
func (in *Instance) ViewportSize() (float64, float64) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page == nil {
		return 1280, 720
	}
	var w, h int
	rod.Try(func() {
		w = in.page.MustEval(`() => window.innerWidth`).Int()
		h = in.page.MustEval(`() => window.innerHeight`).Int()
	})
	if w > 0 && h > 0 {
		return float64(w), float64(h)
	}
	return 1280, 720
}

// SetOnNewMessages 设置新消息回调。
func (in *Instance) SetOnNewMessages(cb func(jsonRaw string)) {
	in.mu.Lock()
	in.onNewMsgs = cb
	in.mu.Unlock()
}

// InitSDK 在后台初始化 IM SDK（不持锁，不阻塞其他操作）。
func (in *Instance) InitSDK(timeout time.Duration) error {
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := page.Eval(jsBootstrap)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		var raw string
		if str := res.Value.Str(); str != "" {
			raw = str
		} else {
			b, _ := json.Marshal(res.Value.Val())
			raw = string(b)
		}
		var out struct {
			Ok      bool   `json:"ok"`
			SelfUID string `json:"self_uid"`
			Error   string `json:"error"`
			ModID   int    `json:"mod_id"`
		}
		if json.Unmarshal([]byte(raw), &out) == nil && out.Ok {
			if out.ModID > 0 {
				in.SaveModID(out.ModID)
			}
			uid, nick := in.fetchUserInfo()
			if uid != "" {
				in.selfUID = uid
			}
			in.nickname = nick
			log.Printf("[SDK] 初始化成功 self_uid=%s nickname=%s", in.selfUID, in.nickname)
			in.registerBindingOnce()
			rod.Try(func() {
				in.page.MustEval(jsRegisterReceiver)
			})
			in.mu.Lock()
			in.sdkReady = true
			in.mu.Unlock()
			_ = in.SaveState()
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("IM SDK 初始化超时")
}

// EnsureReady 确保页面完成加载且 IM SDK 注入成功。
func (in *Instance) EnsureReady(timeout time.Duration) error {
	in.mu.Lock()
	if in.page == nil {
		in.mu.Unlock()
		return fmt.Errorf("页面未就绪")
	}
	page := in.page
	if savedModID := in.LoadModID(); savedModID > 0 {
		rod.Try(func() {
			page.MustEval(fmt.Sprintf("() => (window.__obModId = %d)", savedModID))
		})
	}
	in.mu.Unlock()

	// registerBindingOnce 不能在 in.mu 持有时调用（它自身需要获取 in.mu）
	in.registerBindingOnce()

	var lastErr error
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := page.Eval(jsBootstrap)
		if err != nil {
			lastErr = fmt.Errorf("eval 失败: %w", err)
			time.Sleep(2 * time.Second)
			continue
		}
		var raw string
		if str := res.Value.Str(); str != "" {
			raw = str
		} else {
			b, _ := json.Marshal(res.Value.Val())
			raw = string(b)
		}
		var out struct {
			Ok      bool   `json:"ok"`
			SelfUID string `json:"self_uid"`
			Error   string `json:"error"`
			ModID   int    `json:"mod_id"`
		}
		if json.Unmarshal([]byte(raw), &out) == nil && out.Ok {
			in.mu.Lock()
			in.selfUID = out.SelfUID
			if out.ModID > 0 {
				in.SaveModID(out.ModID)
			}
			uid, nick := in.fetchSelfInfoLocked()
			if uid != "" {
				in.selfUID = uid
			}
			in.nickname = nick
			in.mu.Unlock()
			// registerBindingOnce 不能在 in.mu 持有时调用
			in.registerBindingOnce()
			rod.Try(func() {
				page.MustEval(jsRegisterReceiver)
			})
			in.mu.Lock()
			in.sdkReady = true
			in.mu.Unlock()
			log.Printf("[%s] IM SDK 初始化成功 self_uid=%s nickname=%s mod_id=%v", in.ID, in.selfUID, in.nickname, out.ModID)
			if err := in.SaveState(); err != nil {
				log.Printf("[%s] 保存登录态失败: %v", in.ID, err)
			} else {
				log.Printf("[%s] 登录态保存成功", in.ID)
			}
			return nil
		}
		lastErr = fmt.Errorf("%s", out.Error)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("IM SDK 初始化超时: %v", lastErr)
}

// EnsureSDK 快速检查 SDK 是否就绪，未就绪则自动重初始化。
func (in *Instance) EnsureSDK() error {
	in.mu.Lock()
	if in.page == nil {
		in.mu.Unlock()
		return fmt.Errorf("页面未就绪")
	}
	if in.sdkReady {
		in.mu.Unlock()
		return nil
	}
	page := in.page
	in.mu.Unlock()
	log.Printf("[%s] SDK 不可用，尝试自动重初始化...", in.ID)
	if err := in.ensureSDKLocked(page); err != nil {
		return err
	}
	return nil
}

// ensureSDKLocked 执行 SDK 重初始化（由 reinitMu 保证不并发执行）。
func (in *Instance) ensureSDKLocked(page *rod.Page) error {
	in.reinitMu.Lock()
	defer in.reinitMu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	var lastErr error
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		res, err := page.Eval(jsBootstrap)
		if err != nil {
			lastErr = fmt.Errorf("eval 失败: %w", err)
			time.Sleep(2 * time.Second)
			continue
		}
		var raw string
		if str := res.Value.Str(); str != "" {
			raw = str
		} else {
			b, _ := json.Marshal(res.Value.Val())
			raw = string(b)
		}
		var out struct {
			Ok      bool   `json:"ok"`
			SelfUID string `json:"self_uid"`
			Error   string `json:"error"`
			ModID   int    `json:"mod_id"`
		}
		if json.Unmarshal([]byte(raw), &out) == nil && out.Ok {
			if out.SelfUID != "" {
				in.selfUID = out.SelfUID
			}
			if out.ModID > 0 {
				in.SaveModID(out.ModID)
			}
			uid, nick := in.fetchSelfInfoLocked()
			if uid != "" {
				in.selfUID = uid
			}
			if nick != "" {
				in.nickname = nick
			}
			in.registerBindingOnce()
			rod.Try(func() {
				in.page.MustEval(jsRegisterReceiver)
			})
			log.Printf("[%s] SDK 自动重初始化成功 self_uid=%s", in.ID, in.selfUID)
			in.mu.Lock()
			in.sdkReady = true
			in.mu.Unlock()
			return nil
		}
		lastErr = fmt.Errorf("%s", out.Error)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("SDK 自动重初始化失败: %v", lastErr)
}

// GetCachedConversations 获取缓存的会话列表，过期时间 30 秒。
func (in *Instance) GetCachedConversations() (interface{}, error) {
	in.mu.Lock()
	if in.convCache != nil && time.Since(in.convCacheTime) < 30*time.Second {
		cache := in.convCache
		in.mu.Unlock()
		return cache, nil
	}
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return nil, fmt.Errorf("页面未就绪")
	}
	res := page.MustEval(`async () => {
		var ctx = window.__imCtx;
		if (!ctx || !ctx.imSdkService) return JSON.stringify({ ok: false, error: 'imSdkService 不可用' });
		var clm = ctx.imSdkService.conversationListManager;
		if (!clm || !clm.getAllConversation) return JSON.stringify({ ok: false, error: 'getAllConversation 方法不可用' });
		var result = await clm.getAllConversation();
		return JSON.stringify({ ok: true, result: result });
	}`)
	var raw string
	if str := res.Str(); str != "" {
		raw = str
	} else {
		b, _ := json.Marshal(res.Val())
		raw = string(b)
	}
	var out struct {
		Ok     bool        `json:"ok"`
		Error  string      `json:"error"`
		Result interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	in.mu.Lock()
	in.convCache = out.Result
	in.convCacheTime = time.Now()
	in.mu.Unlock()
	return out.Result, nil
}

// InvalidateConvCache 清除会话列表缓存（发送/删除消息后调用）。
func (in *Instance) InvalidateConvCache() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.convCache = nil
}

// StartHealthMonitor 当页面崩溃/导航导致 SDK 丢失时自动恢复。
func (in *Instance) StartHealthMonitor(interval time.Duration, onStop func()) {
	in.mu.Lock()
	if in.healthMonitorRunning {
		in.mu.Unlock()
		return
	}
	in.healthMonitorRunning = true
	in.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			<-ticker.C
			in.mu.Lock()
			if in.page == nil || in.browser == nil {
				in.healthMonitorRunning = false
				in.mu.Unlock()
				log.Printf("[%s] 健康监控: 页面/浏览器已关闭，停止监控", in.ID)
				if onStop != nil {
					onStop()
				}
				return
			}
			page := in.page
			in.mu.Unlock()

			// 检查页面是否存活
			alive := rod.Try(func() {
				page.MustEval(`() => 1`)
			})
			if alive != nil {
				log.Printf("[%s] 健康监控: 页面不可用 (%v)，等待恢复...", in.ID, alive)
				time.Sleep(3 * time.Second)
				alive2 := rod.Try(func() {
					page.MustEval(`() => 1`)
				})
				if alive2 != nil {
					log.Printf("[%s] 健康监控: 页面仍然不可用，停止监控", in.ID)
					in.mu.Lock()
					in.healthMonitorRunning = false
					in.mu.Unlock()
					if onStop != nil {
						onStop()
					}
					return
				}
			}
			// 检查 SDK 是否可用
			sdkRes, sdkErr := page.Eval(`() => !!(window.__sdkInst && window.__imCtx)`)
			if sdkErr != nil {
				log.Printf("[%s] 健康监控: SDK 检查失败 (%v)，停止监控", in.ID, sdkErr)
				in.mu.Lock()
				in.healthMonitorRunning = false
				in.mu.Unlock()
				if onStop != nil {
					onStop()
				}
				return
			}
			sdkOK := sdkRes.Value.Bool()
			if !sdkOK {
				log.Printf("[%s] 健康监控: SDK 不可用，自动重初始化...", in.ID)
				in.mu.Lock()
				in.sdkReady = false
				in.mu.Unlock()
				if err := in.EnsureSDK(); err != nil {
					log.Printf("[%s] 健康监控: SDK 重初始化失败: %v", in.ID, err)
				} else {
					log.Printf("[%s] 健康监控: SDK 重初始化成功", in.ID)
				}
			}
		}
	}()
}

// fetchSelfInfoLocked 获取自身昵称等信息（需持锁，页面已就绪）。
func (in *Instance) fetchSelfInfoLocked() (uid, nickname string) {
	res, err := in.page.Eval(jsGetSelfInfo)
	if err != nil {
		return "", ""
	}
	var raw string
	if str := res.Value.Str(); str != "" {
		raw = str
	} else {
		b, _ := json.Marshal(res.Value.Val())
		raw = string(b)
	}
	var out struct {
		UID      string `json:"uid"`
		Nickname string `json:"nickname"`
	}
	if json.Unmarshal([]byte(raw), &out) == nil {
		return out.UID, out.Nickname
	}
	return "", ""
}

// SelfUID 当前登录账号的数字 uid。
func (in *Instance) SelfUID() string {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.selfUID
}

// SelfNickname 当前登录账号的昵称。
func (in *Instance) SelfNickname() string {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.nickname
}

// GetUserNickname 通过 SDK userCacheManager 获取指定用户的昵称。
func (in *Instance) GetUserNickname(uid string) string {
	if uid == "" {
		return ""
	}
	in.mu.Lock()
	page := in.page
	ready := in.sdkReady
	in.mu.Unlock()
	if !ready || page == nil {
		return ""
	}
	res := page.MustEval(fmt.Sprintf(`() => {try{var sdk=window.__imCtx&&window.__imCtx.imSdkService;if(!sdk||!sdk.userCacheManager||!sdk.userCacheManager.getUserInfo)return"";var info=sdk.userCacheManager.getUserInfo(%q);return info?(info.nickname||""):""}catch(e){return""}}`, uid))
	return res.Str()
}

// SDKStatus SDK 运行时状态信息。
type SDKStatus struct {
	SDKReady          bool   `json:"sdk_ready"`
	SelfUID           string `json:"self_uid"`
	ModID             int    `json:"mod_id"`
	ConversationCount int    `json:"conversation_count"`
	ReceiverRegistered bool  `json:"receiver_registered"`
	ConnectionStatus  string `json:"connection_status"`
}

// GetSDKStatus 获取完整的 SDK 运行时状态。
func (in *Instance) GetSDKStatus() SDKStatus {
	if in.page == nil {
		return SDKStatus{}
	}
	res := in.page.MustEval(jsGetSDKStatus)
	var out SDKStatus
	var raw string
	if str := res.Str(); str != "" {
		raw = str
	} else {
		b, _ := json.Marshal(res.Val())
		raw = string(b)
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// LoginCheckResult 检测登录状态并返回完整用户信息。
type LoginCheckResult struct {
	LoggedIn bool `json:"logged_in"`
	User     *struct {
		UID       string `json:"uid"`
		SecUID    string `json:"sec_uid"`
		Nickname  string `json:"nickname"`
		Avatar    string `json:"avatar"`
		UniqueID  string `json:"unique_id"`
		ShortID   string `json:"short_id"`
		Signature string `json:"signature"`
		Gender    int    `json:"gender"`
	} `json:"user,omitempty"`
	SDKReady bool `json:"sdk_ready"`
	ModID    int  `json:"mod_id"`
}

// CheckLoginWithUser 检测登录状态并返回完整用户信息。
func (in *Instance) CheckLoginWithUser() LoginCheckResult {
	if in.page == nil {
		return LoginCheckResult{}
	}
	res := in.page.MustEval(jsCheckLogin)
	var out LoginCheckResult
	var raw string
	if str := res.Str(); str != "" {
		raw = str
	} else {
		b, _ := json.Marshal(res.Val())
		raw = string(b)
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// fetchUserInfo 直接从页面获取 uid 和昵称（不持锁，适合后台调用）。
func (in *Instance) fetchUserInfo() (uid, nickname string) {
	if in.page == nil {
		return "", ""
	}
	res, err := in.page.Eval(jsGetSelfInfo)
	if err != nil {
		return "", ""
	}
	var raw string
	if str := res.Value.Str(); str != "" {
		raw = str
	} else {
		b, _ := json.Marshal(res.Value.Val())
		raw = string(b)
	}
	var out struct {
		UID      string `json:"uid"`
		Nickname string `json:"nickname"`
	}
	if json.Unmarshal([]byte(raw), &out) == nil {
		return out.UID, out.Nickname
	}
	return "", ""
}

// IsLoggedIn 检测登录状态（不阻塞 EnsureReady）。
func (in *Instance) IsLoggedIn(ctx context.Context) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return false, nil
	}
	// 用 Eval（非 MustEval）避免页面异常时 panic
	res, err := page.Eval(jsCheckLogin)
	if err != nil {
		return false, fmt.Errorf("eval 登录检测失败: %w", err)
	}
	var raw string
	if str := res.Value.Str(); str != "" {
		raw = str
	} else {
		b, _ := json.Marshal(res.Value.Val())
		raw = string(b)
	}
	var out struct {
		LoggedIn bool `json:"logged_in"`
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return false, fmt.Errorf("解析登录状态失败: %s", raw)
	}
	return out.LoggedIn, nil
}

// GotoQRLogin 通过 passport API 直接获取登录二维码。
func (in *Instance) GotoQRLogin(ctx context.Context) (string, error) {
	in.mu.Lock()
	defer in.mu.Unlock()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if in.page == nil {
		return "", fmt.Errorf("浏览器未启动")
	}
	if !strings.Contains(in.page.MustInfo().URL, "douyin.com") {
		in.page.MustNavigate("https://www.douyin.com/chat").MustWaitLoad()
		time.Sleep(5 * time.Second)
	}

	var lastErr string
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
			in.page.MustReload().MustWaitLoad()
			time.Sleep(3 * time.Second)
		}
		res := in.page.MustEval(jsGetQRCode)

		var out struct {
			OK          bool   `json:"ok"`
			Error       string `json:"error"`
			ImageBase64 string `json:"image_base64"`
			Token       string `json:"token"`
			Method      string `json:"method"`
		}
		var raw string
		if str := res.Str(); str != "" {
			raw = str
		} else {
			b, _ := json.Marshal(res.Val())
			raw = string(b)
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			lastErr = fmt.Sprintf("解析失败: %s", raw)
			continue
		}
		if !out.OK {
			lastErr = out.Error
			continue
		}

		log.Printf("[%s] QR 截取成功 method=%s token=%q", in.ID, out.Method, out.Token)
		in.qrToken = out.Token
		return out.ImageBase64, nil
	}
	return "", fmt.Errorf("获取二维码失败(重试5次): %s", lastErr)
}

// QRToken 当前待扫码的token。
func (in *Instance) QRToken() string {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.qrToken
}

// CheckQRCode 轮询QR码扫码状态。返回 status: 0未扫/1已扫/2已确认/3过期。
func (in *Instance) CheckQRCode(token string) (int, string, error) {
	in.mu.Lock()
	defer in.mu.Unlock()

	argJSON, _ := json.Marshal([]map[string]string{{"token": token}})
	res := in.page.MustEval(fmt.Sprintf(`async () => { var __fn = %s; return await __fn(%s); }`, jsCheckQRCode, string(argJSON)))

	var out struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error"`
		Status      int    `json:"status"`
		RedirectURL string `json:"redirect_url"`
	}
	var raw string
	if str := res.Str(); str != "" {
		raw = str
	} else {
		b, _ := json.Marshal(res.Val())
		raw = string(b)
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return -1, "", fmt.Errorf("解析扫码状态失败: %s", raw)
	}
	if !out.OK {
		return -1, "", fmt.Errorf("查询扫码状态失败: %s", out.Error)
	}
	return out.Status, out.RedirectURL, nil
}

// ImportCookies 导入 cookie 到当前浏览器页面。
func (in *Instance) ImportCookies(cookies []CookieImport) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page == nil {
		return fmt.Errorf("页面未就绪")
	}
	params := make([]*proto.NetworkCookieParam, 0, len(cookies))
	for _, c := range cookies {
		params = append(params, &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  proto.TimeSinceEpoch(c.Expires),
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
			SameSite: proto.NetworkCookieSameSite(c.SameSite),
		})
	}
	return proto.NetworkSetCookies{Cookies: params}.Call(in.page)
}

// CookieImport 用于 ImportCookies 的 cookie 结构。
type CookieImport struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HttpOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite,omitempty"`
}

// WaitLoginSuccess 轮询等待扫码成功；成功后保存登录态。
func (in *Instance) WaitLoginSuccess(ctx context.Context, pollEvery time.Duration) error {
	// 统一通过页面内部状态判断登录：轮询 userInfoStore.curLoginUserInfo
	// 不再依赖外部 QR API（CheckQRCode），因为登录态变化频繁，应直接从页面变量获取
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollEvery):
		}
		ok, err := in.IsLoggedIn(ctx)
		if err == nil && ok {
			return nil
		}
		if err != nil {
			log.Printf("[%s] 登录状态轮询: %v", in.ID, err)
		}
	}
}

// ReloadAndInit 重新加载页面并初始化 IM SDK。
func (in *Instance) ReloadAndInit(timeout time.Duration) error {
	in.mu.Lock()
	in.page.MustNavigate("https://www.douyin.com/chat").MustWaitLoad()
	in.mu.Unlock()
	return in.EnsureReady(timeout)
}

// Disconnect 保存状态并关闭浏览器（确保 Chrome 进程被终止）。
func (in *Instance) Disconnect() {
	if err := in.SaveState(); err != nil {
		log.Printf("[%s] 断开前保存状态失败: %v", in.ID, err)
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.inputSession != nil {
		in.inputSession.close()
		in.inputSession = nil
	}
	// 先通过 rod 关闭浏览器
	if in.browser != nil {
		func() {
			defer func() { recover() }() // 防止 MustClose panic
			in.browser.MustClose()
		}()
		in.browser = nil
	}
	// 再通过 launcher 确保 Chrome 进程被 kill（防止进程泄漏）
	if in.launcher != nil {
		in.launcher.Kill()
		in.launcher = nil
	}
	in.page = nil
	in._pageReady.Store(false)
	in.bindingReady = false
}

// Close 优雅关闭实例：保存状态 → 关闭浏览器（确保 Chrome 进程被终止）。
func (in *Instance) Close() {
	in.mu.Lock()
	s := in.inputSession
	in.inputSession = nil
	in.mu.Unlock()
	if s != nil {
		s.close()
	}
	if err := in.SaveState(); err != nil {
		log.Printf("[%s] 关闭前保存状态失败: %v", in.ID, err)
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	// 先通过 rod 关闭浏览器
	if in.browser != nil {
		func() {
			defer func() { recover() }()
			in.browser.MustClose()
		}()
		in.browser = nil
	}
	// 再通过 launcher 确保 Chrome 进程被 kill
	if in.launcher != nil {
		in.launcher.Kill()
		in.launcher = nil
	}
	in.page = nil
	in._pageReady.Store(false)
	in.bindingReady = false
}

// Running 浏览器是否在运行。
func (in *Instance) Running() bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.browser != nil || in.AttachURL != ""
}

var _ = context.Background

// SetCallbackURL 设置 JS→Go 消息回调地址。
func (in *Instance) SetCallbackURL(url string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.callbackURL = url
}

// registerBindingOnce 注册新消息桥接：注入回调 URL 到页面（每实例一次）。
func (in *Instance) registerBindingOnce() {
	in.mu.Lock()
	if in.bindingReady || in.page == nil || in.callbackURL == "" {
		in.mu.Unlock()
		return
	}
	page := in.page
	cbURL := in.callbackURL
	accountID := in.ID
	in.mu.Unlock()
	rod.Try(func() {
		page.MustEval(fmt.Sprintf(`() => { window.__obAccountId = %q; window.__obCallbackURL = %q; }`, accountID, cbURL))
	})
	in.mu.Lock()
	in.bindingReady = true
	in.mu.Unlock()
	log.Printf("[%s] 回调变量注入成功 url=%s", accountID, cbURL)
}

// StartMessagePolling 启动 Go 侧轮询：每 2s 通过 page.MustEval 拉取 JS 侧 __obNewMsgs 队列，
// 绕过 HTTPS→HTTP 混合内容限制，将新消息发布到 EventBus。
func (in *Instance) StartMessagePolling(onNewMsg func(rawJSON string)) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			in.mu.Lock()
			page := in.page
			ready := in.sdkReady
			in.mu.Unlock()
			if !ready || page == nil {
				continue
			}
			res, err := page.Eval(jsDrainNewMsgs)
			if err != nil {
				log.Printf("[%s] 消息轮询 eval 失败: %v", in.ID, err)
				continue
			}
			var out struct {
				Ok    bool                     `json:"ok"`
				Msgs  []map[string]interface{} `json:"msgs"`
				Error string                   `json:"error"`
			}
			var raw string
			if str := res.Value.Str(); str != "" {
				raw = str
			} else {
				b, _ := json.Marshal(res.Value.Val())
				raw = string(b)
			}
			if json.Unmarshal([]byte(raw), &out) != nil || !out.Ok || len(out.Msgs) == 0 {
				continue
			}
			for _, m := range out.Msgs {
				b, err := json.Marshal(m)
				if err != nil {
					continue
				}
				if onNewMsg != nil {
					onNewMsg(string(b))
				}
			}
		}
	}()
}
