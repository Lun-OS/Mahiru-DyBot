package onebot

// WebUI 静态文件托管（Vite 构建产物 webui/ 目录，SPA fallback 到 index.html）。

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// spaHandler 托管 dir 下的静态文件；未命中路径回退 index.html（前端路由）。
func spaHandler(dir string) http.HandlerFunc {
	fs := http.StripPrefix("/webui/", http.FileServer(http.Dir(dir)))
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/webui/")
		if rel == "" || rel == "." || rel == "/" {
			serveIndex(w, r, dir)
			return
		}
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if st, err := os.Stat(full); err != nil || st.IsDir() {
			// 不存在的资源回退到 SPA 入口（前端路由接管）
			serveIndex(w, r, dir)
			return
		}
		fs.ServeHTTP(w, r)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request, dir string) {
	http.ServeFile(w, r, filepath.Join(dir, "index.html"))
}
