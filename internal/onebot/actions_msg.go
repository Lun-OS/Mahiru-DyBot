package onebot

// 消息操作类 action：撤回/删除/点赞/回复。

import "strconv"

func init() {
	Register("recall_message", actRecallMessage)
	Register("delete_message", actDeleteMessage)
	Register("like_message", actLikeMessage)
	Register("reply_message", actReplyMessage)
	// OneBot v11 标准名称别名
	Register("delete_msg", actDeleteMessage)
	Register("get_msg", actGetMsg)
}

// actGetMsg 获取单条消息（OneBot v11 标准 API）
func actGetMsg(ctx *ActionContext) *ActionResult {
	var req struct {
		MessageID interface{} `json:"message_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	// message_id 可以是数字或字符串（大数字用字符串避免精度丢失）
	var serverID string
	switch v := req.MessageID.(type) {
	case string:
		serverID = v
	case float64:
		serverID = strconv.FormatInt(int64(v), 10)
	default:
		return failResult(RetCodeBadRequest, "message_id 缺失或非法", ctx.Echo)
	}
	if serverID == "" || serverID == "0" {
		return failResult(RetCodeBadRequest, "message_id 缺失或非法", ctx.Echo)
	}
	result, err := inst.GetMessageByServerId(serverID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	if result == nil {
		return failResult(RetCodeNotFound, "消息不存在", ctx.Echo)
	}
	var msgIDInt int64
	if v, err := strconv.ParseInt(serverID, 10, 64); err == nil {
		msgIDInt = v
	}
	msg := map[string]interface{}{
		"message_id": msgIDInt,
		"raw":        result,
	}
	return okResult(msg, ctx.Echo)
}

func actRecallMessage(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		ServerID       interface{} `json:"server_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	serverID := ToString(req.ServerID)
	if convID == "" || serverID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 和 server_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.RecallMessage(convID, serverID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actDeleteMessage(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		ServerID       interface{} `json:"server_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	serverID := ToString(req.ServerID)
	if convID == "" || serverID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 和 server_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.DeleteMessage(convID, serverID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actLikeMessage(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		ServerID       interface{} `json:"server_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	serverID := ToString(req.ServerID)
	if convID == "" || serverID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 和 server_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.LikeMessage(convID, serverID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actReplyMessage(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		ServerID       interface{} `json:"server_id"`
		Text           string      `json:"text"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	serverID := ToString(req.ServerID)
	if convID == "" || serverID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 和 server_id 缺失", ctx.Echo)
	}
	if req.Text == "" {
		return failResult(RetCodeBadRequest, "text 为空", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.ReplyMessage(convID, serverID, req.Text)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	data := map[string]interface{}{
		"message_id": result.ServerID,
		"client_id":  result.ClientID,
	}
	if result.ServerCheckCode != 0 {
		data["server_check_code"] = result.ServerCheckCode
		data["server_check_msg"] = result.ServerCheckMsg
	}
	return okResult(data, ctx.Echo)
}
