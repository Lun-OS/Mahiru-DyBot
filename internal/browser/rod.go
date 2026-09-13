package browser

// Rod 浏览器驱动
//
// Linux 依赖处理：检测缺失 .so → 查映射表 → 安装包 → 递归重测

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

var (
	rodMu               sync.Mutex
	rodPage             *rod.Page
	rodSetup            bool
	browserPathOverride string
)

func SetBrowserPath(path string) { browserPathOverride = path }

func EnsureChromium() {
	binPath := resolveBrowserPath()
	log.Printf("[INIT] Chromium 就绪: %s", binPath)
}

func LaunchBrowser(userDataDir string, vpW, vpH int) (*rod.Page, *rod.Browser, *launcher.Launcher, error) {
	rodMu.Lock()
	defer rodMu.Unlock()
	if vpW <= 0 { vpW = 1280 }
	if vpH <= 0 { vpH = 720 }

	binPath := resolveBrowserPath()
	log.Printf("[INIT] 浏览器路径: %s", binPath)

	l := launcher.New().
		Bin(binPath).UserDataDir(userDataDir).Headless(true).NoSandbox(true).
		Set("window-size", fmt.Sprintf("%d,%d", vpW, vpH)).
		Set("disable-gpu").
		Set("disable-background-timer-throttling").
		Set("disable-backgrounding-occluded-windows").
		Set("disable-renderer-backgrounding").
		Set("disable-features", strings.Join([]string{
			"TranslateUI", "Translate", "BackForwardCache", "MediaRouter",
			"OptimizationHints", "PaintHolding", "RendererCodeIntegrity", "AudioServiceSandbox",
		}, ",")).
		Set("disable-component-update").Set("disable-default-apps").
		Set("disable-extensions").Set("disable-sync").
		Set("disable-background-networking").Set("disable-breakpad").
		Set("disable-client-side-phishing-detection").Set("disable-hang-monitor").
		Set("disable-popup-blocking").Set("disable-prompt-on-repost").
		Set("disable-tab-for-desktop-share").
		Set("disable-webgl").Set("disable-webgl2").
		Set("disable-2d-canvas-image-chromium").Set("disable-accelerated-2d-canvas").
		Set("disable-accelerated-video-decode").Set("disable-quic").
		Set("disable-zero-maxognito-variable").Set("mute-audio").Set("disable-remote-fonts").
		Set("disable-blink-features", "AutomationControlled").
		Set("excludeSwitches", "enable-automation").Set("useAutomationExtension", "false").
		Set("js-flags", "--max-old-space-size=128 --opt-off-aliasing-for-arrays").
		Set("lang", "zh-CN")

	controlURL, err := l.Launch()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Rod Launch 失败: %w", err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, nil, nil, fmt.Errorf("Rod Connect 失败: %w", err)
	}
	page, err := stealth.Page(browser)
	if err != nil {
		browser.MustClose()
		return nil, nil, nil, fmt.Errorf("Stealth Page 失败: %w", err)
	}

	page.MustEvalOnNewDocument(`() => {
		Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
		Object.defineProperty(navigator, 'languages', {get: () => ['zh-CN','zh','en']});
		Object.defineProperty(navigator, 'plugins', {get: () => [1,2,3,4,5]});
		window.chrome = {runtime: {}, loadTimes: function(){}, csi: function(){}, app: {}};
		const oq = window.navigator.permissions.query;
		window.navigator.permissions.query = (p) => p.name === 'notifications' ?
			Promise.resolve({state: Notification.permission}) : oq(p);
	}`)

	if err = (proto.NetworkSetBlockedURLs{Urls: []string{
		"*.woff", "*.woff2", "*.ttf", "*.otf", "*.eot",
		"*.mp4", "*.webm", "*.ogg", "*.mp3", "*.wav", "*.svg",
	}}).Call(page); err != nil {
		log.Printf("[WARN] 资源阻断设置失败: %v", err)
	}

	rodPage = page
	rodSetup = true
	log.Printf("[INIT] Rod 浏览器已启动: %dx%d, GPU=OFF, WebGL=OFF, 字体/媒体=阻断, 反检测=ON", vpW, vpH)
	return page, browser, l, nil
}

func ConnectChrome(cdpURL string) (*rod.Page, *rod.Browser, error) {
	browser := rod.New().ControlURL(cdpURL)
	if err := browser.Connect(); err != nil { return nil, nil, err }
	page, err := stealth.Page(browser)
	if err != nil { browser.MustClose(); return nil, nil, err }
	rodPage = page
	return page, browser, nil
}

