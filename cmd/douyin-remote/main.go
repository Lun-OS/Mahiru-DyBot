//go:build windows

// Mahiru DyBot Remote —— Mahiru DyBot 桌面客户端（Windows，WebView2 内嵌网页外壳）
//
// 完全自包含：整个 WebUI 前端(含登录页)通过 go:embed 打包进本程序，
// 运行时由本地 HTTP 服务提供页面；对远端服务端的依赖仅为 API 反向代理与 UDP 数据面。
//
// 本地服务(127.0.0.1:随机端口, 单一来源)：
//   /webui/*        内嵌前端静态文件(SPA fallback index.html)
//   /api/* /event   反向代理 → 远端服务端(HTTP/WS)
//   /remote         本地 WS 桥：下行二进制=JPEG整帧 / 文本=JSON状态；上行鼠标键盘 JSON
//
// 数据面 UDP(服务端:17837)：HELLO 鉴权 → FRAME_REQ 拉流(JPEG分片≤30fps) → MOUSE·KEY 注入
//
// 用法:
//   douyin-remote.exe -server IP:17837 -account acc_xxx [-token 可选] [-http http://IP:17836]
//   -token 可选：不填则先在内嵌页面登录，登录成功自动注入令牌并建立画面链路。
package main

import (
	"embed"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jchv/go-webview2"
)

//go:embed all:webui
var webuiFS embed.FS

const (
	magic        = 0xD0B1
	tHello       = 0x01
	tFrameReq    = 0x02
	tMouse       = 0x03
	tKey         = 0x04
	tInfo        = 0x81
	tFrame       = 0x82
	pullPeriod   = 33 * time.Millisecond // ≈30fps
)

var errText = map[byte]string{
	1: "令牌无效或已过期（请在内嵌页面重新登录）",
	2: "账号不在线或不存在",
	3: "服务端内部错误",
	4: "协议错误",
}

// ---------------- UDP 数据面 ----------------

type udpPump struct {
	conn      net.Conn
	accountID string
	token     string

	helloOK atomic.Bool
	vpW     int
	vpH     int
	jpegQ   int

	mu      sync.Mutex
	parts   map[uint32][][]byte
	lastSeq uint32

	wsMu      sync.Mutex
	wsClients map[*wsConn]struct{}
}

type wsConn struct {
	c    *websocket.Conn
	send chan []byte
}

func newPump(account, token string) *udpPump {
	return &udpPump{
		accountID: account,
		token:     token,
		parts:     map[uint32][][]byte{},
		wsClients: map[*wsConn]struct{}{},
	}
}

func pack(t byte, payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint16(out, magic)
	out[2] = t
	copy(out[4:], payload)
	return out
}

func (p *udpPump) helloPacket() []byte {
	p.mu.Lock()
	tok := p.token
	acc := p.accountID
	p.mu.Unlock()
	var pl []byte
	pl = append(pl, byte(len(tok)))
	pl = append(pl, tok...)
	pl = append(pl, byte(len(acc)))
	pl = append(pl, acc...)
	return pack(tHello, pl)
}

func (p *udpPump) mousePacket(sub byte, x, y float32, buttons byte) []byte {
	pl := make([]byte, 10)
	pl[0] = sub
	binary.LittleEndian.PutUint32(pl[1:], math.Float32bits(x))
	binary.LittleEndian.PutUint32(pl[5:], math.Float32bits(y))
	pl[9] = buttons
	return pack(tMouse, pl)
}

func (p *udpPump) keyPacket(name string) []byte {
	return pack(tKey, append([]byte{byte(len(name))}, name...))
}

// SetToken 更新令牌并立即重新握手（内嵌页面登录成功后由 JS 绑定调用）。
func (p *udpPump) SetToken(tok string) {
	p.mu.Lock()
	p.token = tok
	p.mu.Unlock()
	if p.conn == nil {
		return
	}
	_, _ = p.conn.Write(p.helloPacket())
	log.Printf("[UDP] HELLO sent (new token): token=%dchars account=%s", len(tok), p.accountID)
}

