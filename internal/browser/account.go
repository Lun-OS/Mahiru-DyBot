package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mahiru-dybot/internal/config"
	"mahiru-dybot/internal/eventbus"
)

// 账号生命周期状态。
const (
	StateStopped   = "stopped"    // 浏览器未启动
	StateStarting  = "starting"   // 正在拉起浏览器/注入
	StateQRPending = "qr_pending" // 等待扫码登录
	StateOnline    = "online"     // 已登录且 IM SDK 就绪
	StateError     = "error"      // 启动/登录失败
)

// AccountMeta 账号持久化元数据（accounts.json）。
type AccountMeta struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	UID            string `json:"uid,omitempty"`
	Nickname       string `json:"nickname,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	ViewportWidth  int    `json:"viewport_width,omitempty"`
	ViewportHeight int    `json:"viewport_height,omitempty"`
	CustomUA       string `json:"custom_ua,omitempty"`
}

// AccountInfo 运行时账号信息（meta + 状态）。
type AccountInfo struct {
	AccountMeta
	State string `json:"state"`
	Error string `json:"error,omitempty"`

	ActualUA            string `json:"actual_ua,omitempty"`
	ActualViewportWidth  int    `json:"actual_viewport_width,omitempty"`
	ActualViewportHeight int    `json:"actual_viewport_height,omitempty"`
}

// accountsFile accounts.json 结构。
type accountsFile struct {
	Version  int           `json:"version"`
	Accounts []AccountMeta `json:"accounts"`
}

// Account 一个抖音账号：元数据 + 运行中的浏览器实例。
type Account struct {
	Meta AccountMeta

	mu      sync.Mutex
	state   string
	lastErr string
	inst    *Instance

	onStateChange func(a *Account, prev, cur, lastErr string)
}

// Instance 返回运行中的实例（未启动返回 nil）。
func (a *Account) Instance() *Instance {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inst
}

// State 当前状态。
func (a *Account) State() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

func (a *Account) setState(s, lastErr string) {
	a.mu.Lock()
	prev := a.state
	a.state = s
	a.lastErr = lastErr
	cb := a.onStateChange
	meta := a.Meta
	a.mu.Unlock()
	if cb != nil && prev != s {
		cb(a, prev, s, lastErr)
	}
	log.Printf("[账号:%s/%s] %s -> %s%s", meta.ID, meta.Name, prev, s, errSuffix(lastErr))
}

func errSuffix(e string) string {
	if e == "" {
		return ""
	}
	return " (" + e + ")"
}

// AccountManager 多账号注册表。
type AccountManager struct {
	mu         sync.RWMutex
	startMu    sync.Mutex // 串行化 Start，防并发双开
	rootDir    string     // storage/
	accDir     string     // storage/accounts
	accounts   map[string]*Account
	bus        *eventbus.Bus
	listenAddr string // 服务监听地址，用于构建 JS→Go 回调 URL
}

// NewAccountManager 加载 accounts.json 并构建管理器。自动扫描磁盘补充 accounts.json 缺失的目录。
func NewAccountManager(storageRoot string, bus *eventbus.Bus) (*AccountManager, error) {
	accDir := filepath.Join(storageRoot, "accounts")
	if err := os.MkdirAll(accDir, 0o755); err != nil {
		return nil, err
	}
	am := &AccountManager{rootDir: storageRoot, accDir: accDir, accounts: map[string]*Account{}, bus: bus}
	data, err := os.ReadFile(filepath.Join(accDir, "accounts.json"))
	if err == nil {
		var af accountsFile
		if json.Unmarshal(data, &af) == nil {
			for _, m := range af.Accounts {
				am.accounts[m.ID] = &Account{Meta: m, state: StateStopped}
			}
		}
	}
	// 扫描磁盘：补充 accounts.json 中没有但目录存在的账号
	entries, _ := os.ReadDir(accDir)
	known := map[string]bool{}
	for id := range am.accounts {
		known[id] = true
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "acc_") {
			continue
		}
		if known[e.Name()] {
			continue
		}
		// 目录存在但 accounts.json 没有 → 自动补充
		meta := AccountMeta{
			ID:        e.Name(),
			Name:      "恢复的账号",
			CreatedAt: time.Now().Unix(),
		}
		am.accounts[meta.ID] = &Account{Meta: meta, state: StateStopped}
		am.accounts[meta.ID].onStateChange = am.publishState
		log.Printf("[INIT] 扫描发现未注册账号目录: %s，已自动补充", e.Name())
	}
	for _, a := range am.accounts {
		a.onStateChange = am.publishState
	}
	log.Printf("[INIT] 账号管理器就绪，已加载 %d 个账号", len(am.accounts))
	return am, nil
}

// SetListenAddr 设置服务监听地址，用于构建 JS→Go 消息回调 URL。
func (am *AccountManager) SetListenAddr(addr string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.listenAddr = addr
}

// publishState 发布账号状态事件到总线。
func (am *AccountManager) publishState(a *Account, prev, cur, lastErr string) {
	if am.bus == nil {
		return
	}
	am.bus.Publish(eventbus.TopicAccount, AccountStateEvent{
		AccountID: a.Meta.ID,
		Name:      a.Meta.Name,
		PrevState: prev,
		State:     cur,
		Error:     lastErr,
	})
}

// saveLocked 持久化 accounts.json（需持锁）。
func (am *AccountManager) saveLocked() {
	af := accountsFile{Version: 1}
	for _, a := range am.accounts {
		af.Accounts = append(af.Accounts, a.Meta)
	}
	data, _ := json.MarshalIndent(af, "", "  ")
	_ = os.WriteFile(filepath.Join(am.accDir, "accounts.json"), data, 0o644)
}

// Create 新建账号（仅登记元数据，不启动浏览器）。
func (am *AccountManager) Create(name string) (*AccountMeta, error) {
	if name == "" {
		name = "账号"
	}
	am.mu.Lock()
	defer am.mu.Unlock()
	m := AccountMeta{
		ID:        config.NewID("acc"),
		Name:      name,
		CreatedAt: config.NowUnix(),
	}
	am.accounts[m.ID] = &Account{Meta: m, state: StateStopped}
	am.accounts[m.ID].onStateChange = am.publishState
	am.saveLocked()
	log.Printf("[账号:%s] 已创建 (%s)", m.ID, name)
	return &m, nil
}

// Rename 重命名账号。
func (am *AccountManager) Rename(id, name string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	a, ok := am.accounts[id]
	if !ok {
		return errors.New("账号不存在")
	}
	a.Meta.Name = name
	am.saveLocked()
	return nil
}

// UpdateSettings 更新账号设置（viewport、UA 等）。
func (am *AccountManager) UpdateSettings(id string, viewportWidth, viewportHeight int, customUA string, vpChanged bool) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	a, ok := am.accounts[id]
	if !ok {
		return errors.New("账号不存在")
	}
	if vpChanged {
		if viewportWidth > 0 {
			a.Meta.ViewportWidth = viewportWidth
		} else {
			a.Meta.ViewportWidth = 0
		}
		if viewportHeight > 0 {
			a.Meta.ViewportHeight = viewportHeight
		} else {
			a.Meta.ViewportHeight = 0
		}
	}
	a.Meta.CustomUA = customUA
	am.saveLocked()
	return nil
}

// Delete 删除账号：停止浏览器、移除数据目录与元数据。
func (am *AccountManager) Delete(id string) error {
	_ = am.Stop(id)
	am.mu.Lock()
	defer am.mu.Unlock()
	a, ok := am.accounts[id]
	if !ok {
		return errors.New("账号不存在")
	}
	_ = os.RemoveAll(am.instanceDir(id))
	delete(am.accounts, id)
	am.saveLocked()
	log.Printf("[账号:%s] 已删除 (%s)", id, a.Meta.Name)
	return nil
}

// instanceDir 账号私有存储目录。
func (am *AccountManager) instanceDir(id string) string {
	return filepath.Join(am.accDir, id)
}

// InstanceDir 返回账号私有存储目录（公开方法）。
func (am *AccountManager) InstanceDir(id string) (string, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	if _, ok := am.accounts[id]; !ok {
		return "", false
	}
	return am.instanceDir(id), true
}

// Get 获取账号。
func (am *AccountManager) Get(id string) (*Account, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	a, ok := am.accounts[id]
	return a, ok
}

// List 返回全部账号（含运行时状态）。
func (am *AccountManager) List() []AccountInfo {
	am.mu.RLock()
	defer am.mu.RUnlock()
	out := make([]AccountInfo, 0, len(am.accounts))
	for _, a := range am.accounts {
		info := AccountInfo{AccountMeta: a.Meta, State: a.State(), Error: a.lastErr}
		out = append(out, info)
	}
	return out
}

// OnlineCount 在线账号数。
func (am *AccountManager) OnlineCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	n := 0
	for _, a := range am.accounts {
		if a.State() == StateOnline {
			n++
		}
	}
	return n
}

// LastError 账号最近一次错误（供展示）。
func (a *Account) LastError() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastErr
}

// Info 返回单个账号运行时信息。
func (am *AccountManager) Info(id string) (*AccountInfo, bool) {
	a, ok := am.Get(id)
	if !ok {
		return nil, false
	}
	info := &AccountInfo{AccountMeta: a.Meta, State: a.State(), Error: a.LastError()}
	if a.inst != nil {
		a.inst.mu.Lock()
		info.ActualUA = a.inst.userAgent
		info.ActualViewportWidth = a.inst.viewportWidth
		info.ActualViewportHeight = a.inst.viewportHeight
		a.inst.mu.Unlock()
	}
	return info, true
}

// Resolve 选择目标账号：
//   - id 非空 → 定向查找（须在线）
//   - id 为空 → 仅一个在线账号时自动选择，否则报错并列出可用账号
func (am *AccountManager) Resolve(id string) (*Account, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	if id != "" {
		a, ok := am.accounts[id]
		if !ok {
			// ID 不匹配时，回退到第一个在线账号（兼容 OneBot 客户端发任意 self_id）
			for _, a2 := range am.accounts {
				if a2.State() == StateOnline {
					return a2, nil
				}
			}
			return nil, fmt.Errorf("账号不存在: %s", id)
		}
		if a.State() != StateOnline {
			return nil, fmt.Errorf("账号 %s 不在线(当前 %s)", id, a.State())
		}
		return a, nil
	}
	var online []*Account
	for _, a := range am.accounts {
		if a.State() == StateOnline {
			online = append(online, a)
		}
	}
	switch len(online) {
	case 0:
		return nil, errors.New("没有在线账号，请先通过 WebUI 创建并登录")
	case 1:
		return online[0], nil
	default:
		ids := ""
		for i, a := range online {
			if i > 0 {
				ids += ", "
			}
			ids += fmt.Sprintf("%s(%s)", a.Meta.ID, a.Meta.Name)
		}
		return nil, fmt.Errorf("存在多个在线账号，请在参数中指定 account_id 或 self_id。可用: %s", ids)
	}
}

// Start 启动账号浏览器（异步）：
// 有本地登录态且有效 → 直接进入 online；
// 无登录态/登录失效 → 进入 qr_pending 等待扫码。
func (am *AccountManager) Start(id string) error {
	am.startMu.Lock()
	defer am.startMu.Unlock()
	a, ok := am.Get(id)
	if !ok {
		return errors.New("账号不存在")
	}
	a.mu.Lock()
	cur := a.state
	inst := a.inst
	dir := am.instanceDir(id)
	name := a.Meta.Name
	a.mu.Unlock()

	if cur == StateOnline || cur == StateStarting || cur == StateQRPending {
		return fmt.Errorf("账号已在运行中(状态 %s)", cur)
	}

	// Stop 后 Start：尝试复用已有实例（Chrome 进程仍存活）
	if inst != nil {
		if err := inst.Launch(); err != nil {
			log.Printf("[账号:%s/%s] 复用已有实例失败，重新启动: %v", id, name, err)
			inst.Close()
			a.mu.Lock()
			a.inst = nil
			a.mu.Unlock()
		} else {
			a.setState(StateStarting, "")
			go am.startAsync(a, inst, id, name)
			return nil
		}
	}

	a.setState(StateStarting, "")

	in, err := NewInstance(id, dir, a.Meta.CustomUA, a.Meta.ViewportWidth, a.Meta.ViewportHeight)
	if err != nil {
		a.setState(StateError, err.Error())
		return err
	}
	// 自动生成的 UA 写回 accounts.json
	if a.Meta.CustomUA == "" {
		a.Meta.CustomUA = in.userAgent
		am.saveLocked()
	}
	// 消息回调 → 总线
	in.SetOnNewMessages(func(raw string) {
		log.Printf("[账号:%s] 收到新消息回调 len=%d", id, len(raw))
		if am.bus != nil {
			am.bus.Publish(eventbus.TopicMessage, MessageEvent{AccountID: id, Raw: raw})
		}
	})
	// 设置 JS→Go 消息回调 URL
	am.mu.RLock()
	listenAddr := am.listenAddr
	am.mu.RUnlock()
	if listenAddr != "" {
		in.SetCallbackURL("http://127.0.0.1" + listenAddr + "/api/internal/msg/" + id)
	}

	a.mu.Lock()
	a.inst = in
	a.mu.Unlock()

	// 异步启动：Launch + 登录检测 + EnsureReady 全部在后台运行
	go am.startAsync(a, in, id, name)
	return nil
}

// startAsync 后台启动流程，不阻塞 HTTP 响应。
func (am *AccountManager) startAsync(a *Account, in *Instance, id, name string) {
	if err := in.Launch(); err != nil {
		in.Close()
		a.mu.Lock()
		a.inst = nil
		a.mu.Unlock()
		a.setState(StateError, err.Error())
		return
	}
	log.Printf("[账号:%s/%s] 浏览器已启动，检测登录态...", id, name)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	loggedIn, _ := in.IsLoggedIn(ctx)
	if loggedIn {
		a.setState(StateOnline, "")
		go am.initSDKBackground(a, in, id, name)
		return
	}
	a.setState(StateQRPending, "")

	// 后台自动检测登录：每5秒重检一次，QR扫码/手动登录后自动 Finalize
	go am.watchLogin(a, in, id, name)
}

// watchLogin 在 qr_pending 状态下轮询检测登录态，登录成功后自动完成 Finalize。
func (am *AccountManager) watchLogin(a *Account, in *Instance, id, name string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	maxWait := 10 * time.Minute // 最多等10分钟
	deadline := time.Now().Add(maxWait)
	for {
		select {
		case <-ticker.C:
			// 只在 qr_pending 状态下检测
			if a.State() != StateQRPending {
				return
			}
			if time.Now().After(deadline) {
				log.Printf("[账号:%s/%s] 自动登录检测超时(%v)，停止轮询", id, name, maxWait)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			ok, err := in.IsLoggedIn(ctx)
			cancel()
			if err != nil || !ok {
				continue
			}
			// 检测到已登录 → 自动 Finalize
			log.Printf("[账号:%s/%s] 自动检测到登录成功，执行 Finalize...", id, name)
			if ferr := am.FinalizeLogin(id); ferr != nil {
				log.Printf("[账号:%s/%s] 自动 Finalize 失败: %v", id, name, ferr)
				a.setState(StateError, "自动Finalize失败: "+ferr.Error())
			} else {
				log.Printf("[账号:%s/%s] 自动 Finalize 成功，已上线", id, name)
			}
			return
		}
	}
}

// initSDKBackground 后台初始化 SDK + 启动健康监控 + Go侧消息轮询。
func (am *AccountManager) initSDKBackground(a *Account, in *Instance, id, name string) {
	if err := in.InitSDK(60 * time.Second); err != nil {
		log.Printf("[账号:%s/%s] IM SDK初始化失败(不影响截图等操作): %v", id, name, err)
	} else {
		am.updateLoginMeta(a, in)
		// 启动 Go 侧消息轮询（绕过 HTTPS→HTTP 混合内容限制）
		in.StartMessagePolling(func(rawJSON string) {
			if am.bus != nil {
				am.bus.Publish(eventbus.TopicMessage, MessageEvent{AccountID: id, Raw: rawJSON})
			}
		})
		log.Printf("[账号:%s/%s] Go侧消息轮询已启动", id, name)
	}
	// 启动 SDK 健康监控（每30秒检查一次，页面崩溃时回调停止）
	in.StartHealthMonitor(30*time.Second, func() {
		log.Printf("[账号:%s/%s] 健康监控: 页面异常，标记为错误", id, name)
		a.setState(StateError, "浏览器页面异常")
	})
}

// updateLoginMeta 登录成功后回填 UID/昵称到元数据。
func (am *AccountManager) updateLoginMeta(a *Account, in *Instance) {
	a.mu.Lock()
	if u := in.SelfUID(); u != "" {
		a.Meta.UID = u
	}
	if n := in.SelfNickname(); n != "" {
		a.Meta.Nickname = n
	}
	a.mu.Unlock()
	am.mu.Lock()
	am.saveLocked()
	am.mu.Unlock()
}

// Stop 停止账号浏览器（断开 CDP 但保留 Chrome 进程，保留浏览器内存态）。
func (am *AccountManager) Stop(id string) error {
	a, ok := am.Get(id)
	if !ok {
		return errors.New("账号不存在")
	}
	a.mu.Lock()
	inst := a.inst
	a.mu.Unlock()
	a.setState(StateStopped, "")
	if inst != nil {
		inst.Disconnect()
		log.Printf("[账号:%s] 浏览器已断开（Chrome 进程保留）", id)
	}
	return nil
}

// FinalizeLogin 扫码确认后的收尾：EnsureReady + 回填元数据 + 置 online。
func (am *AccountManager) FinalizeLogin(id string) error {
	a, ok := am.Get(id)
	if !ok {
		return errors.New("账号不存在")
	}
	inst := a.Instance()
	if inst == nil {
		return errors.New("浏览器未启动")
	}
	if err := inst.EnsureReady(90 * time.Second); err != nil {
		a.setState(StateError, "IM SDK初始化失败: "+err.Error())
		return err
	}
	am.updateLoginMeta(a, inst)
	a.setState(StateOnline, "")
	// 启动健康监控
	inst.StartHealthMonitor(30*time.Second, func() {
		log.Printf("[账号:%s] 健康监控: 页面异常，标记为错误", id)
		a.setState(StateError, "浏览器页面异常")
	})
	return nil
}
