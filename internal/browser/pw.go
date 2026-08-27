package browser

import (
	"log"
	"sync"

	"github.com/playwright-community/playwright-go"
)

var (
	pwMu   sync.Mutex
	pwInst *playwright.Playwright
	pwErr  error
)

// SharedPlaywright 返回进程级共享的 Playwright 驱动。
// 多个账号实例各自 ConnectOverCDP 到独立 Chrome，共享同一驱动进程。
func SharedPlaywright() (*playwright.Playwright, error) {
	pwMu.Lock()
	defer pwMu.Unlock()
	if pwInst != nil {
		return pwInst, nil
	}
	pwInst, pwErr = playwright.Run()
	if pwErr != nil {
		log.Printf("[ERROR] 启动 playwright 驱动失败: %v", pwErr)
		return nil, pwErr
	}
	log.Printf("[INIT] playwright 驱动已启动（进程级共享）")
	return pwInst, nil
}
