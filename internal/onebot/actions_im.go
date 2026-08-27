package onebot

// 消息收发类 action。

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"mahiru-dybot/internal/browser"
)

func init() {
	Register("send_private_msg", actSendPrivateMsg)
	Register("send_group_msg", actSendGroupMsg)
	Register("send_msg", actSendMsg)
	Register("get_history_msg", actGetHistoryMsg)
	Register("get_friend_msg_history", actGetHistoryMsg)
	Register("get_group_msg_history", actGetGroupHistoryMsg)
}

// resolveUserID 尝试将 user_id（uid 或 dy_id/short_id）解析为 uid 字符串。
func resolveUserID(inst *browser.Instance, raw interface{}) string {
	s := ToString(raw)
	if s == "" {
		return ""
	}
	// 纯数字 → 直接当 uid
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return s
	}
	// 非数字 → 按 dy_id / short_id 查好友列表
	convs, err := inst.GetConversations()
	if err != nil {
		return ""
	}
	for _, c := range convs {
		if strings.EqualFold(c.UniqueID, s) || strings.EqualFold(c.ShortIDNum, s) {
			return c.ToUID
		}
	}
	return ""
}

func actSendPrivateMsg(ctx *ActionContext) *ActionResult {
	var req SendMsgRequest
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	text := extractText(req.Message)
	if text == "" {
		return failResult(RetCodeBadRequest, "message 为空", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	uidStr := resolveUserID(inst, req.UserID)
	if uidStr == "" {
		return failResult(RetCodeBadRequest, "user_id 无法解析为有效 uid", ctx.Echo)
	}
	r, err := inst.SendMessage(uidStr, text, 25*time.Second)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	if !r.OK {
		errMsg := r.Error
		if r.ServerCheckMsg != "" {
			errMsg = r.ServerCheckMsg
		} else if r.MsgErrorMsg != "" {
			errMsg = r.MsgErrorMsg
		}
		return failResult(RetCodeInternalErr, "发送失败: "+errMsg, ctx.Echo)
	}
	ctx.Server.rememberPrivateConv(inst.ID, r.ConversationShortID, uidStr)
	result := map[string]interface{}{
		"message_id":      r.ClientID,
		"server_id":       r.ServerID,
		"conversation_id": r.ConversationShortID,
	}
	if r.MsgErrorCode != 0 {
		result["msg_error_code"] = r.MsgErrorCode
		result["msg_error_msg"] = r.MsgErrorMsg
	}
	if r.ServerCheckCode != 0 {
		result["server_check_code"] = r.ServerCheckCode
		result["server_check_msg"] = r.ServerCheckMsg
	}
	return okResult(result, ctx.Echo)
}

func actSendGroupMsg(ctx *ActionContext) *ActionResult {
	var req SendMsgRequest
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	gid, ok := ToInt64(req.GroupID)
	if !ok || gid == 0 {
		return failResult(RetCodeBadRequest, "group_id 缺失或非法", ctx.Echo)
	}
	text := extractText(req.Message)
	if text == "" {
		return failResult(RetCodeBadRequest, "message 为空", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	r, err := inst.SendGroupMessage(strconv.FormatInt(gid, 10), text, 25*time.Second)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	if !r.OK {
		return failResult(RetCodeInternalErr, "发送失败: "+r.Error, ctx.Echo)
	}
	result := map[string]interface{}{
		"message_id":      r.ClientID,
		"server_id":       r.ServerID,
		"conversation_id": r.ConversationShortID,
		"flight_status":   r.FlightStatus,
	}
	if r.MsgErrorCode != 0 {
		result["msg_error_code"] = r.MsgErrorCode
		result["msg_error_msg"] = r.MsgErrorMsg
	}
	if r.ServerCheckCode != 0 {
		result["server_check_code"] = r.ServerCheckCode
		result["server_check_msg"] = r.ServerCheckMsg
	}
	return okResult(result, ctx.Echo)
}

// actSendMsg 按 message_type 路由到私聊/群聊。
func actSendMsg(ctx *ActionContext) *ActionResult {
	var req SendMsgRequest
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	mt := req.MessageType
	if mt == "" && req.GroupID != nil {
		mt = "group"
	} else if mt == "" {
		mt = "private"
	}
	sub := &ActionContext{
		Action:    "send_" + mt + "_msg",
		RawParams: mustJSON(req),
		Echo:      ctx.Echo,
		Server:    ctx.Server,
	}
	h := registry[sub.Action]
	if h == nil {
		return failResult(RetCodeBadRequest, "message_type 非法: "+mt, ctx.Echo)
	}
	return h(sub)
}

func actGetHistoryMsg(ctx *ActionContext) *ActionResult {
	var req HistoryMsgRequest
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	uid, ok := ToInt64(req.UserID)
	if !ok || uid == 0 {
		return failResult(RetCodeBadRequest, "user_id 缺失或非法", ctx.Echo)
	}
	count := req.Count
	if count <= 0 {
		count = req.Limit
	}
	if count <= 0 {
		count = 20
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	selfUID, _ := strconv.ParseInt(inst.SelfUID(), 10, 64)
	convID := strconv.FormatInt(uid, 10)
	_ = inst.FetchConversation(convID)
	items, err := inst.GetHistoryMessages(convID, count)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	if req.ReverseOrder {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
	msgs := make([]map[string]interface{}, 0, len(items))
	selfUIDStr := inst.SelfUID()
	selfNickname := inst.SelfNickname()
	for i, it := range items {
		msgID := hashStringID(it.ClientID)
		var textContent string
		if it.Text != "" {
			textContent = it.Text
		} else {
			textContent = it.ContentRaw
		}
		isFromMe := it.Sender == selfUIDStr
		senderUID, _ := strconv.ParseInt(it.Sender, 10, 64)
		var nickname string
		if isFromMe {
			nickname = selfNickname
		} else {
			nickname = inst.GetUserNickname(it.Sender)
		}
		postType := "message"
		if isFromMe {
			postType = "message_sent"
		}
		msgs = append(msgs, map[string]interface{}{
			"self_id":        selfUID,
			"user_id":        uid,
			"time":           it.CreatedAt,
			"message_id":     msgID,
			"message_seq":    i + 1,
			"message_type":   "private",
			"sender":         map[string]interface{}{"user_id": senderUID, "nickname": nickname},
			"raw_message":    textContent,
			"font":           14,
			"sub_type":       "friend",
			"message":        []interface{}{map[string]interface{}{"type": "text", "data": map[string]interface{}{"text": textContent}}},
			"message_format": "array",
			"post_type":      postType,
		})
	}
	return okResult(map[string]interface{}{"messages": msgs}, ctx.Echo)
}

func actGetGroupHistoryMsg(ctx *ActionContext) *ActionResult {
	var req HistoryMsgRequest
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	gid, ok := ToInt64(req.GroupID)
	if !ok || gid == 0 {
		return failResult(RetCodeBadRequest, "group_id 缺失或非法", ctx.Echo)
	}
	count := req.Count
	if count <= 0 {
		count = req.Limit
	}
	if count <= 0 {
		count = 20
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	selfUID, _ := strconv.ParseInt(inst.SelfUID(), 10, 64)
	selfUIDStr := inst.SelfUID()
	selfNickname := inst.SelfNickname()
	convID := strconv.FormatInt(gid, 10)
	_ = inst.FetchConversation(convID)
	items, err := inst.GetHistoryMessages(convID, count)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	if req.ReverseOrder {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
	msgs := make([]map[string]interface{}, 0, len(items))
	for i, it := range items {
		msgID := hashStringID(it.ClientID)
		var textContent string
		if it.Text != "" {
			textContent = it.Text
		} else {
			textContent = it.ContentRaw
		}
		isFromMe := it.Sender == selfUIDStr
		senderUID, _ := strconv.ParseInt(it.Sender, 10, 64)
		var nickname string
		if isFromMe {
			nickname = selfNickname
		} else {
			nickname = inst.GetUserNickname(it.Sender)
		}
		role := "member"
		if isFromMe {
			role = "owner"
		}
		postType := "message"
		if isFromMe {
			postType = "message_sent"
		}
		msgs = append(msgs, map[string]interface{}{
			"self_id":        selfUID,
			"group_id":       gid,
			"time":           it.CreatedAt,
			"message_id":     msgID,
			"message_seq":    i + 1,
			"message_type":   "group",
			"sender":         map[string]interface{}{"user_id": senderUID, "nickname": nickname, "card": "", "role": role},
			"raw_message":    textContent,
			"font":           14,
			"sub_type":       "normal",
			"message":        []interface{}{map[string]interface{}{"type": "text", "data": map[string]interface{}{"text": textContent}}},
			"message_format": "array",
			"post_type":      postType,
		})
	}
	return okResult(map[string]interface{}{"messages": msgs}, ctx.Echo)
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