// SetAccount 运行时切换画面目标账号（前端远程画布挂载时调用）。
func (p *udpPump) SetAccount(id string) {
	if id == "" {
		return
	}
	p.mu.Lock()
	p.accountID = id
	p.mu.Unlock()
	if p.conn == nil {
		return
	}
	p.helloOK.Store(false)
	st, _ := json.Marshal(map[string]interface{}{"status": "switching", "account": id})
	p.broadcast(st)
	_, _ = p.conn.Write(p.helloPacket())
	log.Printf("[UDP] HELLO sent: token=%dchars account=%s", len(p.token), id)
}

func (p *udpPump) start(server string) error {
	conn, err := net.Dial("udp", server)
	if err != nil {
		return err
	}
	p.conn = conn
	go p.readLoop()
	go p.pullLoop()
	go p.helloRetry()
	return nil
}

// helloRetry 持续尝试 HELLO：未握手成功时每3秒重发一次（token/account 可能后置注入）。
func (p *udpPump) helloRetry() {
	for {
		time.Sleep(3 * time.Second)
		if p.helloOK.Load() {
			continue
		}
		p.mu.Lock()
		acc := p.accountID
		tok := p.token
		p.mu.Unlock()
		if acc == "" || tok == "" {
			continue
		}
		if _, err := p.conn.Write(p.helloPacket()); err != nil {
			return
		}
		log.Printf("[UDP] HELLO retry: token=%dchars account=%s", len(tok), acc)
	}
}

func (p *udpPump) broadcast(v []byte) {
	p.wsMu.Lock()
	targets := make([]*wsConn, 0, len(p.wsClients))
	for c := range p.wsClients {
		targets = append(targets, c)
	}
	p.wsMu.Unlock()
	if len(targets) == 0 {
		if len(v) > 0 && v[0] == '{' {
			log.Printf("[bridge] broadcast: 0 clients, msg=%s", v)
		}
		return
	}
	for _, c := range targets {
		select {
		case c.send <- v:
		default:
			log.Printf("[bridge] broadcast: send buffer full, dropping %d bytes", len(v))
		}
	}
}

func (p *udpPump) readLoop() {
	buf := make([]byte, 65536)
	rehello := func() {
		time.Sleep(3 * time.Second)
		if p.conn != nil {
			_, _ = p.conn.Write(p.helloPacket())
		}
	}
	for {
		n, err := p.conn.Read(buf)
		if err != nil {
			p.broadcast([]byte(`{"status":"down","error":"UDP断开"}`))
			return
		}
		if n < 4 || binary.LittleEndian.Uint16(buf) != magic {
			continue
		}
		switch buf[2] {
		case tInfo:
			p.vpW = int(binary.LittleEndian.Uint16(buf[4:]))
			p.vpH = int(binary.LittleEndian.Uint16(buf[6:]))
			p.jpegQ = int(buf[8])
			p.helloOK.Store(true)
			log.Printf("[UDP] INFO: %dx%d q=%d", p.vpW, p.vpH, p.jpegQ)
			st, _ := json.Marshal(map[string]interface{}{"status": "online", "w": p.vpW, "h": p.vpH, "q": p.jpegQ})
			p.broadcast(st)
		case tFrame:
			p.onChunk(buf[:n])
		default:
			if n >= 6 && buf[2] == 0x83 {
				code := buf[4]
				l := int(buf[5])
				msgStr := ""
				if 6+l <= n {
					msgStr = string(buf[6 : 6+l])
				}
				if t, ok := errText[code]; ok {
					msgStr = t
				}
				log.Printf("[UDP] ERR code=%d msg=%s", code, msgStr)
				es, _ := json.Marshal(map[string]interface{}{"status": "error", "error": msgStr})
				p.broadcast(es)
				if code == 1 || code == 2 {
					go rehello()
				}
			}
		}
	}
}

