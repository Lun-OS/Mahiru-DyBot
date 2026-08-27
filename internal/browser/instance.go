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
	"time"

	"github.com/playwright-community/playwright-go"
)

// Instance 单个抖音账号的浏览器实例（Playwright 管理的无头 Chromium + 独立存储）。
type Instance struct {
	ID         string // 所属账号 ID
	StorageDir string // 实例私有目录（state.json/mod.json 所在）
	AttachURL  string // 可选：连接已有 Chrome 的 CDP 地址（高级用法），为空则 Playwright 原生 Launch

	mu       sync.Mutex
	browser  playwright.Browser
	context  playwright.BrowserContext
	page     playwright.Page
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

	// SDK 重初始化互斥锁（防止并发 reinit 导致 page.Evaluate 竞争）
	reinitMu sync.Mutex
	sdkReady bool // SDK 就绪缓存标志（避免快速路径 page.Evaluate 阻塞）

	callbackURL string // JS → Go 消息回调地址（http://127.0.0.1:port/api/internal/msg/{id}）
}

// NewInstance 创建账号浏览器实例（不启动浏览器，调用 Launch 启动）。
func NewInstance(id, storageDir, customUA string, vpW, vpH int) (*Instance, error) {
	if _, err := SharedPlaywright(); err != nil {
		return nil, err
	}
	ua := resolveUA(customUA)
	log.Printf("[%s] User-Agent: %s", id, ua)
	if vpW <= 0 {
		vpW = 1920
	}
	if vpH <= 0 {
		vpH = 1080
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
	pw, _ := SharedPlaywright()

	if in.AttachURL != "" {
		return in.connectCDP(pw, in.AttachURL)
	}

	// Playwright 原生 Launch：headless Chromium，进程由 Playwright 管理
	log.Printf("[%s] Playwright 原生 Launch (headless=true)", in.ID)
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-gpu",
			"--mute-audio",
			"--lang=zh-CN",
		},
	})
	if err != nil {
		return fmt.Errorf("Playwright Launch 失败: %w", err)
	}
	in.browser = browser

	// 创建 BrowserContext：如果有保存的 state.json 则自动恢复 cookies + localStorage
	ctxOpts := playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(in.userAgent),
		Viewport:  &playwright.Size{Width: in.viewportWidth, Height: in.viewportHeight},
	}
	statePath := filepath.Join(in.StorageDir, "state.json")
	if in.HasSavedState() {
		ctxOpts.StorageStatePath = playwright.String(statePath)
		log.Printf("[%s] 从 state.json 恢复 cookies + localStorage", in.ID)
	}
	ctx, err := browser.NewContext(ctxOpts)
	if err != nil {
		_ = browser.Close()
		return fmt.Errorf("创建上下文失败: %w", err)
	}
	in.context = ctx

	page, err := ctx.NewPage()
	if err != nil {
		_ = ctx.Close()
		_ = browser.Close()
		return fmt.Errorf("创建页面失败: %w", err)
	}
	in.page = page

	// 导航到 douyin.com（建立 origin，用于 sessionStorage 注入）
	log.Printf("[%s] 导航到 douyin.com", in.ID)
	if err := in.gotoWithRetry(page, "https://www.douyin.com/"); err != nil {
		log.Printf("[%s] 首次导航失败: %v", in.ID, err)
	}

	// 恢复 sessionStorage（Playwright StorageState 不包含 sessionStorage）
	if in.HasSavedState() {
		in.restoreSessionStorage()
	}

	// 导航到 /chat
	log.Printf("[%s] 导航到 /chat", in.ID)
	if err := in.gotoWithRetry(page, "https://www.douyin.com/chat"); err != nil {
		log.Printf("[%s] 打开 /chat 失败: %v", in.ID, err)
	}

	// 等待页面 JS 初始化，最多等20秒
	loggedIn := false
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		res, err := page.Evaluate(`!!(window.userInfoStore && window.userInfoStore.curLoginUserInfo)`)
		if err == nil {
			if v, ok := res.(bool); ok && v {
				loggedIn = true
				break
			}
		}
	}
	if !loggedIn {
		log.Printf("[%s] 未登录，再试 reload...", in.ID)
		_, _ = page.Reload(playwright.PageReloadOptions{
			WaitUntil: playwright.WaitUntilStateLoad,
		})
		time.Sleep(10 * time.Second)
	}

	title, _ := page.Evaluate("document.title")
	url, _ := page.Evaluate("location.href")
	log.Printf("[%s] 页面状态 title=%v url=%v", in.ID, title, url)

	wpAvail, _ := page.Evaluate(`typeof window.webpackChunkdouyin_web`)
	log.Printf("[%s] webpack可用: %v", in.ID, wpAvail)

	return nil
}