func SetGlobalPage(p *rod.Page)   { rodMu.Lock(); defer rodMu.Unlock(); rodPage = p }
func ClearGlobalPage()             { rodMu.Lock(); defer rodMu.Unlock(); rodPage = nil; rodSetup = false }
func CloseRod()                    { rodMu.Lock(); defer rodMu.Unlock(); rodPage = nil; rodSetup = false }

// ==================== 浏览器路径 ====================

func resolveBrowserPath() string {
	if browserPathOverride != "" {
		if _, err := os.Stat(browserPathOverride); err != nil {
			log.Fatalf("[FATAL] 指定的浏览器路径不存在: %s", browserPathOverride)
		}
		return browserPathOverride
	}

	execDir := getExecDir()
	browserRoot := filepath.Join(execDir, "browser")

	if bin := findChromiumInDir(browserRoot); bin != "" {
		if runtime.GOOS == "windows" || testChromiumLaunch(bin) {
			return bin
		}
		log.Printf("[WARN] 已有 Chromium 无法运行，重新下载...")
		os.RemoveAll(browserRoot)
	}

	return downloadChromium(browserRoot)
}

func downloadChromium(browserRoot string) string {
	log.Printf("[INIT] Chromium 未找到，自动下载到: %s", browserRoot)
	b := launcher.NewBrowser()
	b.RootDir = browserRoot
	binPath, err := b.Get()
	if err != nil {
		log.Fatalf("[FATAL] Chromium 下载失败: %v", err)
	}
	log.Printf("[INIT] Chromium 下载完成: %s", binPath)

	if runtime.GOOS == "windows" {
		return binPath
	}

	ensureLinuxDeps(binPath)
	return binPath
}

// ==================== Linux 依赖（动态检测） ====================

func ensureLinuxDeps(binPath string) {
	const maxRounds = 5
	for round := 0; round < maxRounds; round++ {
		if testChromiumLaunch(binPath) {
			installChineseFonts()
			return
		}
		libs := detectMissingLibs(binPath)
		if len(libs) == 0 {
			log.Fatalf("[FATAL] Chromium 启动失败，无法检测缺失库")
		}
		log.Printf("[INIT] 第%d轮: 检测到缺失库 %v", round+1, libs)

		pkgs := resolvePkgs(libs)
		if len(pkgs) == 0 {
			log.Fatalf("[FATAL] 无法将缺失库映射到系统包: %v", libs)
		}

		if !installPkgs(pkgs) && !installPkgsSudo(pkgs) {
			log.Fatalf("[FATAL] 安装依赖失败，请以 root 权限执行:\n  apt-get install -y %s",
				strings.Join(pkgs, " "))
		}
	}
	log.Fatalf("[FATAL] Chromium 依赖安装 %d 轮后仍无法运行", maxRounds)
}

func testChromiumLaunch(binPath string) bool {
	cmd := exec.Command(binPath, "--headless", "--no-sandbox", "--disable-gpu", "--dump-dom", "about:blank")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[WARN] Chromium 启动测试失败: %v\n%s", err, string(output))
		return false
	}
	return true
}

func detectMissingLibs(binPath string) []string {
	cmd := exec.Command(binPath, "--headless", "--no-sandbox", "--disable-gpu", "--dump-dom", "about:blank")
	output, _ := cmd.CombinedOutput()
	out := string(output)

	var libs []string
	seen := make(map[string]bool)
	for {
		idx := strings.Index(out, "error while loading shared libraries: ")
		if idx == -1 { break }
		out = out[idx+len("error while loading shared libraries: "):]
		colonIdx := strings.Index(out, ":")
		if colonIdx == -1 { break }
		lib := strings.TrimSpace(out[:colonIdx])
		out = out[colonIdx+1:]
		if !seen[lib] {
			seen[lib] = true
			libs = append(libs, lib)
		}
	}
	return libs
}

// ==================== 包名映射 ====================

