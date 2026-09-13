package onebot

// HanChat 兼容 API：/api/accounts, /api/accounts/{id}/status, /api/bot/{id}/{action}
// 这些 API 用于兼容 HanChat-QQBotManager 前端。

import (
	"encoding/json"
	"net/http"
	"strconv"

	"mahiru-dybot/internal/browser"
)

// handleHanChatAccounts GET /api/accounts
// 返回格式与 HanChat 期望的一致：{ status: "ok", retcode: 0, data: [...] }
func (s *Server) handleHanChatAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := s.BM.List()
	data := make([]map[string]interface{}, 0, len(accounts))
	for _, acc := range accounts {
		selfID := acc.UID

		isOnline := acc.State == "online"
		loginInfo := map[string]interface{}{}
		if isOnline && acc.UID != "" {
			uid, _ := strconv.ParseInt(acc.UID, 10, 64)
			loginInfo = map[string]interface{}{
				"user_id":  uid,
				"nickname": acc.Nickname,
			}
		}

		data = append(data, map[string]interface{}{
			"self_id":          selfID,
			"custom_name":      acc.Name,
			"status":           acc.State,
			"is_online":        isOnline,
			"login_info":       loginInfo,
			"last_connected_at": acc.CreatedAt,
		})
	}
	writeActionResult(w, &ActionResult{
		Status:  "ok",
		RetCode: RetCodeOK,
		Data:    data,
	})
}

// handleHanChatAccountStatus GET /api/accounts/{selfId}/status
func (s *Server) handleHanChatAccountStatus(w http.ResponseWriter, r *http.Request) {
	selfID := r.PathValue("selfId")
	acc := s.findAccountByUserID(selfID)
	if acc == nil {
		writeActionResult(w, failResult(RetCodeNotFound, "账号不存在", nil))
		return
	}
	writeActionResult(w, okResult(map[string]interface{}{
		"self_id":  selfID,
		"status":   acc.State,
		"is_online": acc.State == "online",
	}, nil))
}

// handleHanChatBotAction POST /api/bot/{selfId}/{action}
// 路由到标准 OneBot action 处理器。
func (s *Server) handleHanChatBotAction(w http.ResponseWriter, r *http.Request) {
	selfID := r.PathValue("selfId")
	action := r.PathValue("action")

	// 校验账号存在
	acc := s.findAccountByUserID(selfID)
	if acc == nil {
		writeActionResult(w, failResult(RetCodeNotFound, "账号不存在", nil))
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeActionResult(w, failResult(RetCodeBadRequest, err.Error(), nil))
		return
	}

	var params json.RawMessage
	if len(body) > 0 {
		params = json.RawMessage(body)
	}

	// 注入 self_id 到 params，让 action 路由到正确的账号
	var paramMap map[string]interface{}
	if json.Unmarshal(params, &paramMap) == nil {
		paramMap["self_id"] = selfID
		injected, _ := json.Marshal(paramMap)
		params = injected
	}

	var echo interface{}
	var probe map[string]interface{}
	if json.Unmarshal(params, &probe) == nil {
		echo = probe["echo"]
	}

	result := s.Dispatch(action, params, echo)
	if result.RetCode == RetCodeNotFound {
		w.WriteHeader(http.StatusNotFound)
	}
	writeActionResult(w, result)
}

// findAccountByUserID 根据 user_id (UID) 查找账号。
// 返回的 AccountInfo 是新分配的副本，仅用于读取字段。
func (s *Server) findAccountByUserID(userID string) *browser.AccountInfo {
	for _, a := range s.BM.List() {
		if a.UID == userID {
			acc := a // 拷贝
			return &acc
		}
	}
	return nil
}
