package browser

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"

	"github.com/playwright-community/playwright-go"
)

// modMeta IM SDK webpack 模块 ID 持久化。
type modMeta struct {
	ModID int `json:"mod_id"`
}

// SaveModID 保存 IM SDK 模块 ID 到实例目录 mod.json。
func (in *Instance) SaveModID(modID int) {
	_ = os.MkdirAll(in.StorageDir, 0o755)
	path := filepath.Join(in.StorageDir, "mod.json")
	data, _ := json.MarshalIndent(modMeta{ModID: modID}, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
}

// LoadModID 从实例目录加载上次成功的模块 ID，返回 -1 表示不存在。
func (in *Instance) LoadModID() int {
	path := filepath.Join(in.StorageDir, "mod.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	var meta modMeta
	if json.Unmarshal(data, &meta) != nil {
		return -1
	}
	return meta.ModID
}

// resolveUA 决定本实例的 User-Agent：
//  1. 配置了自定义 UA -> 直接使用
//  2. 都没有 -> 随机生成一个真实感 Chrome UA
func resolveUA(customUA string) string {
	if customUA != "" {
		return customUA
	}
	return randomChromeUA()
}

// randomChromeUA 生成随机版本的 Chrome UA（根据运行平台自动选择操作系统标识）。
func randomChromeUA() string {
	major, err := rand.Int(rand.Reader, big.NewInt(12))
	if err != nil {
		major = big.NewInt(0)
	}
	minor, _ := rand.Int(rand.Reader, big.NewInt(5))
	build, _ := rand.Int(rand.Reader, big.NewInt(10))
	ver := 120 + major.Int64() // 120 ~ 131
	osToken := "Windows NT 10.0; Win64; x64"
	switch runtime.GOOS {
	case "linux":
		osToken = "X11; Linux x86_64"
	case "darwin":
		osToken = "Macintosh; Intel Mac OS X 10_15_7"
	}
	return fmt.Sprintf(
		"Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.%d.%d.%d Safari/537.36",
		osToken, ver, minor.Int64(), build.Int64()+4000, build.Int64()*7+100,
	)
}

// SaveState 将当前登录态(cookies + localStorage + sessionStorage)持久化到 state.json。
// 使用原子写入：先写临时文件再 rename，防止写一半崩溃导致状态损坏。
func (in *Instance) SaveState() error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.context == nil {
		return fmt.Errorf("context 未初始化")
	}
	_ = os.MkdirAll(in.StorageDir, 0o755)
	path := filepath.Join(in.StorageDir, "state.json")
	tmpPath := path + ".tmp"

	// 1. 保存 cookies + localStorage（Playwright StorageState）到临时文件
	_, err := in.context.StorageState(playwright.BrowserContextStorageStateOptions{Path: playwright.String(tmpPath)})
	if err != nil {
		return err
	}

	// 2. 追加 sessionStorage（Playwright 不支持，通过 page JS 采集）
	if in.page != nil {
		ssData := in.collectSessionStorage()
		if len(ssData) > 0 {
			_ = in.mergeSessionStorage(tmpPath, ssData)
		}
	}

	// 3. 原子替换：rename 临时文件到正式路径
	if err := os.Rename(tmpPath, path); err != nil {
		// rename 失败时回退到直接写入（跨文件系统场景）
		// 重新调用 StorageState 直接写入正式路径
		_, err2 := in.context.StorageState(playwright.BrowserContextStorageStateOptions{Path: playwright.String(path)})
		if err2 != nil {
			// 最后手段：读取 tmp 写入正式路径
			data, readErr := os.ReadFile(tmpPath)
			if readErr == nil {
				_ = os.WriteFile(path, data, 0o644)
			}
		}
		_ = os.Remove(tmpPath)
	}
	return nil
}

// collectSessionStorage 从当前页面收集 sessionStorage。
func (in *Instance) collectSessionStorage() map[string][]sessionStorageItem {
	result := make(map[string][]sessionStorageItem)
	res, err := in.page.Evaluate(`(() => {
		var out = {};
		for (var i = 0; i < sessionStorage.length; i++) {
			var key = sessionStorage.key(i);
			out[key] = sessionStorage.getItem(key);
		}
		return JSON.stringify(out);
	})()`)
	if err != nil {
		return nil
	}
	var raw map[string]string
	var ss string
	switch v := res.(type) {
	case string:
		ss = v
	default:
		b, _ := json.Marshal(res)
		ss = string(b)
	}
	if json.Unmarshal([]byte(ss), &raw) != nil {
		return nil
	}
	origin, _ := in.page.Evaluate("location.origin")
	originStr, _ := origin.(string)
	if originStr == "" {
		return nil
	}
	items := make([]sessionStorageItem, 0, len(raw))
	for k, v := range raw {
		items = append(items, sessionStorageItem{Name: k, Value: v})
	}
	result[originStr] = items
	return result
}

// sessionStorageItem sessionStorage 键值对。
type sessionStorageItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// mergeSessionStorage 将 sessionStorage 追加到已有的 state.json。
func (in *Instance) mergeSessionStorage(path string, ssData map[string][]sessionStorageItem) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	ss := make(map[string]interface{})
	for origin, items := range ssData {
		ss[origin] = items
	}
	state["session_storage"] = ss
	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// cookieToOptional playwright.Cookie -> OptionalCookie 转换。
func cookieToOptional(c playwright.Cookie) playwright.OptionalCookie {
	oc := playwright.OptionalCookie{
		Name:     c.Name,
		Value:    c.Value,
		Domain:   playwright.String(c.Domain),
		Path:     playwright.String(c.Path),
		Expires:  playwright.Float(c.Expires),
		HttpOnly: playwright.Bool(c.HttpOnly),
		Secure:   playwright.Bool(c.Secure),
	}
	if c.SameSite != nil {
		ss := *c.SameSite
		oc.SameSite = &ss
	}
	return oc
}

// HasSavedState 实例目录是否存在可恢复的登录态。
// 检查 state.json 存在且包含 sessionid cookie。
func (in *Instance) HasSavedState() bool {
	path := filepath.Join(in.StorageDir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 10 {
		return false
	}
	// 验证文件内容是否包含有效 cookie
	var state struct {
		Cookies []struct {
			Name string `json:"name"`
		} `json:"cookies"`
	}
	if json.Unmarshal(data, &state) != nil {
		return false
	}
	for _, c := range state.Cookies {
		if c.Name == "sessionid" {
			return true
		}
	}
	return false
}