func resolvePkgs(libs []string) []string {
	useRPM := commandExists("yum") || commandExists("dnf")

	pkgSet := make(map[string]bool)
	for _, lib := range libs {
		var pkg string
		if useRPM {
			pkg = libToRPM[lib]
		} else {
			pkg = libToDEB[lib]
		}
		if pkg != "" {
			pkgSet[pkg] = true
		} else {
			// 尝试去掉版本号后缀匹配
			base := lib
			if idx := strings.LastIndex(lib, ".so"); idx > 0 {
				base = lib[:idx+3]
			}
			if useRPM {
				pkg = libToRPM[base]
			} else {
				pkg = libToDEB[base]
			}
			if pkg != "" {
				pkgSet[pkg] = true
			}
		}
	}

	pkgs := make([]string, 0, len(pkgSet))
	for pkg := range pkgSet {
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

var libToDEB = map[string]string{
	"libcairo.so.2":         "libcairo2",
	"libcairo.so":           "libcairo2",
	"libatk-1.0.so.0":       "libatk1.0-0",
	"libatk-1.0.so":         "libatk1.0-0",
	"libatk-bridge2.0.so.0": "libatk-bridge2.0-0",
	"libatk-bridge2.0.so":   "libatk-bridge2.0-0",
	"libcups.so.2":           "libcups2",
	"libcups.so":             "libcups2",
	"libdrm.so.2":            "libdrm2",
	"libdrm.so":              "libdrm2",
	"libxkbcommon.so.0":      "libxkbcommon0",
	"libxkbcommon.so":        "libxkbcommon0",
	"libxcomposite.so.1":     "libxcomposite1",
	"libxcomposite.so":       "libxcomposite1",
	"libxdamage.so.1":        "libxdamage1",
	"libxdamage.so":          "libxdamage1",
	"libxrandr.so.2":         "libxrandr2",
	"libxrandr.so":           "libxrandr2",
	"libgbm.so.1":            "libgbm1",
	"libgbm.so":              "libgbm1",
	"libnss.so.3":            "libnss3",
	"libnss.so":              "libnss3",
	"libasound.so.2":         "libasound2",
	"libasound.so":           "libasound2",
	"libXss.so.1":            "libxss1",
	"libXss.so":              "libxss1",
	"libXtst.so.6":           "libxtst6",
	"libXtst.so":             "libxtst6",
	"libpango-1.0.so.0":      "libpango-1.0-0",
	"libpango-1.0.so":        "libpango-1.0-0",
	"libglib-2.0.so.0":       "libglib2.0-0",
	"libglib-2.0.so":         "libglib2.0-0",
	"libgobject-2.0.so.0":    "libglib2.0-0",
	"libgobject-2.0.so":      "libglib2.0-0",
	"libgio-2.0.so.0":        "libglib2.0-0",
	"libgio-2.0.so":          "libglib2.0-0",
	"libdbus-glib-1.so.2":    "libdbus-glib-1-2",
	"libdbus-glib-1.so":      "libdbus-glib-1-2",
	"libXi.so.6":             "libxi6",
	"libXi.so":               "libxi6",
	"libXext.so.6":           "libxext6",
	"libXext.so":             "libxext6",
	"libX11.so.6":            "libx11-6",
	"libX11.so":              "libx11-6",
	"libXfixes.so.3":         "libxfixes3",
	"libXfixes.so":           "libxfixes3",
	"libXrender.so.1":        "libxrender1",
	"libXrender.so":          "libxrender1",
	"libfontconfig.so.1":     "libfontconfig1",
	"libfontconfig.so":       "libfontconfig1",
	"libfreetype.so.6":       "libfreetype6",
	"libfreetype.so":         "libfreetype6",
	"libpixman-1.so.0":       "libpixman-1-0",
	"libpixman-1.so":         "libpixman-1-0",
	"libpng16.so.16":         "libpng16-16",
	"libpng16.so":            "libpng16-16",
	"libjpeg.so.62":          "libjpeg-turbo8",
	"libjpeg.so.8":           "libjpeg-turbo8",
	"libjpeg.so":             "libjpeg-turbo8",
	"libpangocairo-1.0.so.0": "libpangocairo-1.0-0",
	"libpangocairo-1.0.so":   "libpangocairo-1.0-0",
	"libpangoft2-1.0.so.0":   "libpangoft2-1.0-0",
	"libpangoft2-1.0.so":     "libpangoft2-1.0-0",
	"libffi.so.8":             "libffi8",
	"libffi.so.7":             "libffi7",
	"libffi.so":               "libffi8",
	"libxcb.so.1":             "libxcb1",
	"libxcb.so":               "libxcb1",
	"libstdc++.so.6":          "libstdc++6",
	"libgcc_s.so.1":           "libgcc-s1",
	"libatomic.so.1":          "libatomic1",
	"libgtk-3.so.0":           "libgtk3-0",
	"libgtk-3.so":             "libgtk3-0",
	"libgdk-3.so.0":           "libgtk3-0",
	"libgdk-3.so":             "libgtk3-0",
	"libgdk_pixbuf-2.0.so.0":  "libgdk-pixbuf-2.0-0",
	"libgdk_pixbuf-2.0.so":    "libgdk-pixbuf-2.0-0",
	"libxcb-shm.so.0":         "libxcb-shm0",
	"libxcb-shm.so":           "libxcb-shm0",
	"libxcb-render.so.0":      "libxcb-render0",
	"libxcb-render.so":        "libxcb-render0",
	"libX11-xcb.so.1":         "libx11-xcb1",
	"libX11-xcb.so":           "libx11-xcb1",
	"libexpat.so.1":           "libexpat1",
	"libexpat.so":             "libexpat1",
}

var libToRPM = map[string]string{
	"libcairo.so.2":         "cairo",
	"libcairo.so":           "cairo",
	"libatk-1.0.so.0":       "atk",
	"libatk-1.0.so":         "atk",
	"libatk-bridge2.0.so.0": "at-spi2-atk",
	"libatk-bridge2.0.so":   "at-spi2-atk",
	"libcups.so.2":           "cups-libs",
	"libcups.so":             "cups-libs",
	"libdrm.so.2":            "libdrm",
	"libdrm.so":              "libdrm",
	"libxkbcommon.so.0":      "libxkbcommon",
	"libxkbcommon.so":        "libxkbcommon",
	"libxcomposite.so.1":     "libXcomposite",
	"libxcomposite.so":       "libXcomposite",
	"libxdamage.so.1":        "libXdamage",
	"libxdamage.so":          "libXdamage",
	"libxrandr.so.2":         "libXrandr",
	"libxrandr.so":           "libXrandr",
	"libgbm.so.1":            "mesa-libgbm",
	"libgbm.so":              "mesa-libgbm",
	"libnss.so.3":            "nss",
	"libnss.so":              "nss",
	"libasound.so.2":         "alsa-lib",
	"libasound.so":           "alsa-lib",
	"libXss.so.1":            "libXScrnSaver",
	"libXss.so":              "libXScrnSaver",
	"libXtst.so.6":           "libXtst",
	"libXtst.so":             "libXtst",
	"libpango-1.0.so.0":      "pango",
	"libpango-1.0.so":        "pango",
	"libglib-2.0.so.0":       "glib2",
	"libglib-2.0.so":         "glib2",
	"libgobject-2.0.so.0":    "glib2",
	"libgobject-2.0.so":      "glib2",
	"libgio-2.0.so.0":        "glib2",
	"libgio-2.0.so":          "glib2",
	"libdbus-glib-1.so.2":    "dbus-glib",
	"libdbus-glib-1.so":      "dbus-glib",
	"libXi.so.6":             "libXi",
	"libXi.so":               "libXi",
	"libXext.so.6":           "libXext",
	"libXext.so":             "libXext",
	"libX11.so.6":            "libX11",
	"libX11.so":              "libX11",
	"libXfixes.so.3":         "libXfixes",
	"libXfixes.so":           "libXfixes",
	"libXrender.so.1":        "libXrender",
	"libXrender.so":          "libXrender",
	"libfontconfig.so.1":     "fontconfig",
	"libfontconfig.so":       "fontconfig",
	"libfreetype.so.6":       "freetype",
	"libfreetype.so":         "freetype",
	"libpixman-1.so.0":       "pixman",
	"libpixman-1.so":         "pixman",
	"libpng16.so.16":         "libpng",
	"libpng16.so":            "libpng",
	"libjpeg.so.62":          "libjpeg-turbo",
	"libjpeg.so.8":           "libjpeg-turbo",
	"libjpeg.so":             "libjpeg-turbo",
	"libpangocairo-1.0.so.0": "pango",
	"libpangocairo-1.0.so":   "pango",
	"libpangoft2-1.0.so.0":   "pango",
	"libpangoft2-1.0.so":     "pango",
	"libffi.so.8":             "libffi",
	"libffi.so.7":             "libffi",
	"libffi.so":               "libffi",
	"libxcb.so.1":             "libxcb",
	"libxcb.so":               "libxcb",
	"libstdc++.so.6":          "libstdc++",
	"libgcc_s.so.1":           "libgcc",
	"libatomic.so.1":          "libatomic",
	"libgtk-3.so.0":           "gtk3",
	"libgtk-3.so":             "gtk3",
	"libgdk-3.so.0":           "gtk3",
	"libgdk-3.so":             "gtk3",
	"libgdk_pixbuf-2.0.so.0":  "gdk-pixbuf2",
	"libgdk_pixbuf-2.0.so":    "gdk-pixbuf2",
	"libxcb-shm.so.0":         "libxcb",
	"libxcb-shm.so":           "libxcb",
	"libxcb-render.so.0":      "libxcb",
	"libxcb-render.so":        "libxcb",
	"libX11-xcb.so.1":         "libX11",
	"libX11-xcb.so":           "libX11",
	"libexpat.so.1":           "expat",
	"libexpat.so":             "expat",
}

// ==================== 包安装 ====================

func installPkgs(pkgs []string) bool {
	if len(pkgs) == 0 { return true }
	if _, err := exec.LookPath("apt-get"); err == nil {
		return runCmd("apt-get", append([]string{"install", "-y"}, pkgs...)...)
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return runCmd("yum", append([]string{"install", "-y"}, pkgs...)...)
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return runCmd("dnf", append([]string{"install", "-y"}, pkgs...)...)
	}
	if _, err := exec.LookPath("apk"); err == nil {
		return runCmd("apk", append([]string{"add", "--no-cache"}, pkgs...)...)
	}
	return false
}

func installPkgsSudo(pkgs []string) bool {
	sudoPath, err := exec.LookPath("sudo")
	if err != nil { return false }
	if _, err := exec.LookPath("apt-get"); err == nil {
		runCmd(sudoPath, "apt-get", "update", "-qq")
		return runCmd(sudoPath, append([]string{"apt-get", "install", "-y"}, pkgs...)...)
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return runCmd(sudoPath, append([]string{"yum", "install", "-y"}, pkgs...)...)
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return runCmd(sudoPath, append([]string{"dnf", "install", "-y"}, pkgs...)...)
	}
	return false
}

// ==================== Playwright 捆绑版 ====================

func downloadPlaywrightChromium(browserRoot string) string {
	if runtime.GOARCH == "arm64" { return "" }
	b := launcher.NewBrowser()
	b.RootDir = browserRoot
	b.Hosts = []launcher.Host{launcher.HostPlaywright}
	b.Revision = launcher.RevisionPlaywright
	binPath, err := b.Get()
	if err != nil { return "" }
	return binPath
}

// ==================== 中文字体 ====================

func installChineseFonts() {
	// 检查是否已有中文字体
	cmd := exec.Command("fc-list", ":lang=zh")
	if output, err := cmd.CombinedOutput(); err == nil && len(output) > 10 {
		return // 已有中文字体
	}

	log.Printf("[INIT] 安装中文字体...")
	if _, err := exec.LookPath("apt-get"); err == nil {
		runCmd("apt-get", "install", "-y", "fonts-noto-cjk", "fonts-wqy-zenhei", "fonts-wqy-microhei")
		runCmd("fc-cache", "-fv")
		return
	}
	if _, err := exec.LookPath("yum"); err == nil {
		runCmd("yum", "install", "-y", "google-noto-sans-cjk-fonts", "wqy-zenhei-fonts", "wqy-microhei-fonts")
		runCmd("fc-cache", "-fv")
		return
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		runCmd("dnf", "install", "-y", "google-noto-sans-cjk-fonts", "wqy-zenhei-fonts", "wqy-microhei-fonts")
		runCmd("fc-cache", "-fv")
		return
	}
}

// ==================== 工具函数 ====================

func runCmd(name string, args ...string) bool {
	log.Printf("[INIT] 执行: %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[WARN] 命令失败: %v", err)
		return false
	}
	return true
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func getExecDir() string {
	execPath, err := os.Executable()
	if err != nil {
		dir, _ := os.Getwd()
		return dir
	}
	return filepath.Dir(execPath)
}

func findChromiumInDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil { return "" }
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "chromium-") { continue }
		binPath := filepath.Join(dir, entry.Name(), "chrome.exe")
		if _, err := os.Stat(binPath); err == nil { return binPath }
		binPath = filepath.Join(dir, entry.Name(), "chrome")
		if _, err := os.Stat(binPath); err == nil { return binPath }
	}
	return ""
}
