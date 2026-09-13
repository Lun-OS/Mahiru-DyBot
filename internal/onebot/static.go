package onebot

// WebUI 静态文件托管（SPA fallback 到 index.html）。
// 前端文件通过 Go embed 内嵌到二进制中。

import (
	"net/http"
	"path"
	"strings"
)

// spaFileHandler 返回一个 HTTP handler，用于托管内嵌的 webui 静态文件。
// 未命中路径回退到 index.html（前端路由）。
func (s *Server) spaFileHandler() http.HandlerFunc {
	fileServer := http.StripPrefix("/webui/", http.FileServer(s.WebUI))

	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/webui/")
		if rel == "" || rel == "." || rel == "/" {
			s.serveIndex(w, r)
			return
		}
		// 尝试打开文件，不存在则回退到 index.html（SPA 路由）
		f, err := s.WebUI.Open(rel)
		if err != nil {
			s.serveIndex(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	}
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	f, err := s.WebUI.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, _ := f.Stat()
	http.ServeContent(w, r, "index.html", stat.ModTime(), f)
}
