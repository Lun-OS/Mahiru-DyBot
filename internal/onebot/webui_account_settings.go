package onebot

// 账号设置更新 API

import (
	"encoding/json"
	"net/http"
)

// handleAccountUpdateSettings POST /api/webui/accounts/{id}/settings
func (s *Server) handleAccountUpdateSettings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	acc, ok := s.BM.Get(id)
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}

	body, _ := readBody(r)
	var update struct {
		Name           *string `json:"name"`
		ViewportWidth  *int    `json:"viewport_width"`
		ViewportHeight *int    `json:"viewport_height"`
		CustomUA       *string `json:"custom_ua"`
	}
	if err := json.Unmarshal(body, &update); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "JSON解析失败"})
		return
	}

	// 更新名字
	if update.Name != nil {
		if err := s.BM.Rename(id, *update.Name); err != nil {
			writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		acc.Meta.Name = *update.Name
	}

	// 更新 viewport / UA
	vpW, vpH := 0, 0
	cua := ""
	uaChanged := false
	vpChanged := false
	if update.ViewportWidth != nil {
		vpW = *update.ViewportWidth
		vpChanged = true
	}
	if update.ViewportHeight != nil {
		vpH = *update.ViewportHeight
		vpChanged = true
	}
	if update.CustomUA != nil {
		cua = *update.CustomUA
		uaChanged = true
	}
	if vpChanged || uaChanged {
		if err := s.BM.UpdateSettings(id, vpW, vpH, cua, vpChanged); err != nil {
			writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		if vpW > 0 {
			acc.Meta.ViewportWidth = vpW
		} else if vpChanged {
			acc.Meta.ViewportWidth = 0
		}
		if vpH > 0 {
			acc.Meta.ViewportHeight = vpH
		} else if vpChanged {
			acc.Meta.ViewportHeight = 0
		}
		if uaChanged {
			acc.Meta.CustomUA = cua
		}
	}

	writeJSONRaw(w, map[string]interface{}{"ok": true, "account": acc})
}