func (p *udpPump) onChunk(pkt []byte) {
	body := pkt[4:]
	if len(body) < 12 {
		return
	}
	seq := binary.LittleEndian.Uint32(body)
	idx := int(binary.LittleEndian.Uint16(body[4:]))
	count := int(binary.LittleEndian.Uint16(body[6:]))
	total := int(binary.LittleEndian.Uint32(body[8:]))
	data := body[12:]

	p.mu.Lock()
	if seq != p.lastSeq {
		p.lastSeq = seq
		p.parts = map[uint32][][]byte{}
	}
	if _, ok := p.parts[seq]; !ok {
		p.parts[seq] = make([][]byte, count)
	}
	chunks := p.parts[seq]
	if idx < len(chunks) && chunks[idx] == nil {
		chunks[idx] = data
	}
	done := true
	for _, c := range chunks {
		if c == nil {
			done = false
			break
		}
	}
	full := chunks
	p.mu.Unlock()
	if !done {
		return
	}
	raw := make([]byte, 0, total)
	for _, c := range full {
		raw = append(raw, c...)
	}
	p.mu.Lock()
	delete(p.parts, seq)
	p.mu.Unlock()
	if len(raw) > 0 {
		first := "[]"
		if len(raw) >= 2 {
			first = fmt.Sprintf("[%02X %02X]", raw[0], raw[1])
		}
		log.Printf("[UDP] FRAME assembled: seq=%d chunks=%d bytes=%d first=%s", seq, count, len(raw), first)
	}
	p.broadcast(raw)
}

func (p *udpPump) pullLoop() {
	ticker := time.NewTicker(pullPeriod)
	defer ticker.Stop()
	req := pack(tFrameReq, nil)
	for range ticker.C {
		if !p.helloOK.Load() {
			continue
		}
		if _, err := p.conn.Write(req); err != nil {
			return
		}
	}
}

// ---------------- 本地一体化服务（静态内嵌 + API反代 + 远程桥） ----------------

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

func (p *udpPump) bridgeHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[bridge] upgrade failed: %v", err)
		return
	}
	client := &wsConn{c: c, send: make(chan []byte, 64)}
	p.wsMu.Lock()
	p.wsClients[client] = struct{}{}
	p.wsMu.Unlock()
	log.Printf("[bridge] client connected (total=%d)", len(p.wsClients))

	go func() { // 单写者
		for data := range client.send {
			mt := websocket.BinaryMessage
			if len(data) > 0 && data[0] == '{' {
				mt = websocket.TextMessage
			}
			_ = c.WriteMessage(mt, data)
		}
		_ = c.Close()
	}()

	defer func() {
		p.wsMu.Lock()
		delete(p.wsClients, client)
		p.wsMu.Unlock()
		close(client.send)
	}()

	c.SetReadLimit(1 << 20)
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			return
		}
		var ev struct {
			T       string  `json:"t"`
			Sub     int     `json:"sub"`
			X       float32 `json:"x"`
			Y       float32 `json:"y"`
			B       int     `json:"b"`
			Name    string  `json:"name"`
			Token   string  `json:"token"`
			Account string  `json:"account"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		switch ev.T {
		case "m":
			btn := byte(0)
			if ev.B&1 != 0 {
				btn = 1
			}
			_, _ = p.conn.Write(p.mousePacket(byte(ev.Sub), ev.X, ev.Y, btn))
		case "k":
			_, _ = p.conn.Write(p.keyPacket(ev.Name))
		case "hello":
			if ev.Token != "" {
				p.SetToken(ev.Token)
			}
			if ev.Account != "" {
				p.SetAccount(ev.Account)
			}
			log.Printf("[bridge] hello via WS: token=%dchars account=%s", len(ev.Token), ev.Account)
		}
	}
}

// spaHandler 内嵌前端的 SPA 托管。
func spaHandler() http.HandlerFunc {
	sub, _ := fs.Sub(webuiFS, "webui")
	fileServer := http.FileServer(http.FS(sub))
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/webui/")
		if rel == "" || rel == "." {
			serveIndexEmbedded(w, r, sub)
			return
		}
		if _, err := fs.Stat(sub, rel); err != nil {
			serveIndexEmbedded(w, r, sub)
			return
		}
		r2 := *r
		r2.URL.Path = "/" + rel
		fileServer.ServeHTTP(w, &r2)
	}
}

func serveIndexEmbedded(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	f, err := sub.Open("index.html")
	if err != nil {
		http.Error(w, "内嵌前端缺失", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, f)
}

// startLocalServer 启动本地单端口服务，返回 (桥WS地址, 本地基址)。
func startLocalServer(p *udpPump, remoteBase string) (string, string, error) {
	target, err := url.Parse(remoteBase)
	if err != nil {
		return "", "", err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"ok":false,"error":"无法连接服务端: ` + e.Error() + `"}`))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/remote", p.bridgeHandler)
	mux.HandleFunc("GET /webui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/webui/", http.StatusFound)
	})
	mux.HandleFunc("GET /webui/", spaHandler())
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/webui/", http.StatusFound)
	})
	// 其余全部反代到远端（/api/*、/event、OneBot action 等）
	// 注意：Go 1.22 ServeMux 按最长模式匹配，上面的具体路由优先于 "/"
	mux.Handle("/", proxy)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		if err := http.Serve(ln, mux); err != nil {
			log.Printf("[client] 本地服务退出: %v", err)
		}
	}()
	return fmt.Sprintf("ws://127.0.0.1:%d/remote", port), fmt.Sprintf("http://127.0.0.1:%d", port), nil
}

