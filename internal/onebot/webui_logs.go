package onebot

// 账号日志 API

import (
	"net/http"
	"strconv"
)

// handleAccountLogs GET /api/webui/accounts/{id}/logs
func (s *Server) handleAccountLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, ok := s.BM.Get(id)
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}

	// 从 SSE 日志缓冲区获取
	logs := s.SSE.GetLogs(id, limit)
	writeJSONRaw(w, map[string]interface{}{"ok": true, "logs": logs})
}
