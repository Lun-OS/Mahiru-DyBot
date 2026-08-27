package onebot

// OneBot v11 相关类型定义（仅实现本项目用到的子集）。
// 参考: https://raw.githubusercontent.com/botuniverse/onebot-11/master/api/public.md
// 扩展：请求参数可携带 account_id / self_id 以路由到指定抖音账号。

import (
	"encoding/json"
	"strconv"
)

// ActionResult OneBot 统一响应帧（HTTP 响应体 / WS action 回帧共用）。
type ActionResult struct {
	Status  string      `json:"status"`                 // ok / failed
	RetCode int         `json:"retcode"`                // 0 成功
	Data    interface{} `json:"data,omitempty"`         // 数据
	Message string      `json:"message,omitempty"`      // 错误信息
	Wording string      `json:"wording,omitempty"`      // 错误信息(人类可读)
	Echo    interface{} `json:"echo,omitempty"`         // 回显
}

// RetCode 约定。
const (
	RetCodeOK          = 0
	RetCodeBadRequest  = 1400 // 参数缺失或非法
	RetCodeUnauth      = 1403 // 鉴权失败
	RetCodeNotFound    = 1404 // action 不存在
	RetCodeInternalErr = 1500 // 执行失败
	RetCodeNoAccount   = 1401 // 无可用账号
)

func okResult(data interface{}, echo interface{}) *ActionResult {
	return &ActionResult{Status: "ok", RetCode: RetCodeOK, Data: data, Echo: echo}
}

func failResult(code int, msg string, echo interface{}) *ActionResult {
	return &ActionResult{Status: "failed", RetCode: code, Message: msg, Wording: msg, Echo: echo}
}

// SendMsgRequest 发送消息通用请求。
type SendMsgRequest struct {
	AccountID  string      `json:"account_id,omitempty"`
	SelfID     interface{} `json:"self_id,omitempty"` // 兼容：标准字段复用为账号路由
	UserID     interface{} `json:"user_id,omitempty"`
	GroupID    interface{} `json:"group_id,omitempty"`
	MessageType string     `json:"message_type,omitempty"` // private / group
	Message    interface{} `json:"message"`
	AutoEscape bool        `json:"auto_escape,omitempty"`
}

// LoginInfo GET /get_login_info 返回。
type LoginInfo struct {
	UserID      int64  `json:"user_id"`
	Nickname    string `json:"nickname"`
	SecUID      string `json:"sec_uid,omitempty"`
	UniqueID    string `json:"unique_id,omitempty"`
	ShortID     string `json:"short_id,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	Signature   string `json:"signature,omitempty"`
	Gender      int    `json:"gender,omitempty"`
	SDKReady    bool   `json:"sdk_ready"`
	ModID       int    `json:"mod_id,omitempty"`
	Connection  string `json:"connection_status,omitempty"`
}

// FriendItem 好友列表项（由会话列表生成）。
type FriendItem struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
	DyID     string `json:"dy_id"`
	ShortID  string `json:"short_id"`
	Avatar   string `json:"avatar"`
}

// GroupItem 群列表项。
type GroupItem struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
	MemberCount int  `json:"member_count"`
}

// HistoryMsgRequest 自定义扩展接口请求。
type HistoryMsgRequest struct {
	AccountID     string      `json:"account_id,omitempty"`
	SelfID        interface{} `json:"self_id,omitempty"`
	UserID        interface{} `json:"user_id,omitempty"`
	GroupID       interface{} `json:"group_id,omitempty"`
	MessageType   string      `json:"message_type,omitempty"`
	Count         int         `json:"count,omitempty"`
	Limit         int         `json:"limit,omitempty"`
	MessageSeq    interface{} `json:"message_seq,omitempty"`
	ReverseOrder  bool        `json:"reverseOrder,omitempty"`
}

// VersionInfo get_version_info 返回。
type VersionInfo struct {
	AppName       string `json:"app_name"`
	Version       string `json:"version"`
	ProtocolVersion string `json:"protocol_version"`
}

// StatusInfo get_status 返回。
type StatusInfo struct {
	Online            bool   `json:"online"`
	Good              bool   `json:"good"`
	AccountsOnline    int    `json:"accounts_online"`
	AccountsTotal     int    `json:"accounts_total"`
	SDKReady          bool   `json:"sdk_ready,omitempty"`
	ConnectionStatus  string `json:"connection_status,omitempty"`
	ConversationCount int    `json:"conversation_count,omitempty"`
}

// EventMessage 私聊/群聊消息事件（WS 推送）。
type EventMessage struct {
	Time        int64       `json:"time"`
	SelfID      int64       `json:"self_id"`
	AccountID   string      `json:"account_id,omitempty"` // 扩展字段
	PostType    string      `json:"post_type"`            // message
	MessageType string      `json:"message_type"`         // private / group
	SubType     string      `json:"sub_type"`             // friend / normal
	UserID      int64       `json:"user_id"`
	GroupID     int64       `json:"group_id,omitempty"`
	Message     interface{} `json:"message"`
	RawMessage  string      `json:"raw_message"`
	Font        int         `json:"font"`
	Sender      EventSender `json:"sender"`
	MessageID   int64       `json:"message_id"`
}

type EventSender struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card,omitempty"`
	Role     string `json:"role,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int    `json:"age,omitempty"`
}

// EventMeta 心跳等元事件。
type EventMeta struct {
	Time          int64  `json:"time"`
	SelfID        int64  `json:"self_id"`
	PostType      string `json:"post_type"` // meta_event
	MetaEventType string `json:"meta_event_type"`
	Interval      int    `json:"interval,omitempty"`
}

// EventAccount 账号状态变更通知（扩展事件，便于调试与前端感知）。
type EventAccount struct {
	Time      int64                  `json:"time"`
	PostType  string                 `json:"post_type"` // meta_event
	MetaEventType string             `json:"meta_event_type"` // account_state
	Account   map[string]interface{} `json:"account"`
}

// WSActionFrame 正向/反向 WS 上传输的 action 请求帧。
type WSActionFrame struct {
	Action string          `json:"action"`
	Params json.RawMessage `json:"params,omitempty"`
	Echo   interface{}     `json:"echo,omitempty"`
}

// ToInt64 宽松转换 user_id 等（数字、字符串、json.Number）。
func ToInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err == nil {
			return n, true
		}
	case json.Number:
		n, err := t.Int64()
		if err == nil {
			return n, true
		}
	case int64:
		return t, true
	case int:
		return int64(t), true
	}
	return 0, false
}

// ToString 宽松转字符串。
func ToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}
