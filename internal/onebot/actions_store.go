package onebot

// B 类: Store 未公开方法 action。
//
// 可用但未封装为 OneBot action 的 SDK 方法（供参考）：
// - requestManager.request: 请求管理器（SDK 内部）
// - utilsManager.getRequestFunction: 获取请求函数（SDK 内部）
// - utilsManager.openUserHomePage: 打开用户主页（UI 操作）
// - utilsManager.downloadFile: 下载文件（SDK 内部）
// - utilsManager.openNewPage: 打开新页面（UI 操作）
// - utilsManager.openNewIframe: 打开新 iframe（UI 操作）
// - utilsManager.openAwemeModal: 打开抖音模态框（UI 操作）
// - imSdkManager.registerEvent: 注册 SDK 事件（SDK 内部）
// - imSdkManager.triggerEvent: 触发 SDK 事件（SDK 内部）
// - frontierMessageManager.registerFrontierMessageListener: 注册前沿消息监听（SDK 内部）
// - sendMessageManager.createMessageBuilder: 创建消息构建器（SDK 内部）

func init() {
	Register("search_conversations", actSearchConversations)
	Register("search_participants", actSearchParticipants)
	Register("request_relations", actRequestRelations)
	Register("gen_local_users", actGenLocalUsers)
	Register("load_messages", actLoadMessages)
	Register("get_message_by_server_id", actGetMessageByServerId)
	Register("get_all_conversations", actGetAllConversations)
	Register("get_all_group_conversations", actGetAllGroupConversations)
	Register("load_more_conversations", actLoadMoreConversations)
}

func actSearchConversations(ctx *ActionContext) *ActionResult {
	var req struct {
		Keyword interface{} `json:"keyword"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	keyword := ToString(req.Keyword)
	if keyword == "" {
		return failResult(RetCodeBadRequest, "keyword 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.SearchConversations(keyword)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actSearchParticipants(ctx *ActionContext) *ActionResult {
	var req struct {
		ConversationID interface{} `json:"conversation_id"`
		Keyword        interface{} `json:"keyword"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	convID := ToString(req.ConversationID)
	keyword := ToString(req.Keyword)
	if convID == "" || keyword == "" {
		return failResult(RetCodeBadRequest, "conversation_id 和 keyword 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.SearchParticipants(convID, keyword)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actRequestRelations(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	if err := inst.RequestRelationsData(); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actGenLocalUsers(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.GenLocalUsers()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actLoadMessages(ctx *ActionContext) *ActionResult {
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
	result, err := inst.LoadMessages(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actGetMessageByServerId(ctx *ActionContext) *ActionResult {
	var req struct {
		ServerID interface{} `json:"server_id"`
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
	result, err := inst.GetMessageByServerId(serverID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actGetAllConversations(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.GetAllConversation()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actGetAllGroupConversations(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.GetAllGroupConversation()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}

func actLoadMoreConversations(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	result, err := inst.LoadMoreConversations()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(result, ctx.Echo)
}
