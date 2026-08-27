package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func base64Std(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// 会话类型常量（对应 SDK ConversationType）。
const (
	ConversationTypeOneToOne = 1 // 私聊
	ConversationTypeGroup    = 2 // 群聊
)

// SendResult 发送结果。
type SendResult struct {
	OK                  bool   `json:"ok"`
	ServerID            string `json:"server_id"`
	ClientID            string `json:"client_id"`
	ConversationShortID string `json:"conversation_short_id"`
	Error               string `json:"error,omitempty"`
	MsgStatus           int    `json:"msg_status,omitempty"`
	MsgSendStatus       int    `json:"msg_send_status,omitempty"`
	MsgErrorCode        int    `json:"msg_error_code,omitempty"`
	MsgErrorMsg         string `json:"msg_error_msg,omitempty"`
	FlightStatus        int    `json:"flight_status,omitempty"`
	ServerCheckCode     int    `json:"server_check_code,omitempty"`
	ServerCheckMsg      string `json:"server_check_msg,omitempty"`
}

// SendMessage 向指定数字 uid 的用户发送文本消息。
func (in *Instance) SendMessage(uid, text string, timeout time.Duration) (*SendResult, error) {
	// 确保 SDK 就绪（自动恢复）
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()

	argJSON, _ := json.Marshal([]map[string]string{{"uid": uid, "text": text}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsSendMessage, string(argJSON)))
	in.mu.Unlock()
	if err != nil {
		return nil, err
	}
	var out SendResult
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析发送结果失败: %s", raw)
	}
	if !out.OK {
		log.Printf("[发送] uid=%s 失败: %s", uid, out.Error)
	} else {
		in.InvalidateConvCache()
	}
	return &out, nil
}

// SendGroupMessage 向群聊发送文本消息。
// groupId 为群会话的 shortId 或完整 id（必须是已加入的群）。
func (in *Instance) SendGroupMessage(groupId, text string, timeout time.Duration) (*SendResult, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()

	argJSON, _ := json.Marshal([]map[string]string{{"group_id": groupId, "text": text}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsSendGroupMessage, string(argJSON)))
	in.mu.Unlock()
	if err != nil {
		return nil, err
	}
	var out SendResult
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析发送结果失败: %s", raw)
	}
	if out.OK {
		in.InvalidateConvCache()
	}
	return &out, nil
}

// Conversation 会话摘要。
type Conversation struct {
	ID         string `json:"id"`
	ShortID    string `json:"short_id"`
	Type       int    `json:"type"`
	ToUID      string `json:"to_uid"`
	ToSecUID   string `json:"to_sec_uid"`
	Name       string `json:"name"`
	Nickname   string `json:"nickname"`
	RemarkName string `json:"remark_name"`
	UniqueID   string `json:"unique_id"`
	ShortIDNum string `json:"short_id_num"`
	Avatar     string `json:"avatar"`
	Unread     int    `json:"unread"`
}

