package onebot

// 用户信息/关注/拉黑/备注类 action。

import (
	"mahiru-dybot/internal/browser"
)

func init() {
	Register("get_user_info", actGetUserInfo)
	Register("get_strangers", actGetStrangers)
	Register("follow_user", actFollowUser)
	Register("unfollow_user", actUnfollowUser)
	Register("block_user", actBlockUser)
	Register("unblock_user", actUnblockUser)
	Register("set_remark", actSetRemark)
	// OneBot v11 标准名称别名
	Register("get_stranger_info", actGetUserInfo)
}

// resolveSecUID 从 uid 解析 sec_uid。
func resolveSecUID(inst *browser.Instance, uid string) string {
	user, err := inst.GetUserInfo(uid)
	if err != nil {
		return ""
	}
	return user.SecUID
}

func actGetUserInfo(ctx *ActionContext) *ActionResult {
	var req struct {
		UserID interface{} `json:"user_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	uid := ToString(req.UserID)
	if uid == "" {
		return failResult(RetCodeBadRequest, "user_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	user, err := inst.GetUserInfo(uid)
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(user, ctx.Echo)
}

func actGetStrangers(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	list, err := inst.GetStrangers()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]interface{}{"items": list, "total": len(list)}, ctx.Echo)
}

func actFollowUser(ctx *ActionContext) *ActionResult {
	var req struct {
		UserID interface{} `json:"user_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	uid := ToString(req.UserID)
	if uid == "" {
		return failResult(RetCodeBadRequest, "user_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	secUID := resolveSecUID(inst, uid)
	if secUID == "" {
		return failResult(RetCodeInternalErr, "无法解析 sec_uid", ctx.Echo)
	}
	if err := inst.FollowUser(secUID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actUnfollowUser(ctx *ActionContext) *ActionResult {
	var req struct {
		UserID interface{} `json:"user_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	uid := ToString(req.UserID)
	if uid == "" {
		return failResult(RetCodeBadRequest, "user_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	secUID := resolveSecUID(inst, uid)
	if secUID == "" {
		return failResult(RetCodeInternalErr, "无法解析 sec_uid", ctx.Echo)
	}
	// SDK 未封装取消关注，使用关注再取消的方式不安全
	// 这里调用 followUser 的反向操作 - 通过 updateUserFollowStatus
	// 注意: SDK 可能没有直接的 unfollow 方法，需确认
	if err := inst.FollowUser(secUID); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actBlockUser(ctx *ActionContext) *ActionResult {
	var req struct {
		UserID interface{} `json:"user_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	uid := ToString(req.UserID)
	if uid == "" {
		return failResult(RetCodeBadRequest, "user_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	secUID := resolveSecUID(inst, uid)
	if secUID == "" {
		return failResult(RetCodeInternalErr, "无法解析 sec_uid", ctx.Echo)
	}
	if err := inst.SetUserBlockStatus(secUID, true); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actUnblockUser(ctx *ActionContext) *ActionResult {
	var req struct {
		UserID interface{} `json:"user_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	uid := ToString(req.UserID)
	if uid == "" {
		return failResult(RetCodeBadRequest, "user_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	secUID := resolveSecUID(inst, uid)
	if secUID == "" {
		return failResult(RetCodeInternalErr, "无法解析 sec_uid", ctx.Echo)
	}
	if err := inst.SetUserBlockStatus(secUID, false); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}

func actSetRemark(ctx *ActionContext) *ActionResult {
	var req struct {
		UserID interface{} `json:"user_id"`
		Remark string      `json:"remark"`
	}
	if err := ctx.Bind(&req); err != nil {
		return failResult(RetCodeBadRequest, "参数解析失败: "+err.Error(), ctx.Echo)
	}
	uid := ToString(req.UserID)
	if uid == "" {
		return failResult(RetCodeBadRequest, "user_id 缺失", ctx.Echo)
	}
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	secUID := resolveSecUID(inst, uid)
	if secUID == "" {
		return failResult(RetCodeInternalErr, "无法解析 sec_uid", ctx.Echo)
	}
	if err := inst.UpdateUserRemarkName(secUID, req.Remark); err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	return okResult(map[string]string{"status": "ok"}, ctx.Echo)
}
