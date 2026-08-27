package onebot

// UDP 网关：为 Windows 桌面客户端(douyin-remote)提供实时画面与控制。
//
// 协议(小端二进制)，公共头: magic u16=0xD0B1 | type u8 | rsv u8
//
//	C→S 0x01 HELLO   {u8 tokLen, token, u8 accLen, accountID}   鉴权并绑定账号
//	C→S 0x02 FRAME_REQ {}                                       请求一帧(客户端自节拍拉流)
//	C→S 0x03 MOUSE   {u8 sub(0move/1down/2up), f32 x, f32 y, u8 buttons}
//	C→S 0x04 KEY     {u8 len, keyName}
//
//	S→C 0x81 INFO    {u16 width, u16 height, u8 jpegQ}
//	S→C 0x82 FRAME   {u32 seq, u16 idx, u16 count, u32 total, data[]}  分片≤1400B
//	S→C 0x83 ERR     {u8 code, u8 len, message}
//
// 设计要点：拉流模式天然限速；丢片直接弃帧等下一帧；控制事件短包容忍丢包；
// 会话以远端地址标识，HELLO 可随时切换目标账号。

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"mahiru-dybot/internal/browser"
)

const (
	udpMagic      = 0xD0B1
	udpChunkSize  = 1400
	udpMinFrameGap = 33 * time.Millisecond // 单会话 ≤30fps 软上限
)

// C→S 消息类型。
const (
	mHello = 0x01
	mFrameReq = 0x02
	mMouse = 0x03
	mKey = 0x04
)

// S→C 消息类型。
const (
	mInfo = 0x81
	mFrame = 0x82
	mErr = 0x83
)

// ERR code。
const (
	eBadAuth = 1
	eBadAccount = 2
	eInternal = 3
	eBadPacket = 4
)

type udpSession struct {
	addr        net.Addr
	accountID   string
	lastFrame   time.Time
}

type udpServer struct {
	srv *Server
	conn *net.UDPConn
	mu sync.Mutex
	sessions map[string]*udpSession
	captureMu sync.Mutex // 全局截图串行化（截图约20ms，规模足够）
}

// StartUDP 启动 UDP 网关（阻塞）。
func (s *Server) StartUDP(addr string) error {
	if addr == "" {
		addr = ":17837"
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	s.udp.conn = conn
	log.Printf("[UDP] 桌面客户端网关已启动 udp://0.0.0.0%s", addr)

	buf := make([]byte, 65536)
	for {
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if strings.Contains(err.Error(), "closed") {
				return nil
			}
			continue
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		s.udpHandle(pkt, raddr)
	}
}

func (s *Server) udpSend(addr net.Addr, msgType byte, payload []byte) {
	ra, ok := addr.(*net.UDPAddr)
	if !ok {
		return
	}
	out := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint16(out[0:], udpMagic)
	out[2] = msgType
	out[3] = 0
	copy(out[4:], payload)
	_, _ = s.udp.conn.WriteToUDP(out, ra)
}

func (s *Server) udpSendErr(addr net.Addr, code byte, msg string) {
	p := make([]byte, 0, 2+len(msg))
	p = append(p, code, byte(len(msg)))
	p = append(p, msg...)
	s.udpSend(addr, mErr, p)
}

func (s *Server) udpHandle(pkt []byte, raddr *net.UDPAddr) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[UDP] PANIC: %v", r)
		}
	}()
	if len(pkt) < 4 || binary.LittleEndian.Uint16(pkt[0:]) != udpMagic {
		return
	}
	msgType := pkt[2]
	body := pkt[4:]

	switch msgType {
	case mHello:
		s.udpHandleHello(body, raddr)
	case mFrameReq:
		s.udpHandleFrameReq(raddr)
	case mMouse:
		s.udpHandleMouse(body, raddr)
	case mKey:
		s.udpHandleKey(body, raddr)
	default:
		s.udpSendErr(raddr, eBadPacket, "未知消息类型")
	}
}

// ---------- HELLO ----------

func (s *Server) udpHandleHello(body []byte, raddr *net.UDPAddr) {
	log.Printf("[UDP] HELLO recv: bodyLen=%d raddr=%s", len(body), raddr)
	if len(body) < 1 {
		s.udpSendErr(raddr, eBadPacket, "HELLO 格式错误")
		return
	}
	tokLen := int(body[0])
	pos := 1
	if pos+tokLen > len(body) {
		s.udpSendErr(raddr, eBadPacket, "HELLO 格式错误")
		return
	}
	token := string(body[pos : pos+tokLen])
	pos += tokLen
	if pos >= len(body) {
		s.udpSendErr(raddr, eBadPacket, "HELLO 缺少账号ID")
		return
	}
	accLen := int(body[pos])
	pos++
	if pos+accLen > len(body) {
		s.udpSendErr(raddr, eBadPacket, "HELLO 格式错误")
		return
	}
	accountID := string(body[pos : pos+accLen])

	if !s.Auth.Validate(token) {
		log.Printf("[UDP] HELLO 拒绝: token无效 len=%d", tokLen)
		s.udpSendErr(raddr, eBadAuth, "令牌无效或已过期，请重新登录 WebUI")
		return
	}
	info, ok := s.BM.Info(accountID)
	if !ok {
		log.Printf("[UDP] HELLO 拒绝: 账号不存在 account=%s", accountID)
		s.udpSendErr(raddr, eBadAccount, "账号不存在: "+accountID)
		return
	}
	// 检查浏览器实例是否在运行（qr_pending/online/error 等均可拉帧，仅 stopped 不行）
	inst, iok := func() (*browser.Instance, bool) {
		a, gok := s.BM.Get(accountID)
		if !gok { return nil, false }
		return a.Instance(), a.Instance() != nil
	}()
	if !iok || inst == nil {
		log.Printf("[UDP] HELLO 拒绝: 浏览器未运行 account=%s state=%s", accountID, info.State)
		s.udpSendErr(raddr, eBadAccount, "浏览器未运行，请先启动: "+accountID)
		return
	}

	s.udp.mu.Lock()
	s.udp.sessions[raddr.String()] = &udpSession{addr: raddr, accountID: accountID, lastFrame: time.Time{}}
	s.udp.mu.Unlock()

	w, h := 1920, 1080
	q := s.RT.Get().JpegQuality
	if inst != nil {
		vw, vh := inst.ViewportSize()
		w, h = int(vw), int(vh)
	}
	payload := make([]byte, 5)
	binary.LittleEndian.PutUint16(payload[0:], uint16(w))
	binary.LittleEndian.PutUint16(payload[2:], uint16(h))
	payload[4] = byte(q)
	s.udpSend(raddr, mInfo, payload)
	log.Printf("[UDP] 会话建立 %s -> %s(%s)", raddr, accountID, info.Name)
}

