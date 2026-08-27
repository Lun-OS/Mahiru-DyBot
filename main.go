package main

import (
	"flag"
	"log"
	"path/filepath"

	"mahiru-dybot/internal/auth"
	"mahiru-dybot/internal/browser"
	"mahiru-dybot/internal/config"
	"mahiru-dybot/internal/eventbus"
	"mahiru-dybot/internal/onebot"
)

// mahiru-dybot v2: 多账号 + WebUI 管理 + OneBot v11（HTTP/正WS/反WS）。
//
// 启动流程：
//   1. 加载配置与运行时设置
//   2. 初始化 WebUI 认证、账号管理器（仅登记，不启动浏览器）
//   3. 监听 HTTP；用户通过 /webui/ 完成设密→建号→扫码→控制
func main() {
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}

	bus := eventbus.New()

	rt, err := config.LoadRuntime(filepath.Join(cfg.StorageDir, "runtime_settings.json"))
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}

	authStore, err := auth.NewStore(filepath.Join(cfg.StorageDir, "webui_auth.json"))
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
	if !authStore.IsInitialized() {
		log.Printf("[INIT] WebUI 尚未设置密码。请访问 http://127.0.0.1%s/webui/ 完成初始化（删除 storage/webui_auth.json 可重置）", cfg.ListenAddr)
	}

	am, err := browser.NewAccountManager(cfg.StorageDir, bus)
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
	am.SetListenAddr(cfg.ListenAddr)

	srv := onebot.NewServer(cfg.ListenAddr, cfg.UdpAddr, cfg.WsPath, am, authStore, rt, bus)

	log.Printf("[INIT] 就绪。浏览器将按需启动（每账号一个无头 Chrome 进程）")
	if err := srv.Start(); err != nil {
		log.Fatalf("[FATAL] HTTP 服务退出: %v", err)
	}
}
