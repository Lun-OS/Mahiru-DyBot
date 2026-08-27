package onebot

// 群聊管理类 action：退出/解散/成员/创建。

import (
	"mahiru-dybot/internal/browser"
	"strconv"
)

func init() {
	Register("leave_group", actLeaveGroup)
	Register("dissolve_group", actDissolveGroup)
	Register("get_group_members", actGetGroupMembers)
	Register("add_group_member", actAddGroupMember)
	Register("remove_group_member", actRemoveGroupMember)
	Register("create_group", actCreateGroup)
}

func actLeaveGroup(ctx *ActionContext) *ActionResult {
	var req struct {
		GroupID interface{} `json:"group_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	gid := ToString(req.GroupID)
	convID := resolveConvIDByGroup(inst, gid)
	if convID == "" {
		return failResult(RetCodeBadRequest, "group_id 无法解析为有效群会话", ctx.Echo)
	}
	if err := inst.LeaveConversation(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actDissolveGroup(ctx *ActionContext) *ActionResult {
	var req struct {
		GroupID interface{} `json:"group_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	gid := ToString(req.GroupID)
	convID := resolveConvIDByGroup(inst, gid)
	if convID == "" {
		return failResult(RetCodeBadRequest, "group_id 无法解析为有效群会话", ctx.Echo)
	}
	if err := inst.DissolveConversation(convID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actGetGroupMembers(ctx *ActionContext) *ActionResult {
	var req struct {
		GroupID interface{} `json:"group_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	gid := ToString(req.GroupID)
	convID := resolveConvIDByGroup(inst, gid)
	if convID == "" {
		return failResult(RetCodeBadRequest, "group_id 无法解析为有效群会话", ctx.Echo)
	}
	members, err := inst.GetConversationParticipants(convID)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]interface{}{"items": members, "total": len(members)}, ctx.Echo)
}

func actAddGroupMember(ctx *ActionContext) *ActionResult {
	var req struct {
		GroupID interface{}   `json:"group_id"`
		UserIDs []interface{} `json:"user_ids"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	gid := ToString(req.GroupID)
	convID := resolveConvIDByGroup(inst, gid)
	if convID == "" {
		return failResult(RetCodeBadRequest, "group_id 无法解析为有效群会话", ctx.Echo)
	}
	if len(req.UserIDs) == 0 {
		return failResult(RetCodeBadRequest, "user_ids 为空", ctx.Echo)
	}
	var secUIDs []string
	for _, uid := range req.UserIDs {
		uidStr := ToString(uid)
		if uidStr == "" {
			continue
		}
		secUID := resolveSecUID(inst, uidStr)
		if secUID != "" {
			secUIDs = append(secUIDs, secUID)
		}
	}
	if len(secUIDs) == 0 {
		return failResult(RetCodeInternalErr, "无法解析任何有效 sec_uid", ctx.Echo)
	}
	if err := inst.AddParticipants(convID, secUIDs); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actRemoveGroupMember(ctx *ActionContext) *ActionResult {
	var req struct {
		GroupID interface{}   `json:"group_id"`
		UserIDs []interface{} `json:"user_ids"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	gid := ToString(req.GroupID)
	convID := resolveConvIDByGroup(inst, gid)
	if convID == "" {
		return failResult(RetCodeBadRequest, "group_id 无法解析为有效群会话", ctx.Echo)
	}
	if len(req.UserIDs) == 0 {
		return failResult(RetCodeBadRequest, "user_ids 为空", ctx.Echo)
	}
	var secUIDs []string
	for _, uid := range req.UserIDs {
		uidStr := ToString(uid)
		if uidStr == "" {
			continue
		}
		secUID := resolveSecUID(inst, uidStr)
		if secUID != "" {
			secUIDs = append(secUIDs, secUID)
		}
	}
	if len(secUIDs) == 0 {
		return failResult(RetCodeInternalErr, "无法解析任何有效 sec_uid", ctx.Echo)
	}
	if err := inst.RemoveParticipants(convID, secUIDs); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actCreateGroup(ctx *ActionContext) *ActionResult {
	var req struct {
		UserIDs []interface{} `json:"user_ids"`
		Name    string        `json:"name"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	if len(req.UserIDs) < 1 {
		return failResult(RetCodeBadRequest, "user_ids 至少需要1个用户", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	var secUIDs []string
	for _, uid := range req.UserIDs {
		uidStr := ToString(uid)
		if uidStr == "" {
			continue
		}
		secUID := resolveSecUID(inst, uidStr)
		if secUID != "" {
			secUIDs = append(secUIDs, secUID)
		}
	}
	if len(secUIDs) == 0 {
		return failResult(RetCodeInternalErr, "无法解析任何有效 sec_uid", ctx.Echo)
	}
	ok, err := inst.CreateConversation(secUIDs, browser.ConversationTypeGroup, req.Name)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	if !ok {
		return failResult(RetCodeInternalErr, "创建群聊失败", ctx.Echo)
	}
	convs, err := inst.GetConversations()
	if err == nil {
		for _, c := range convs {
			if c.Type == browser.ConversationTypeGroup {
				gid, _ := strconv.ParseInt(c.ShortID, 10, 64)
				if gid > 0 {
					return okResult(map[string]interface{}{"group_id": gid, "conversation_id": c.ShortID}, ctx.Echo)
				}
			}
		}
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

// resolveConvIDByGroup 从 group_id 解析群会话 shortId。
func resolveConvIDByGroup(inst *browser.Instance, gid string) string {
	if gid == "" {
		return ""
	}
	convs, err := inst.GetConversations()
	if err != nil {
		return ""
	}
	for _, c := range convs {
		if c.Type == browser.ConversationTypeGroup && (c.ShortID == gid || c.ID == gid) {
			return c.ShortID
		}
	}
	return ""
}