// sessionFor 取会话并校验账号在线。
func (s *Server) udpSessionOnline(raddr *net.UDPAddr) (*udpSession, *browser.Instance, bool) {
	s.udp.mu.Lock()
	sess, ok := s.udp.sessions[raddr.String()]
	s.udp.mu.Unlock()
	if !ok {
		s.udpSendErr(raddr, eBadAuth, "请先发送 HELLO 鉴权")
		return nil, nil, false
	}
	acc, ok := s.BM.Get(sess.accountID)
	if !ok {
		s.udpSendErr(raddr, eBadAccount, "账号不存在")
		return nil, nil, false
	}
	inst := acc.Instance()
	if inst == nil {
		s.udpSendErr(raddr, eBadAccount, "账号浏览器未运行")
		return nil, nil, false
	}
	return sess, inst, true
}

// ---------- FRAME_REQ ----------

func (s *Server) udpHandleFrameReq(raddr *net.UDPAddr) {
	sess, inst, ok := s.udpSessionOnline(raddr)
	if !ok {
		return
	}
	now := time.Now()
	if now.Sub(sess.lastFrame) < udpMinFrameGap {
		return // 超频请求静默丢弃
	}
	sess.lastFrame = now

	q := s.RT.Get().JpegQuality
	s.udp.captureMu.Lock()
	data, w, h, err := inst.CaptureJPEG(q)
	s.udp.captureMu.Unlock()
	if err != nil {
		s.udpSendErr(raddr, eInternal, "截图失败: "+err.Error())
		return
	}

	total := len(data)
	count := (total + udpChunkSize - 1) / udpChunkSize
	if count == 0 {
		count = 1
	}
	seq := uint32(now.UnixNano() & 0xFFFFFFFF)
	hdr := make([]byte, 12)
	for i := 0; i < count; i++ {
		start := i * udpChunkSize
		end := start + udpChunkSize
		if end > total {
			end = total
		}
		binary.LittleEndian.PutUint32(hdr[0:], seq)
		binary.LittleEndian.PutUint16(hdr[4:], uint16(i))
		binary.LittleEndian.PutUint16(hdr[6:], uint16(count))
		binary.LittleEndian.PutUint32(hdr[8:], uint32(total))
		frame := make([]byte, 0, 16+len(hdr)+end-start)
		pre := make([]byte, 4)
		binary.LittleEndian.PutUint16(pre[0:], udpMagic)
		pre[2] = mFrame
		frame = append(frame, pre...)
		frame = append(frame, hdr...)
		frame = append(frame, data[start:end]...)
		_, _ = s.udp.conn.WriteToUDP(frame, raddr)
	}
	_ = w
	_ = h
}

// ---------- MOUSE / KEY ----------

func (s *Server) udpHandleMouse(body []byte, raddr *net.UDPAddr) {
	if len(body) < 10 {
		s.udpSendErr(raddr, eBadPacket, "MOUSE 格式错误")
		return
	}
	sub := body[0]
	x := float64(math.Float32frombits(binary.LittleEndian.Uint32(body[1:])))
	y := float64(math.Float32frombits(binary.LittleEndian.Uint32(body[5:])))
	buttons := body[9]

	sess, inst, ok := s.udpSessionOnline(raddr)
	if !ok {
		return
	}
	in := inst.SessionInput()
	var err error
	switch sub {
	case 0:
		err = in.MouseMove(x, y, buttons&1 != 0)
	case 1:
		err = in.MouseDown(x, y)
	case 2:
		err = in.MouseUp(x, y)
	default:
		return
	}
	if err != nil {
		log.Printf("[UDP] %s 注入失败(%v): %v", sess.accountID, sub, err)
	}
}

func (s *Server) udpHandleKey(body []byte, raddr *net.UDPAddr) {
	if len(body) < 1 {
		return
	}
	nameLen := int(body[0])
	if 1+nameLen > len(body) {
		return
	}
	keyName := string(body[1 : 1+nameLen])
	sess, inst, ok := s.udpSessionOnline(raddr)
	if !ok {
		return
	}
	if err := inst.SessionInput().KeyPress(keyName); err != nil {
		log.Printf("[UDP] %s 按键失败(%s): %v", sess.accountID, keyName, err)
	}
}

var _ = json.Marshal // 占位保持导入一致
