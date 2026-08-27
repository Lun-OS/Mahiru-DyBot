package onebot

// A 类: SDK Instance 未公开方法 action。
//
// 可用但未封装为 OneBot action 的 SDK 方法（供参考）：
// - tickerUpdate: 心跳更新（SDK 内部）
// - clientAck: 客户端确认（SDK 内部）
// - processInitMessage: 处理初始化消息（SDK 内部）
// - processInitConversation: 处理初始化会话（SDK 内部）
// - processPushMessage: 处理推送消息（SDK 内部）

func init() {
	Register("get_messages_by_user", actGetMessagesByUser)
	Register("get_messages_by_conversation", actGetMessagesByConversation)
	Register("fetch_conversation", actFetchConversation)
	Register("update_read_receipt", actUpdateReadReceipt)
	Register("get_message_read_receipt", actGetMessageReadReceipt)
	Register("get_participants_read_index", actGetParticipantsReadIndex)
	Register("get_conversation_participants_async", actGetConversationParticipantsAsync)
	Register("get_conversation_participants_by_page", actGetConversationParticipantsByPage)
	Register("apply_join_group", actApplyJoinGroup)
	Register("upsert_conversation_ext_info", actUpsertConversationExtInfo)
	Register("get_conversation_bots", actGetConversationBots)
	Register("get_stranger_messages", actGetStrangerMessages)
	Register("delete_stranger_conversation", actDeleteStrangerConversation)
	Register("delete_all_stranger_conversations", actDeleteAllStrangerConversations)
	Register("mark_stranger_read", actMarkStrangerRead)
	Register("mark_all_stranger_read", actMarkAllStrangerRead)
	Register("get_stranger_preview", actGetStrangerPreview)
	Register("batch_clear_read", actBatchClearRead)
	Register("clear_conversation_messages", actClearConversationMessages)
	Register("add_local_exts", actAddLocalExts)
	Register("delete_local_exts", actDeleteLocalExts)
	Register("report_message_delay", actReportMessageDelay)
	Register("db_clear", actDbClear)
}

func actGetMessagesByUser(ctx *ActionContext) *ActionResult {
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
	items, err := inst.GetMessagesByUser(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]interface{}{"items": items, "total": len(items)}, ctx.Echo)
}

func actGetMessagesByConversation(ctx *ActionContext) *ActionResult {
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
	items, err := inst.GetMessagesByConversation(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]interface{}{"items": items, "total": len(items)}, ctx.Echo)
}

func actFetchConversation(ctx *ActionContext) *ActionResult {
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
	if err := inst.FetchConversation(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actUpdateReadReceipt(ctx *ActionContext) *ActionResult {
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
	if err := inst.UpdateConversationReadReceipt(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actGetMessageReadReceipt(ctx *ActionContext) *ActionResult {
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
	receipt, err := inst.GetMessageReadReceipt(convID, serverID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(receipt, ctx.Echo)
}

func actGetParticipantsReadIndex(ctx *ActionContext) *ActionResult {
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
	result, err := inst.GetParticipantsReadAndMinIndex(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actGetConversationParticipantsAsync(ctx *ActionContext) *ActionResult {
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
	members, err := inst.GetConversationParticipantsAsync(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]interface{}{"items": members, "total": len(members)}, ctx.Echo)
}

func actGetConversationParticipantsByPage(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		Page           int         `json:"page"`
		PageSize       int         `json:"page_size"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	if convID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 缺失", ctx.Echo)
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.GetConversationParticipantsByPage(convID, req.Page, req.PageSize)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actApplyJoinGroup(ctx *ActionContext) *ActionResult {
	var req struct {
		GroupShortID interface{} `json:"group_short_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	gid := ToString(req.GroupShortID)
	if gid == "" {
		return failResult(RetCodeBadRequest, "group_short_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.ApplyJoinGroup(gid); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actUpsertConversationExtInfo(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{}            `json:"conversation_id"`
		ExtInfo        map[string]interface{} `json:"ext_info"`
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
	if err := inst.UpsertConversationSettingExtInfo(convID, req.ExtInfo); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actGetConversationBots(ctx *ActionContext) *ActionResult {
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
	bots, err := inst.GetConversationBots(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(bots, ctx.Echo)
}

func actGetStrangerMessages(ctx *ActionContext) *ActionResult {
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
	items, err := inst.GetStrangerConversationMessage(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]interface{}{"items": items, "total": len(items)}, ctx.Echo)
}

func actDeleteStrangerConversation(ctx *ActionContext) *ActionResult {
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
	if err := inst.DeleteStrangerConversation(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actDeleteAllStrangerConversations(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.DeleteAllStrangerConversation(); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actMarkStrangerRead(ctx *ActionContext) *ActionResult {
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
	if err := inst.MarkStrangerConversationRead(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actMarkAllStrangerRead(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.MarkAllStrangerConversationRead(); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actGetStrangerPreview(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	preview, err := inst.GetStrangerPreview()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(preview, ctx.Echo)
}

func actBatchClearRead(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationIDs []interface{} `json:"conversation_ids"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	if len(req.ConversationIDs) == 0 {
		return failResult(RetCodeBadRequest, "conversation_ids 为空", ctx.Echo)
	}
	var ids []string
	for _, id := range req.ConversationIDs {
		ids = append(ids, ToString(id))
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.BatchClearConversationRead(ids); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actClearConversationMessages(ctx *ActionContext) *ActionResult {
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
	if err := inst.ClearConversationMessages(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actAddLocalExts(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{}            `json:"conversation_id"`
		Exts           map[string]interface{} `json:"exts"`
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
	if err := inst.AddOrUpdateLocalExts(convID, req.Exts); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actDeleteLocalExts(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{}   `json:"conversation_id"`
		Keys           []interface{} `json:"keys"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	if convID == "" {
		return failResult(RetCodeBadRequest, "conversation_id 缺失", ctx.Echo)
	}
	var keys []string
	for _, k := range req.Keys {
		keys = append(keys, ToString(k))
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.DeleteLocalExts(convID, keys); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actReportMessageDelay(ctx *ActionContext) *ActionResult {
	var req struct {
		ServerID interface{} `json:"server_id"`
		LogID    interface{} `json:"log_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	serverID := ToString(req.ServerID)
	if serverID == "" {
		return failResult(RetCodeBadRequest, "server_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.ReportMessageDelayTime(serverID, ToString(req.LogID)); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actDbClear(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.DbClear(); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}
