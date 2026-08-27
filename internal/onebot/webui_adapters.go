package onebot

// 每个账号独立的连接适配器 API

import (
	"encoding/json"
	"net/http"
)

// getAccountAdapters 获取账号的适配器管理器
func (s *Server) getAccountAdapters(accountID string) (*AccountAdapters, bool) {
	dir, ok := s.BM.InstanceDir(accountID)
	if !ok {
		return nil, false
	}
	return NewAccountAdapters(dir), true
}

// reloadAdapters 热重载账号的适配器连接
func (s *Server) reloadAdapters(accountID string) {
	dir, ok := s.BM.InstanceDir(accountID)
	if !ok {
		return
	}
	s.ReloadAccountAdapters(accountID, dir)
}

// handleAccountAdaptersList GET /api/webui/accounts/{id}/adapters
func (s *Server) handleAccountAdaptersList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store, ok := s.getAccountAdapters(id)
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "adapters": store.List()})
}

// handleAccountAdapterCreate POST /api/webui/accounts/{id}/adapters
func (s *Server) handleAccountAdapterCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store, ok := s.getAccountAdapters(id)
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}
	body, _ := readBody(r)
	var adapter Adapter
	if err := json.Unmarshal(body, &adapter); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "JSON解析失败"})
		return
	}
	if adapter.Type == "" || adapter.Name == "" {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "type 和 name 不能为空"})
		return
	}
	if err := store.Create(&adapter); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	s.reloadAdapters(id)
	writeJSONRaw(w, map[string]interface{}{"ok": true, "adapter": adapter})
}

// handleAccountAdapterUpdate PUT /api/webui/accounts/{id}/adapters/{aid}
func (s *Server) handleAccountAdapterUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	aid := r.PathValue("aid")
	store, ok := s.getAccountAdapters(id)
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}
	body, _ := readBody(r)
	var update map[string]interface{}
	if err := json.Unmarshal(body, &update); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "JSON解析失败"})
		return
	}
	err := store.Update(aid, func(a *Adapter) {
		if v, ok := update["name"].(string); ok {
			a.Name = v
		}
		if v, ok := update["type"].(string); ok {
			a.Type = AdapterType(v)
		}
		if v, ok := update["url"].(string); ok {
			a.URL = v
		}
		if v, ok := update["token"].(string); ok {
			a.Token = v
		}
		if v, ok := update["port"].(float64); ok {
			a.Port = int(v)
		}
		if v, ok := update["enabled"].(bool); ok {
			a.Enabled = v
		}
	})
	if err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	a, _ := store.Get(aid)
	s.reloadAdapters(id)
	writeJSONRaw(w, map[string]interface{}{"ok": true, "adapter": a})
}

// handleAccountAdapterDelete DELETE /api/webui/accounts/{id}/adapters/{aid}
func (s *Server) handleAccountAdapterDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	aid := r.PathValue("aid")
	store, ok := s.getAccountAdapters(id)
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": "账号不存在"})
		return
	}
	if err := store.Delete(aid); err != nil {
		writeJSONRaw(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	s.reloadAdapters(id)
	writeJSONRaw(w, map[string]interface{}{"ok": true})
}

// handleAccountAdaptersStatus GET /api/webui/accounts/{id}/adapters/status
func (s *Server) handleAccountAdaptersStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	mgr, ok := s.adapterMgrs[id]
	s.mu.RUnlock()
	if !ok {
		writeJSONRaw(w, map[string]interface{}{"ok": true, "statuses": []map[string]interface{}{}})
		return
	}
	writeJSONRaw(w, map[string]interface{}{"ok": true, "statuses": mgr.Statuses()})
}