// connectCDP 通过 CDP 连接到已有 Chrome（AttachURL 模式）。
func (in *Instance) connectCDP(pw *playwright.Playwright, cdpURL string) error {
	browser, err := pw.Chromium.ConnectOverCDP(cdpURL, playwright.BrowserTypeConnectOverCDPOptions{
		Timeout: playwright.Float(15000),
	})
	if err != nil {
		return fmt.Errorf("CDP 连接失败 %s: %w", cdpURL, err)
	}
	in.browser = browser

	var ctx0 playwright.BrowserContext
	if len(browser.Contexts()) > 0 {
		ctx0 = browser.Contexts()[0]
	} else {
		ctx0, err = browser.NewContext(playwright.BrowserNewContextOptions{
			UserAgent: playwright.String(in.userAgent),
			Viewport:  &playwright.Size{Width: 1920, Height: 1080},
		})
		if err != nil {
			return fmt.Errorf("创建上下文失败: %w", err)
		}
	}
	in.context = ctx0

	page, err := in.context.NewPage()
	if err != nil {
		return fmt.Errorf("创建页面失败: %w", err)
	}
	in.page = page

	if err := in.gotoWithRetry(page, "https://www.douyin.com/chat"); err != nil {
		log.Printf("[%s] 打开 /chat 失败: %v", in.ID, err)
	}
	return nil
}

func (in *Instance) gotoWithRetry(page playwright.Page, url string) error {
	var lastErr error
	for i := 0; i < 3; i++ {
		_, err := page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateLoad,
			Timeout:   playwright.Float(45000),
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
func (in *Instance) Page() playwright.Page {
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
	pageOrigin, _ := in.page.Evaluate("location.origin")
	pageOriginStr, _ := pageOrigin.(string)
	items, ok := state.SessionStorage[pageOriginStr]
	if !ok || len(items) == 0 {
		return
	}
	for _, item := range items {
		script := fmt.Sprintf(`sessionStorage.setItem(%s, %s)`,
			safeJSStr(item.Name), safeJSStr(item.Value))
		if _, err := in.page.Evaluate(script); err != nil {
			log.Printf("[%s] sessionStorage 恢复失败 %s: %v", in.ID, item.Name, err)
		}
	}
	log.Printf("[%s] 已恢复 %d 个 sessionStorage 项 (origin=%s)", in.ID, len(items), pageOriginStr)
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
	_ = page.Mouse().Move(x-30, y+8)
	time.Sleep(40 * time.Millisecond)
	_ = page.Mouse().Move(x, y)
	time.Sleep(60 * time.Millisecond)
	if err := page.Mouse().Down(); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return page.Mouse().Up()
}

// RightClick 在页面指定坐标处模拟右键点击。
func (in *Instance) RightClick(x, y float64) error {
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	_ = page.Mouse().Move(x-30, y+8)
	time.Sleep(40 * time.Millisecond)
	_ = page.Mouse().Move(x, y)
	time.Sleep(60 * time.Millisecond)
	if err := page.Mouse().Down(playwright.MouseDownOptions{Button: playwright.MouseButtonRight}); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return page.Mouse().Up(playwright.MouseUpOptions{Button: playwright.MouseButtonRight})
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
	_ = page.Mouse().Move(fromX, fromY)
	time.Sleep(80 * time.Millisecond)
	if err := page.Mouse().Down(); err != nil {
		return err
	}

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
		_ = page.Mouse().Move(nx+jx, ny+jy)
		time.Sleep(time.Duration(10+jitterN(18)) * time.Millisecond)
	}
	// 终点精确落点并短暂停顿后再抬起
	_ = page.Mouse().Move(toX, toY)
	time.Sleep(90 * time.Millisecond)
	return page.Mouse().Up()
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
	return in.page.Keyboard().Type(text)
}

// KeyPress 模拟按键（Enter/Escape/Tab/Backspace 等）。
func (in *Instance) KeyPress(key string) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page == nil {
		return fmt.Errorf("页面未就绪")
	}
	return in.page.Keyboard().Press(key)
}

// ViewportSize 返回页面实际客户区尺寸（innerWidth/innerHeight，
// 含滚动条修正，供截图坐标换算使用）。
func (in *Instance) ViewportSize() (float64, float64) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page == nil {
		return 1920, 1080
	}
	w, _ := in.page.Evaluate("window.innerWidth")
	h, _ := in.page.Evaluate("window.innerHeight")
	wf := toFloat(w)
	hf := toFloat(h)
	if wf > 0 && hf > 0 {
		return wf, hf
	}
	if vp := in.page.ViewportSize(); vp != nil && vp.Width > 0 {
		return float64(vp.Width), float64(vp.Height)
	}
	return 1920, 1080
}

