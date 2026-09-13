package browser

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-rod/rod/lib/proto"
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
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("page 未初始化")
	}
	_ = os.MkdirAll(in.StorageDir, 0o755)
	path := filepath.Join(in.StorageDir, "state.json")
	tmpPath := path + ".tmp"

	// 1. 通过 CDP 获取 cookies
	cookies, err := proto.NetworkGetCookies{}.Call(page)
	if err != nil {
		cookies = &proto.NetworkGetCookiesResult{}
	}

	// 2. 通过 JS 获取 localStorage
	lsData := in.collectLocalStorage()

	// 3. 组装 state 结构
	state := make(map[string]interface{})
	cookieList := make([]map[string]interface{}, 0)
	if cookies != nil {
		for _, c := range cookies.Cookies {
			cookieList = append(cookieList, map[string]interface{}{
				"name":     c.Name,
				"value":    c.Value,
				"domain":   c.Domain,
				"path":     c.Path,
				"expires":  c.Expires,
				"httpOnly": c.HTTPOnly,
				"secure":   c.Secure,
				"sameSite": string(c.SameSite),
			})
		}
	}
	state["cookies"] = cookieList

	lsMap := make(map[string]interface{})
	for origin, items := range lsData {
		lsArr := make([]map[string]string, 0, len(items))
		for _, item := range items {
			lsArr = append(lsArr, map[string]string{"name": item.Name, "value": item.Value})
		}
		lsMap[origin] = lsArr
	}
	state["origins"] = lsMap

	// 4. 写入临时文件
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}

	// 5. 追加 sessionStorage
	if page != nil {
		ssData := in.collectSessionStorage()
		if len(ssData) > 0 {
			_ = in.mergeSessionStorage(tmpPath, ssData)
		}
	}

	// 6. 原子替换
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.WriteFile(path, raw, 0o644)
		_ = os.Remove(tmpPath)
	}
	return nil
}

// collectLocalStorage 从当前页面收集 localStorage。
func (in *Instance) collectLocalStorage() map[string][]sessionStorageItem {
	result := make(map[string][]sessionStorageItem)
	res := in.page.MustEval(`() => {
		var out = {};
		for (var i = 0; i < localStorage.length; i++) {
			var key = localStorage.key(i);
			out[key] = localStorage.getItem(key);
		}
		return JSON.stringify(out);
	}`)
	var raw map[string]string
	var ss string
	if str := res.Str(); str != "" {
		ss = str
	} else {
		b, _ := json.Marshal(res.Val())
		ss = string(b)
	}
	if json.Unmarshal([]byte(ss), &raw) != nil {
		return nil
	}
	origin := in.page.MustEval(`() => location.origin`).Str()
	if origin == "" {
		return nil
	}
	items := make([]sessionStorageItem, 0, len(raw))
	for k, v := range raw {
		items = append(items, sessionStorageItem{Name: k, Value: v})
	}
	result[origin] = items
	return result
}

// collectSessionStorage 从当前页面收集 sessionStorage。
func (in *Instance) collectSessionStorage() map[string][]sessionStorageItem {
	result := make(map[string][]sessionStorageItem)
	res := in.page.MustEval(`() => {
		var out = {};
		for (var i = 0; i < sessionStorage.length; i++) {
			var key = sessionStorage.key(i);
			out[key] = sessionStorage.getItem(key);
		}
		return JSON.stringify(out);
	}`)
	var raw map[string]string
	var ss string
	if str := res.Str(); str != "" {
		ss = str
	} else {
		b, _ := json.Marshal(res.Val())
		ss = string(b)
	}
	if json.Unmarshal([]byte(ss), &raw) != nil {
		return nil
	}
	origin := in.page.MustEval(`() => location.origin`).Str()
	if origin == "" {
		return nil
	}
	items := make([]sessionStorageItem, 0, len(raw))
	for k, v := range raw {
		items = append(items, sessionStorageItem{Name: k, Value: v})
	}
	result[origin] = items
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

// HasSavedState 实例目录是否存在可恢复的登录态。
// 检查 state.json 存在且包含 sessionid cookie。
func (in *Instance) HasSavedState() bool {
	path := filepath.Join(in.StorageDir, "state.json")
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 10 {
		return false
	}
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
