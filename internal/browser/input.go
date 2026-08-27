package browser

// 实时输入会话与画面捕获：
//   - InputSession: 基于 Playwright 鼠标/键盘 API 的增量输入注入
//   - CaptureJPEG: Playwright 原生 JPEG 截图

import (
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

// InputSession 单账号的持久输入通道。
type InputSession struct {
	inst *Instance
}

// SessionInput 获取（懒创建）输入会话。
func (in *Instance) SessionInput() *InputSession {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.inputSession == nil {
		in.inputSession = &InputSession{inst: in}
	}
	return in.inputSession
}

// CloseSessionInput 关闭并释放输入会话。
func (in *Instance) CloseSessionInput() {
	in.mu.Lock()
	s := in.inputSession
	in.inputSession = nil
	in.mu.Unlock()
	if s != nil {
		s.close()
	}
}

func (s *InputSession) close() {}

// MouseMove 移动鼠标。pressed=true 表示按住左键移动（拖动中）。
func (s *InputSession) MouseMove(x, y float64, pressed bool) error {
	in := s.inst
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	return page.Mouse().Move(x, y)
}

// MouseDown 在坐标处按下左键。
func (s *InputSession) MouseDown(x, y float64) error {
	in := s.inst
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	if err := page.Mouse().Move(x, y); err != nil {
		return err
	}
	time.Sleep(15 * time.Millisecond)
	return page.Mouse().Down()
}

// MouseUp 抬起左键。
func (s *InputSession) MouseUp(x, y float64) error {
	in := s.inst
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	return page.Mouse().Up()
}

// KeyPress 按键透传给 Playwright 键盘（名称如 Enter/Escape/a/1）。
func (s *InputSession) KeyPress(key string) error {
	in := s.inst
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.page == nil {
		return fmt.Errorf("页面未就绪")
	}
	return in.page.Keyboard().Press(key)
}

// MouseScroll 在指定坐标处滚动鼠标滚轮。deltaX/Y 正值=向下/向右。
func (s *InputSession) MouseScroll(x, y float64, deltaX, deltaY int) error {
	in := s.inst
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return fmt.Errorf("页面未就绪")
	}
	if err := page.Mouse().Move(x, y); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return page.Mouse().Wheel(float64(deltaX), float64(deltaY))
}

// CaptureJPEG 全页 JPEG 截图。返回编码数据与视口尺寸。
func (in *Instance) CaptureJPEG(quality int) ([]byte, int, int, error) {
	in.mu.Lock()
	page := in.page
	in.mu.Unlock()
	if page == nil {
		return nil, 0, 0, fmt.Errorf("页面未就绪")
	}
	opts := playwright.PageScreenshotOptions{
		Type:    playwright.ScreenshotTypeJpeg,
		Quality: playwright.Int(quality),
	}
	data, err := page.Screenshot(opts)
	if err != nil {
		return nil, 0, 0, err
	}
	w, h := in.ViewportSize()
	return data, int(w), int(h), nil
}