func toFloat(v interface{}) float64 {
	switch t := v.(type) {
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case float64:
		return t
	default:
		return 0
	}
}

// SetOnNewMessages 设置新消息回调。
func (in *Instance) SetOnNewMessages(cb func(jsonRaw string)) {
	in.mu.Lock()
	in.onNewMsgs = cb
	in.mu.Unlock()
}

// InitSDK 在后台初始化 IM SDK（不持锁，不阻塞其他操作）。
func (in *Instance) InitSDK(timeout time.Duration) error {
	if in.page == nil {
		return fmt.Errorf("页面未就绪")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := in.page.Evaluate(jsBootstrap)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		var out struct {
			Ok      bool   `json:"ok"`
			SelfUID string `json:"self_uid"`
			Error   string `json:"error"`
			ModID   int    `json:"mod_id"`
		}
		var raw string
		switch v := res.(type) {
		case string:
			raw = v
		default:
			b, _ := json.Marshal(res)
			raw = string(b)
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
			_, _ = in.page.Evaluate(jsRegisterReceiver)
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
	defer in.mu.Unlock()

	in.registerBindingOnce()

	if savedModID := in.LoadModID(); savedModID > 0 {
		_, _ = in.page.Evaluate(fmt.Sprintf("window.__obModId = %d", savedModID))
	}

	var lastErr error
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		res, err := in.page.Evaluate(jsBootstrap)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		var out struct {
			Ok      bool   `json:"ok"`
			SelfUID string `json:"self_uid"`
			Error   string `json:"error"`
			ModID   int    `json:"mod_id"`
		}
		var raw string
		switch v := res.(type) {
		case string:
			raw = v
		default:
			b, _ := json.Marshal(res)
			raw = string(b)
		}
		if json.Unmarshal([]byte(raw), &out) == nil {
			if out.Ok {
				in.selfUID = out.SelfUID
				if out.ModID > 0 {
					in.SaveModID(out.ModID)
				}
				uid, nick := in.fetchSelfInfoLocked()
				if uid != "" {
					in.selfUID = uid
				}
				in.nickname = nick
			log.Printf("[%s] IM SDK 初始化成功 self_uid=%s nickname=%s mod_id=%v", in.ID, in.selfUID, in.nickname, out.ModID)
			in.registerBindingOnce()
			_, _ = in.page.Evaluate(jsRegisterReceiver)
			in.sdkReady = true
			in.mu.Unlock()
				if err := in.SaveState(); err != nil {
					log.Printf("[%s] 保存登录态失败: %v", in.ID, err)
				} else {
					log.Printf("[%s] 登录态保存成功", in.ID)
				}
				in.mu.Lock()
				return nil
			}
			lastErr = fmt.Errorf("%s", out.Error)
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("IM SDK 初始化超时: %v", lastErr)
}

// EnsureSDK 快速检查 SDK 是否就绪，未就绪则自动重初始化。
// 使用缓存标志避免快速路径 page.Evaluate 阻塞（Playwright 串行化）。
func (in *Instance) EnsureSDK() error {
	in.mu.Lock()
	if in.page == nil {
		in.mu.Unlock()
		return fmt.Errorf("页面未就绪")
	}
	if in.sdkReady {
		in.mu.Unlock()
		return nil // SDK 已就绪（缓存命中）
	}
	page := in.page
	in.mu.Unlock()
	// SDK 丢失（页面刷新/导航后），尝试重初始化
	// 注意：不在这里持有 mu，避免阻塞其他 API 调用
	log.Printf("[%s] SDK 不可用，尝试自动重初始化...", in.ID)
	if err := in.ensureSDKLocked(page); err != nil {
		return err
	}
	return nil
}

// ensureSDKLocked 执行 SDK 重初始化（由 reinitMu 保证不并发执行）。
func (in *Instance) ensureSDKLocked(page playwright.Page) error {
	in.reinitMu.Lock()
	defer in.reinitMu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	var lastErr error
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		res, err := page.Evaluate(jsBootstrap)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		var out struct {
			Ok      bool   `json:"ok"`
			SelfUID string `json:"self_uid"`
			Error   string `json:"error"`
			ModID   int    `json:"mod_id"`
		}
		var raw string
		switch v := res.(type) {
		case string:
			raw = v
		default:
			b, _ := json.Marshal(res)
			raw = string(b)
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
			_, _ = in.page.Evaluate(jsRegisterReceiver)
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
	res, err := page.Evaluate(`(async () => {
		var ctx = window.__imCtx;
		if (!ctx || !ctx.imSdkService) return JSON.stringify({ ok: false, error: 'imSdkService 不可用' });
		var clm = ctx.imSdkService.conversationListManager;
		if (!clm || !clm.getAllConversation) return JSON.stringify({ ok: false, error: 'getAllConversation 方法不可用' });
		var result = await clm.getAllConversation();
		return JSON.stringify({ ok: true, result: result });
	})()`)
	if err != nil {
		return nil, err
	}
	var out struct {
		Ok     bool        `json:"ok"`
		Error  string      `json:"error"`
		Result interface{} `json:"result"`
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
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
// 当页面崩溃/导航导致 SDK 丢失时自动恢复。
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
			// 先检查页面是否存活（不持锁评估 JS）
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

			// 检查页面是否存活（不持锁）
			_, err := page.Evaluate(`1`)
			if err != nil {
				log.Printf("[%s] 健康监控: 页面不可用 (%v)，等待恢复...", in.ID, err)
				time.Sleep(3 * time.Second)
				_, err2 := page.Evaluate(`1`)
				if err2 != nil {
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
			// 检查 SDK 是否可用（不持锁）
			sdkRes, sdkErr := page.Evaluate(`!!(window.__sdkInst && window.__imCtx)`)
			sdkOK := false
			if sdkErr == nil {
				if v, ok := sdkRes.(bool); ok {
					sdkOK = v
				}
			}
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
	res, err := in.page.Evaluate(jsGetSelfInfo)
	if err != nil {
		return "", ""
	}
	var out struct {
		UID      string `json:"uid"`
		Nickname string `json:"nickname"`
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
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
	res, err := page.Evaluate(fmt.Sprintf(`(function(){try{var sdk=window.__imCtx&&window.__imCtx.imSdkService;if(!sdk||!sdk.userCacheManager||!sdk.userCacheManager.getUserInfo)return"";var info=sdk.userCacheManager.getUserInfo(%q);return info?(info.nickname||""):""}catch(e){return""}})()`, uid))
	if err != nil {
		return ""
	}
	if s, ok := res.(string); ok {
		return s
	}
	return ""
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
	res, err := in.page.Evaluate(jsGetSDKStatus)
	if err != nil {
		return SDKStatus{}
	}
	var out SDKStatus
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// CheckLoginWithUser 检测登录状态并返回完整用户信息。
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
	res, err := in.page.Evaluate(jsCheckLogin)
	if err != nil {
		return LoginCheckResult{}
	}
	var out LoginCheckResult
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
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
	res, err := in.page.Evaluate(jsGetSelfInfo)
	if err != nil {
		return "", ""
	}
	var out struct {
		UID      string `json:"uid"`
		Nickname string `json:"nickname"`
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
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
	defer in.mu.Unlock()
	if in.page == nil {
		return false, nil
	}
	res, err := in.page.Evaluate(jsCheckLogin)
	if err != nil {
		return false, err
	}
	var out struct {
		LoggedIn bool `json:"logged_in"`
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
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
	// 确保在 douyin.com 页面上
	if !strings.Contains(in.page.URL(), "douyin.com") {
		_, _ = in.page.Goto("https://www.douyin.com/chat", playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(30000),
		})
		time.Sleep(5 * time.Second)
	}

	// 重试获取 DOM QR（页面可能需要时间渲染登录面板）
	var lastErr string
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
			// 刷新页面重试
			_, _ = in.page.Reload(playwright.PageReloadOptions{
				WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			})
			time.Sleep(3 * time.Second)
		}
		res, err := in.page.Evaluate(jsGetQRCode)
		if err != nil {
			lastErr = err.Error()
			continue
		}

		var out struct {
			OK          bool   `json:"ok"`
			Error       string `json:"error"`
			ImageBase64 string `json:"image_base64"`
			Token       string `json:"token"`
			Method      string `json:"method"`
		}
		var raw string
		switch v := res.(type) {
		case string:
			raw = v
		default:
			b, _ := json.Marshal(res)
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
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsCheckQRCode, string(argJSON)))
	if err != nil {
		return -1, "", err
	}

	var out struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error"`
		Status      int    `json:"status"`
		RedirectURL string `json:"redirect_url"`
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
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

// ImportCookies 导入 cookie 到当前浏览器上下文。
func (in *Instance) ImportCookies(cookies []playwright.Cookie) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	opt := make([]playwright.OptionalCookie, 0, len(cookies))
	for _, c := range cookies {
		opt = append(opt, cookieToOptional(c))
	}
	return in.context.AddCookies(opt)
}

// WaitLoginSuccess 轮询等待扫码成功；成功后保存登录态。
func (in *Instance) WaitLoginSuccess(ctx context.Context, pollEvery time.Duration) error {
	token := in.QRToken()
	if token == "" {
		// 无token则直接轮询页面登录状态
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
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollEvery):
		}
		status, redirectURL, err := in.CheckQRCode(token)
		if err != nil {
			log.Printf("[%s] 轮询扫码状态失败: %v", in.ID, err)
			continue
		}
		switch status {
		case 1:
			log.Printf("[%s] QR码已扫码，等待确认...", in.ID)
		case 2:
			log.Printf("[%s] QR码已确认，跳转: %s", in.ID, redirectURL)
			targetURL := redirectURL
			if targetURL == "" {
				targetURL = "https://www.douyin.com"
			}
			in.mu.Lock()
			_, _ = in.page.Goto(targetURL, playwright.PageGotoOptions{
				WaitUntil: playwright.WaitUntilStateLoad,
				Timeout:   playwright.Float(45000),
			})
			time.Sleep(5 * time.Second)
			_, _ = in.page.Goto("https://www.douyin.com/chat", playwright.PageGotoOptions{
				WaitUntil: playwright.WaitUntilStateLoad,
				Timeout:   playwright.Float(45000),
			})
			time.Sleep(8 * time.Second)
			in.mu.Unlock()
			if err := in.SaveState(); err != nil {
				log.Printf("[%s] 登录后保存 cookies 失败: %v", in.ID, err)
			}
			return nil
		case 3:
			return fmt.Errorf("QR码已过期")
		case 0:
			// 未扫码，继续轮询
		}
	}
}

// ReloadAndInit 重新加载页面并初始化 IM SDK。
func (in *Instance) ReloadAndInit(timeout time.Duration) error {
	in.mu.Lock()
	_, err := in.page.Goto("https://www.douyin.com/chat", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	})
	in.mu.Unlock()
	if err != nil {
		return err
	}
	return in.EnsureReady(timeout)
}

// Disconnect 保存状态并关闭浏览器（Playwright 自动终止 Chromium 进程）。
func (in *Instance) Disconnect() {
	// 先保存完整浏览器状态
	if err := in.SaveState(); err != nil {
		log.Printf("[%s] 断开前保存状态失败: %v", in.ID, err)
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.inputSession != nil {
		in.inputSession.close()
		in.inputSession = nil
	}
	if in.context != nil {
		_ = in.context.Close()
		in.context = nil
	}
	if in.browser != nil {
		_ = in.browser.Close()
		in.browser = nil
	}
	in.page = nil
	in.bindingReady = false
}

// Close 优雅关闭实例：保存状态 → 关闭浏览器。
func (in *Instance) Close() {
	in.mu.Lock()
	s := in.inputSession
	in.inputSession = nil
	in.mu.Unlock()
	if s != nil {
		s.close()
	}
	// 先保存完整浏览器状态到 state.json
	if err := in.SaveState(); err != nil {
		log.Printf("[%s] 关闭前保存状态失败: %v", in.ID, err)
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.context != nil {
		_ = in.context.Close()
		in.context = nil
	}
	if in.browser != nil {
		_ = in.browser.Close()
		in.browser = nil
	}
	in.page = nil
	in.bindingReady = false
}

// Running 浏览器是否在运行。
func (in *Instance) Running() bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.browser != nil || in.AttachURL != ""
}

var _ = context.Background // 保持 context 导入

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
	_, err := page.Evaluate(fmt.Sprintf(`window.__obAccountId = %q; window.__obCallbackURL = %q;`, accountID, cbURL))
	if err != nil {
		log.Printf("[%s] 注入回调变量失败: %v", accountID, err)
		return
	}
	in.mu.Lock()
	in.bindingReady = true
	in.mu.Unlock()
	log.Printf("[%s] 回调变量注入成功 url=%s", accountID, cbURL)
}

// StartMessagePolling 启动 Go 侧轮询：每 2s 通过 page.Evaluate 拉取 JS 侧 __obNewMsgs 队列，
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
			res, err := page.Evaluate(jsDrainNewMsgs)
			if err != nil {
				continue
			}
			var out struct {
				Ok    bool                     `json:"ok"`
				Msgs  []map[string]interface{} `json:"msgs"`
				Error string                   `json:"error"`
			}
			var raw string
			switch v := res.(type) {
			case string:
				raw = v
			default:
				b, _ := json.Marshal(res)
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
