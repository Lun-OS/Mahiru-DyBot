package onebot

// OneBot action 注册表与分发核心。
// 所有功能以 Action 形式注册，HTTP POST / <action>、正向WS、反向WS 三通道共用同一分发器。
// 新增 API：在 actions_*.go 中实现 handler 并调用 Register() 即可，无需改动通道代码。

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mahiru-dybot/internal/browser"
)

// ActionContext 单次 action 调用上下文。
type ActionContext struct {
	Action  string            `json:"-"`
	Params  map[string]interface{} `json:"-"` // 解析后的参数表（可能为空）
	RawParams json.RawMessage `json:"-"`
	Echo    interface{}       `json:"-"`
	Server  *Server           `json:"-"`
	Account *browser.Account  `json:"-"` // 已解析的目标账号（账号无关的 action 为 nil）
}

// Bind 将参数解码到结构体。
func (c *ActionContext) Bind(v interface{}) error {
	if len(c.RawParams) == 0 {
		return nil
	}
	return json.Unmarshal(c.RawParams, v)
}

// AccountIDParam 返回请求中的账号路由标识（account_id 优先，其次 self_id）。
func (c *ActionContext) AccountIDParam() string {
	if c.Params == nil {
		return ""
	}
	if v, ok := c.Params["account_id"]; ok {
		if s := ToString(v); s != "" && s != "0" {
			return s
		}
	}
	if v, ok := c.Params["self_id"]; ok {
		if s := ToString(v); s != "" && s != "0" {
			return s
		}
	}
	return ""
}

// ActionHandler action 处理函数。
type ActionHandler func(ctx *ActionContext) *ActionResult

var registry = map[string]ActionHandler{}

// Register 注册一个 action。重复注册会覆盖并告警。
func Register(name string, h ActionHandler) {
	if _, dup := registry[name]; dup {
		fmt.Printf("[WARN] action %s 被重复注册，已覆盖\n", name)
	}
	registry[name] = h
}

// RegisteredActions 返回全部已注册 action 名（调试用）。
func RegisteredActions() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// Dispatch 分发一个 action：
//  1. 查注册表；2. 解析账号路由；3. 执行 handler。
func (s *Server) Dispatch(action string, rawParams json.RawMessage, echo interface{}) *ActionResult {
	h, ok := registry[action]
	if !ok {
		return failResult(RetCodeNotFound, "未知action: "+action, echo)
	}

	ctx := &ActionContext{
		Action:    action,
		RawParams: rawParams,
		Echo:      echo,
		Server:    s,
	}
	if len(rawParams) > 0 {
		var m map[string]interface{}
		if json.Unmarshal(rawParams, &m) == nil {
			ctx.Params = m
		}
	}

	// 需要账号但未指定时的路由规则在具体 handler 中按需调用 resolveAccount，
	// 因为部分 action（如 get_version_info）无需账号。
	return h(ctx)
}

// resolveAccount 在 handler 内部解析目标账号：
//   - 参数带 account_id / self_id → 定向（须在线）
//   - 未指定 → 仅一个在线账号时自动选择；多个/零个则报错
func (s *Server) resolveAccount(ctx *ActionContext) (*browser.Account, *ActionResult) {
	id := ctx.AccountIDParam()
	acc, err := s.BM.Resolve(id)
	if err != nil {
		return nil, failResult(RetCodeNoAccount, err.Error(), ctx.Echo)
	}
	return acc, nil
}

// accountInst 快捷获取在线账号的浏览器实例。
func (s *Server) accountInst(ctx *ActionContext) (*browser.Instance, *ActionResult) {
	acc, res := s.resolveAccount(ctx)
	if res != nil {
		return nil, res
	}
	inst := acc.Instance()
	if inst == nil {
		return nil, failResult(RetCodeNoAccount, "账号浏览器未运行", ctx.Echo)
	}
	return inst, nil
}

// extractText 支持 string 或 OneBot 消息段数组，提取纯文本。
func extractText(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		var sb strings.Builder
		for _, seg := range t {
			if m, ok := seg.(map[string]interface{}); ok {
				if mt, _ := m["type"].(string); mt == "text" {
					if d, ok := m["data"].(map[string]interface{}); ok {
						if txt, _ := d["text"].(string); txt != "" {
							sb.WriteString(txt)
						}
					}
				}
			}
		}
		return sb.String()
	default:
		return ToString(v)
	}
}

var _ = strconv.Itoa // 占位保持导入
