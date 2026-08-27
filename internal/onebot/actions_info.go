package onebot

// 信息查询类 action。

import (
	"strconv"

	"mahiru-dybot/internal/browser"
)

func init() {
	Register("get_login_info", actGetLoginInfo)
	Register("get_friend_list", actGetFriendList)
	Register("get_group_list", actGetGroupList)
	Register("get_version_info", actGetVersionInfo)
	Register("get_status", actGetStatus)
	// OneBot v11 标准名称别名
	Register("get_status", actGetStatus)
}

const (
	appName    = "Mahiru DyBot"
	appVersion = "2.0.0"
)

func actGetVersionInfo(ctx *ActionContext) *ActionResult {
	return okResult(VersionInfo{
		AppName:         appName,
		Version:         appVersion,
		ProtocolVersion: "v11",
	}, ctx.Echo)
}

func actGetStatus(ctx *ActionContext) *ActionResult {
	online := ctx.Server.BM.OnlineCount() > 0
	total := len(ctx.Server.BM.List())
	// 获取第一个在线账号的 SDK 状态
	var sdkReady bool
	var convCount int
	var connStatus string
	for _, a := range ctx.Server.BM.List() {
		if a.State == "online" {
			if acc, ok := ctx.Server.BM.Get(a.ID); ok && acc.Instance() != nil {
				st := acc.Instance().GetSDKStatus()
				sdkReady = st.SDKReady
				convCount = st.ConversationCount
				connStatus = st.ConnectionStatus
				break
			}
		}
	}
	return okResult(StatusInfo{
		Online:            online,
		Good:              online,
		AccountsOnline:    ctx.Server.BM.OnlineCount(),
		AccountsTotal:     total,
		SDKReady:          sdkReady,
		ConnectionStatus:  connStatus,
		ConversationCount: convCount,
	}, ctx.Echo)
}

func actGetLoginInfo(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	// 获取完整登录信息
	loginCheck := inst.CheckLoginWithUser()
	sdkStatus := inst.GetSDKStatus()

	self := int64(0)
	if n, err := strconv.ParseInt(inst.SelfUID(), 10, 64); err == nil {
		self = n
	}
	nickname := inst.SelfNickname()

	// 优先使用 jsCheckLogin 返回的用户数据（更完整）
	info := LoginInfo{
		UserID:     self,
		Nickname:   nickname,
		SDKReady:   sdkStatus.SDKReady,
		ModID:      sdkStatus.ModID,
		Connection: sdkStatus.ConnectionStatus,
	}
	if loginCheck.User != nil {
		info.SecUID = loginCheck.User.SecUID
		info.UniqueID = loginCheck.User.UniqueID
		info.ShortID = loginCheck.User.ShortID
		info.Avatar = loginCheck.User.Avatar
		info.Signature = loginCheck.User.Signature
		info.Gender = loginCheck.User.Gender
		if loginCheck.User.UID != "" {
			if n, err := strconv.ParseInt(loginCheck.User.UID, 10, 64); err == nil {
				info.UserID = n
			}
		}
		if loginCheck.User.Nickname != "" {
			info.Nickname = loginCheck.User.Nickname
		}
	}
	return okResult(info, ctx.Echo)
}

func actGetFriendList(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	convs, err := inst.GetConversations()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	seen := map[string]bool{}
	items := []FriendItem{}
	for _, c := range convs {
		if c.Type == 1 && c.ToUID != "" && !seen[c.ToUID] {
			seen[c.ToUID] = true
			id, _ := strconv.ParseInt(c.ToUID, 10, 64)
			nickname := c.Nickname
			if nickname == "" {
				nickname = c.Name
			}
			items = append(items, FriendItem{
				UserID:   id,
				Nickname: nickname,
				Remark:   c.RemarkName,
				DyID:     c.UniqueID,
				ShortID:  c.ShortIDNum,
				Avatar:   c.Avatar,
			})
			ctx.Server.rememberPrivateConv(inst.ID, c.ShortID, c.ToUID)
		}
	}
	return okResult(items, ctx.Echo)
}

func actGetGroupList(ctx *ActionContext) *ActionResult {
	inst, res := ctx.Server.accountInst(ctx)
	if res != nil {
		return res
	}
	convs, err := inst.GetConversations()
	if err != nil {
		return failResult(RetCodeInternalErr, err.Error(), ctx.Echo)
	}
	items := []GroupItem{}
	for _, c := range convs {
		if c.Type == browser.ConversationTypeGroup {
			gid, _ := strconv.ParseInt(c.ShortID, 10, 64)
			items = append(items, GroupItem{GroupID: gid, GroupName: c.Name})
		}
	}
	return okResult(items, ctx.Echo)
}
