package onebot

// 会话管理类 action：置顶/免打扰/删除/已读。

func init() {
	Register("set_conversation_pin", actSetConversationPin)
	Register("set_conversation_mute", actSetConversationMute)
	Register("delete_conversation", actDeleteConversation)
	Register("mark_read", actMarkRead)
}

func actSetConversationPin(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		Pinned         bool        `json:"pinned"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	if convID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.SetConversationPin(convID, req.Pinned); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actSetConversationMute(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		Muted          bool        `json:"muted"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	if convID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.SetConversationMute(convID, req.Muted); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actDeleteConversation(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	if convID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.DeleteConversation(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actMarkRead(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	if convID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.MarkConversationRead(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}