// ---------------- main ----------------

var (
	flagServer    = flag.String("server", "127.0.0.1:17837", "服务端 UDP 地址")
	flagHTTPBase  = flag.String("http", "", "服务端 WebUI/API 基址(默认由 -server 推导 http://<host>:17836)")
	flagToken     = flag.String("token", "", "WebUI 登录令牌(可选；不填则在内嵌页面登录后自动获取)")
	flagAccount   = flag.String("account", "", "(已废弃，可不填) 初始画面账号；通常由页面内动态选择")
	flagHeadlessT = flag.Int("headless-test", 0, "自检模式：不开窗口运行N秒后退出")
)

func main() {
	flag.Parse()
	// 无参数 = 默认连接本机服务(127.0.0.1)，双击即可用；
	// 远程服务器场景: douyin-remote.exe -server <IP>:17837 [-http http://IP:17836]
	base := *flagHTTPBase
	if base == "" {
		host, _, err := net.SplitHostPort(*flagServer)
		if err != nil || host == "" {
			host = "127.0.0.1"
		}
		base = "http://" + host + ":17836"
	}
	base = strings.TrimRight(base, "/")

	p := newPump(*flagAccount, *flagToken)
	if err := p.start(*flagServer); err != nil {
		log.Fatalf("[client] UDP 启动失败: %v", err)
	}

	bridgeURL, localBase, err := startLocalServer(p, base)
	if err != nil {
		log.Fatalf("[client] 本地服务启动失败: %v", err)
	}
	log.Printf("[client] 本地服务: %s (前端内嵌 | API→%s)", localBase, base)

	target := fmt.Sprintf("%s/webui/?remote=%s#/accounts", localBase, bridgeURL)

	if *flagHeadlessT > 0 {
		runSelfCheck(p, localBase)
		return
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Mahiru DyBot Console",
			Width:  1240,
			Height: 800,
			Center: true,
		},
	})
	if w == nil {
		log.Fatal("WebView2 初始化失败。请安装 Microsoft Edge WebView2 Runtime: https://go.microsoft.com/fwlink/p/?LinkId=2124703")
	}
	defer w.Destroy()
	w.SetTitle("Mahiru DyBot Console")
	w.SetSize(1240, 800, webview2.HintNone)
	if err := w.Bind("obSetToken", func(token string) error {
		p.SetToken(token)
		return nil
	}); err != nil {
		log.Printf("[client] Bind(obSetToken) FAIL: %v", err)
	}
	if err := w.Bind("obSetAccount", func(id string) error {
		p.SetAccount(id)
		return nil
	}); err != nil {
		log.Printf("[client] Bind(obSetAccount) FAIL: %v", err)
	}
	w.Navigate(target)
	w.Run()
}

func runSelfCheck(p *udpPump, localBase string) {
	log.Printf("[client] 自检模式 %d 秒（不开窗口）...", *flagHeadlessT)
	time.Sleep(time.Duration(*flagHeadlessT) * time.Second)

	resp, err := http.Get(localBase + "/webui/")
	if err != nil || resp.StatusCode != 200 {
		code := 0
		if err == nil {
			code = resp.StatusCode
		}
		log.Printf("[client] 内嵌前端: FAIL (err=%v code=%d)", err, code)
		os.Exit(1)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	log.Printf("[client] 内嵌前端: HTTP 200 len>100=%v", len(body) > 100)
	log.Printf("[client] HELLO=%v (未选账号/离线时为 false 属正常)", p.helloOK.Load())
}