// GetConversations 获取会话列表。
func (in *Instance) GetConversations() ([]Conversation, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsGetConversationList)
	if err != nil {
		return nil, err
	}
	var out struct {
		Ok    bool           `json:"ok"`
		Error string         `json:"error"`
		Items []Conversation `json:"items"`
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
		return nil, fmt.Errorf("解析会话列表失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// HistoryMessage 历史消息条目。
type HistoryMessage struct {
	Sender    string `json:"sender"`
	Type      int    `json:"type"`
	Text      string `json:"text"`
	ContentRaw string `json:"content_raw"`
	ClientID  string `json:"client_id"`
	CreatedAt int64  `json:"created_at"`
}

// GetHistoryMessages 获取与指定 uid 的历史消息（本地已加载部分）。
func (in *Instance) GetHistoryMessages(uid string, count int) ([]HistoryMessage, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()

	argJSON, _ := json.Marshal([]map[string]interface{}{{"uid": uid, "count": count}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetHistoryMessages, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var out struct {
		Ok    bool             `json:"ok"`
		Error string           `json:"error"`
		Items []HistoryMessage `json:"items"`
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
		return nil, fmt.Errorf("解析历史消息失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// IncomingMessage 浏览器推送的新消息（jsRegisterReceiver 上报结构）。
type IncomingMessage struct {
	AccountID          string `json:"account_id,omitempty"` // Go侧补充
	ConversationShortID string `json:"conversation_short_id"`
	ConversationID     string `json:"conversation_id"`
	ConversationType   int    `json:"conversation_type"` // 1=private, 2=group
	Sender             string `json:"sender"`
	SenderNickname     string `json:"sender_nickname"`
	Type               int    `json:"type"`
	MsgType            string `json:"msg_type"` // text / sticker / ...
	Text               string `json:"text"`
	Content            string `json:"content"`
	ClientID           string `json:"client_id"`
	ServerID           string `json:"server_id"`
	IsFromMe           bool   `json:"is_from_me"`
	CreatedAt          int64  `json:"created_at"`
}

// MessageEvent 事件总线上的新消息事件载荷。
type MessageEvent struct {
	AccountID string `json:"account_id"`
	Raw       string `json:"raw"`
}

// AccountStateEvent 账号生命周期状态变更事件载荷。
type AccountStateEvent struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	PrevState string `json:"prev_state"`
	State     string `json:"state"`
	Error     string `json:"error,omitempty"`
}

// ParseIncoming 解析浏览器回调的原始 JSON。
func ParseIncoming(raw string) (*IncomingMessage, error) {
	var m IncomingMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

var _ = base64Std // 预留工具函数

// ---------- 用户信息 ----------

// UserInfo 用户详细信息。
type UserInfo struct {
	UID                  string `json:"uid"`
	SecUID               string `json:"sec_uid"`
	Nickname             string `json:"nickname"`
	UniqueID             string `json:"unique_id"`
	ShortID              string `json:"short_id"`
	Signature            string `json:"signature"`
	AvatarThumb          string `json:"avatar_thumb"`
	AvatarSmall          string `json:"avatar_small"`
	FollowStatus         int    `json:"follow_status"`
	FollowerStatus       int    `json:"follower_status"`
	VerificationType     int    `json:"verification_type"`
	CustomVerify         string `json:"custom_verify"`
	EnterpriseVerifyReason string `json:"enterprise_verify_reason"`
	StoreRegion          string `json:"store_region"`
}

// GetUserInfo 获取用户详细信息。
func (in *Instance) GetUserInfo(userID string) (*UserInfo, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"user_id": userID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetUserInfo, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok    bool     `json:"ok"`
		Error string   `json:"error"`
		User  UserInfo `json:"user"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return &out.User, nil
}

// GetStrangers 获取陌生人会话列表。
func (in *Instance) GetStrangers() ([]Conversation, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsGetStrangers)
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok    bool           `json:"ok"`
		Error string         `json:"error"`
		Items []Conversation `json:"items"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析陌生人列表失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// ---------- 关注/拉黑 ----------

// FollowUser 关注用户。
func (in *Instance) FollowUser(secUID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"sec_uid": secUID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsFollowUser, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// SetUserBlockStatus 拉黑/取消拉黑用户。
func (in *Instance) SetUserBlockStatus(secUID string, block bool) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"sec_uid": secUID, "block": block}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsSetUserBlockStatus, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// UpdateUserRemarkName 修改好友备注名。
func (in *Instance) UpdateUserRemarkName(secUID, remark string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"sec_uid": secUID, "remark": remark}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsUpdateUserRemarkName, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// ---------- 消息操作 ----------

// RecallMessage 撤回消息。
func (in *Instance) RecallMessage(conversationID, serverID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID, "server_id": serverID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsRecallMessage, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// DeleteMessage 删除消息。
func (in *Instance) DeleteMessage(conversationID, serverID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID, "server_id": serverID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsDeleteMessage, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// LikeMessage 点赞消息。
func (in *Instance) LikeMessage(conversationID, serverID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID, "server_id": serverID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsLikeMessage, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// ReplyResult 回复消息结果。
type ReplyResult struct {
	OK              bool   `json:"ok"`
	ServerID        string `json:"server_id"`
	ClientID        string `json:"client_id"`
	Error           string `json:"error,omitempty"`
	ServerCheckCode int    `json:"server_check_code,omitempty"`
	ServerCheckMsg  string `json:"server_check_msg,omitempty"`
}

// ReplyMessage 回复消息。
func (in *Instance) ReplyMessage(conversationID, serverID, text string) (*ReplyResult, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID, "server_id": serverID, "text": text}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsReplyMessage, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out ReplyResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析回复结果失败: %s", raw)
	}
	return &out, nil
}

// ---------- 会话管理 ----------

// SetConversationPin 置顶/取消置顶会话。
func (in *Instance) SetConversationPin(conversationID string, pinned bool) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "pinned": pinned}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsSetConversationPin, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// SetConversationMute 免打扰/取消免打扰。
func (in *Instance) SetConversationMute(conversationID string, muted bool) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "muted": muted}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsSetConversationMute, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// DeleteConversation 删除会话。
func (in *Instance) DeleteConversation(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsDeleteConversation, string(argJSON)))
	in.mu.Unlock()
	if err != nil {
		return err
	}
	if err := checkOK(res); err == nil {
		in.InvalidateConvCache()
	}
	return checkOK(res)
}

// MarkConversationRead 标记会话已读。
func (in *Instance) MarkConversationRead(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsMarkConversationRead, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// ---------- 群聊管理 ----------

// LeaveConversation 退出群聊。
func (in *Instance) LeaveConversation(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsLeaveConversation, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// DissolveConversation 解散群聊（仅群主）。
func (in *Instance) DissolveConversation(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsDissolveConversation, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// GroupParticipant 群成员。
type GroupParticipant struct {
	UID      string `json:"uid"`
	SecUID   string `json:"sec_uid"`
	Nickname string `json:"nickname"`
	Role     int    `json:"role"`
}

// GetConversationParticipants 获取群成员列表。
func (in *Instance) GetConversationParticipants(conversationID string) ([]GroupParticipant, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetConversationParticipants, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok    bool               `json:"ok"`
		Error string             `json:"error"`
		Items []GroupParticipant `json:"items"`
		Total int                `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析群成员失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// AddParticipants 添加群成员。
func (in *Instance) AddParticipants(conversationID string, secUIDs []string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "sec_uids": secUIDs}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsAddParticipants, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// RemoveParticipants 移除群成员。
func (in *Instance) RemoveParticipants(conversationID string, secUIDs []string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "sec_uids": secUIDs}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsRemoveParticipants, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// CreateConversation 创建会话。
func (in *Instance) CreateConversation(participants []string, convType int, name string) (bool, error) {
	if err := in.EnsureSDK(); err != nil {
		return false, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"participants": participants, "type": convType, "name": name}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsCreateConversation, string(argJSON)))
	if err != nil {
		return false, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Payload interface{} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return false, fmt.Errorf("解析创建会话结果失败: %s", raw)
	}
	if !out.Ok {
		return false, fmt.Errorf("%s", out.Error)
	}
	return true, nil
}

// checkOK 通用检查 JS 返回结果 ok 字段。
func checkOK(res interface{}) error {
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return fmt.Errorf("%s", out.Error)
	}
	return nil
}

// ==================== A 类: SDK Instance 未公开方法 ====================

// GetMessagesByUser 按用户获取消息列表。
func (in *Instance) GetMessagesByUser(conversationID string) ([]HistoryMessage, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetMessagesByUser, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var out struct {
		Ok    bool             `json:"ok"`
		Error string           `json:"error"`
		Items []HistoryMessage `json:"items"`
		Total int              `json:"total"`
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
		return nil, fmt.Errorf("解析消息列表失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// GetMessagesByConversation 按会话获取消息列表。
func (in *Instance) GetMessagesByConversation(conversationID string) ([]HistoryMessage, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetMessagesByConversation, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var out struct {
		Ok    bool             `json:"ok"`
		Error string           `json:"error"`
		Items []HistoryMessage `json:"items"`
		Total int              `json:"total"`
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
		return nil, fmt.Errorf("解析消息列表失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// FetchConversation 从服务器拉取会话状态。
func (in *Instance) FetchConversation(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsFetchConversation, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// UpdateConversationReadReceipt 更新已读回执。
func (in *Instance) UpdateConversationReadReceipt(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsUpdateConversationReadReceipt, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// ReadReceipt 消息已读回执。
type ReadReceipt struct {
	FinishedParticipants []string `json:"finishedParticipants"`
	ExpectedParticipants []string `json:"expectedParticipants"`
}

// GetMessageReadReceipt 获取消息已读回执。
func (in *Instance) GetMessageReadReceipt(conversationID, serverID string) (*ReadReceipt, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID, "server_id": serverID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetMessageReadReceipt, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Receipt ReadReceipt `json:"receipt"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析已读回执失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return &out.Receipt, nil
}

// GetParticipantsReadAndMinIndex 获取群成员已读状态和最小索引。
func (in *Instance) GetParticipantsReadAndMinIndex(conversationID string) (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetParticipantsReadAndMinIndex, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// GetConversationParticipantsAsync 异步获取群成员列表。
func (in *Instance) GetConversationParticipantsAsync(conversationID string) ([]GroupParticipant, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetConversationParticipantsAsync, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok    bool               `json:"ok"`
		Error string             `json:"error"`
		Items []GroupParticipant `json:"items"`
		Total int                `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析群成员失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// GetConversationParticipantsByPage 分页获取群成员。
func (in *Instance) GetConversationParticipantsByPage(conversationID string, page, pageSize int) (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "page": page, "page_size": pageSize}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetConversationParticipantsByPage, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// ApplyJoinGroup 申请加入群聊。
func (in *Instance) ApplyJoinGroup(groupShortID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"group_short_id": groupShortID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsApplyJoinGroup, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// UpsertConversationSettingExtInfo 更新会话扩展设置。
func (in *Instance) UpsertConversationSettingExtInfo(conversationID string, extInfo map[string]interface{}) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "ext_info": extInfo}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsUpsertConversationSettingExtInfo, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// GetConversationBots 获取会话机器人列表。
func (in *Instance) GetConversationBots(conversationID string) (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetConversationBots, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Bots    interface{} `json:"bots"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Bots, nil
}

// GetStrangerConversationMessage 获取陌生人会话消息。
func (in *Instance) GetStrangerConversationMessage(conversationID string) ([]HistoryMessage, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetStrangerConversationMessage, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var out struct {
		Ok    bool             `json:"ok"`
		Error string           `json:"error"`
		Items []HistoryMessage `json:"items"`
		Total int              `json:"total"`
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
		return nil, fmt.Errorf("解析消息列表失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Items, nil
}

// DeleteStrangerConversation 删除陌生人会话。
func (in *Instance) DeleteStrangerConversation(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsDeleteStrangerConversation, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// DeleteAllStrangerConversation 删除所有陌生人会话。
func (in *Instance) DeleteAllStrangerConversation() error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsDeleteAllStrangerConversation)
	if err != nil {
		return err
	}
	return checkOK(res)
}

// MarkStrangerConversationRead 标记陌生人会话已读。
func (in *Instance) MarkStrangerConversationRead(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsMarkStrangerConversationRead, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// MarkAllStrangerConversationRead 标记所有陌生人会话已读。
func (in *Instance) MarkAllStrangerConversationRead() error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsMarkAllStrangerConversationRead)
	if err != nil {
		return err
	}
	return checkOK(res)
}

// GetStrangerPreview 获取陌生人预览。
func (in *Instance) GetStrangerPreview() (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsGetStrangerPreview)
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// BatchClearConversationRead 批量清除会话已读。
func (in *Instance) BatchClearConversationRead(conversationIDs []string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_ids": conversationIDs}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsBatchClearConversationRead, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// ClearConversationMessages 清空会话消息。
func (in *Instance) ClearConversationMessages(conversationID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsClearConversationMessages, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// AddOrUpdateLocalExts 添加/更新本地扩展。
func (in *Instance) AddOrUpdateLocalExts(conversationID string, exts map[string]interface{}) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "exts": exts}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsAddOrUpdateLocalExts, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// DeleteLocalExts 删除本地扩展。
func (in *Instance) DeleteLocalExts(conversationID string, keys []string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]interface{}{{"conversation_id": conversationID, "keys": keys}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsDeleteLocalExts, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// ReportMessageDelayTime 上报消息延迟。
func (in *Instance) ReportMessageDelayTime(serverID, logID string) error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"server_id": serverID, "log_id": logID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsReportMessageDelayTime, string(argJSON)))
	if err != nil {
		return err
	}
	return checkOK(res)
}

// DbClear 清空本地数据库。
func (in *Instance) DbClear() error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsDbClear)
	if err != nil {
		return err
	}
	return checkOK(res)
}

// ==================== B 类: Store 未公开方法 ====================

// SearchConversations 搜索会话。
func (in *Instance) SearchConversations(keyword string) (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"keyword": keyword}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsSearchConversations, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// SearchParticipants 搜索群成员。
func (in *Instance) SearchParticipants(conversationID, keyword string) (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID, "keyword": keyword}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsSearchParticipants, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// RequestRelationsData 请求好友关系数据。
func (in *Instance) RequestRelationsData() error {
	if err := in.EnsureSDK(); err != nil {
		return err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsRequestRelationsData)
	if err != nil {
		return err
	}
	return checkOK(res)
}

// GenLocalUsers 生成本地用户列表。
func (in *Instance) GenLocalUsers() (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsGenLocalUsers)
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// LoadMessages 加载消息列表。
func (in *Instance) LoadMessages(conversationID string) (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"conversation_id": conversationID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsLoadMessages, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// GetMessageByServerId 按 serverId 获取消息。
func (in *Instance) GetMessageByServerId(serverID string) (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	argJSON, _ := json.Marshal([]map[string]string{{"server_id": serverID}})
	res, err := in.page.Evaluate(fmt.Sprintf("(%s)(%s)", jsGetMessageByServerId, string(argJSON)))
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// GetAllConversation 获取所有会话（conversationListManager）。
func (in *Instance) GetAllConversation() (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	return in.GetCachedConversations()
}

// GetAllGroupConversation 获取所有群聊（conversationListManager）。
func (in *Instance) GetAllGroupConversation() (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsGetAllGroupConversation)
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}

// LoadMoreConversations 加载更多会话（conversationListManager）。
func (in *Instance) LoadMoreConversations() (interface{}, error) {
	if err := in.EnsureSDK(); err != nil {
		return nil, err
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	res, err := in.page.Evaluate(jsLoadMoreConversations)
	if err != nil {
		return nil, err
	}
	var raw string
	switch v := res.(type) {
	case string:
		raw = v
	default:
		b, _ := json.Marshal(res)
		raw = string(b)
	}
	var out struct {
		Ok      bool        `json:"ok"`
		Error   string      `json:"error"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析结果失败: %s", raw)
	}
	if !out.Ok {
		return nil, fmt.Errorf("%s", out.Error)
	}
	return out.Result, nil
}
