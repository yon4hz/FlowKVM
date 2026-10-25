//go:build !clientonly

package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const DefaultBuildMode = "server"

type Server struct {
	tcpPort   int
	edge      string // "left", "right", "top", "bottom"
	targetIP  string // optional Client IP or "auto"
	localW    int32
	localH    int32
	anchorX   int32
	anchorY   int32
	hiderHwnd uintptr
	hookTID   uint32

	isRemote  atomic.Bool
	connected atomic.Bool

	mu          sync.Mutex
	clientConn  net.Conn
	clientName  string
	clientW     int32
	clientH     int32
	remoteX     int32
	remoteY     int32
	ignoredIPs  map[string]bool

	// Coalesced mouse move state (prevents 1000Hz mouse from flooding USB Tethering / Wi-Fi)
	moveDirty      atomic.Bool
	moveX          atomic.Int32
	moveY          atomic.Int32
	moveDX         atomic.Int32
	moveDY         atomic.Int32
	lastVerifiedIP atomic.Value // string

	sendCh          chan Packet
	lastClipSeq     uint32
	lastRecvClipTxt string
	fileMgr         *FileTransferManager
}

func NewServer(port int, edge string, targetIP string) *Server {
	w, h := GetScreenSize()
	s := &Server{
		tcpPort:    port,
		edge:       strings.ToLower(edge),
		targetIP:   strings.TrimSpace(targetIP),
		localW:     w,
		localH:     h,
		anchorX:    w / 2,
		anchorY:    h / 2,
		clientW:    w,
		clientH:    h,
		remoteX:    w / 2,
		remoteY:    h / 2,
		ignoredIPs: make(map[string]bool),
		sendCh:     make(chan Packet, 512),
	}
	s.fileMgr = NewFileTransferManager(DefaultFilePort, func() {
		s.lastClipSeq = GetClipboardSequence()
	})
	return s
}

func (s *Server) isIgnoredIP(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ignoredIPs[ip]
}

func (s *Server) addIgnoredIP(ip string) {
	if ip == "" || ip == "127.0.0.1" {
		return
	}
	s.mu.Lock()
	s.ignoredIPs[ip] = true
	s.mu.Unlock()
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", s.tcpPort))
	if err != nil {
		return fmt.Errorf("TCP port %d dinlenemedi: %w", s.tcpPort, err)
	}

	s.printBanner()

	// Start Dedicated File Transfer Channel (Port 24802)
	s.fileMgr.StartListener()

	// Start UDP Discovery Responder & Broadcaster
	go s.runUDPDiscovery()

	// Start Active Outbound Client Seeker (Solves Phone USB Tethering / NAT!)
	go s.runActiveClientSeeker()

	// Start Clipboard Watcher
	go s.runClipboardWatcher()

	// Start TCP Accept Loop
	go s.acceptLoop(ln)

	// Run Windows Low-Level Hooks on the main locked OS thread
	s.runHookThread()
	return nil
}

func (s *Server) printBanner() {
	hostname, _ := os.Hostname()
	ips := getLocalIPv4s()
	edgeDesc := map[string]string{
		"left":   T("SOL KENAR (Fareyi sola götürünce diğer PC'ye geçer)", "LEFT EDGE (Move cursor to left edge to switch)"),
		"right":  T("SAĞ KENAR (Fareyi sağa götürünce diğer PC'ye geçer)", "RIGHT EDGE (Move cursor to right edge to switch)"),
		"top":    T("ÜST KENAR (Fareyi yukarı götürünce diğer PC'ye geçer)", "TOP EDGE (Move cursor to top edge to switch)"),
		"bottom": T("ALT KENAR (Fareyi aşağı götürünce diğer PC'ye geçer)", "BOTTOM EDGE (Move cursor to bottom edge to switch)"),
	}[s.edge]

	fmt.Println("================================================================")
	fmt.Printf("  FlowKVM - %s [%s]\n", T("ANA BİLGİSAYAR (SERVER) AKTİF", "MAIN PC (SERVER) ACTIVE"), hostname)
	fmt.Println("================================================================")
	fmt.Printf("  %-18s: %dx%d\n", T("Ekran Çözünürlüğü", "Screen Resolution"), s.localW, s.localH)
	fmt.Printf("  %-18s: %s\n", T("Geçiş Yönü", "Switch Edge"), edgeDesc)
	fmt.Printf("  %-18s: Ctrl + Alt + S\n", T("Kısayol (Geçiş)", "Switch Hotkey"))
	fmt.Printf("  %-18s: Ctrl + Alt + Q  (Pause/Break)\n", T("Acil Geri Çağırma", "Failsafe Recall"))
	fmt.Printf("  %-18s: %s (Port: %d)\n", T("Yerel IP Adresleri", "Local IP Addresses"), strings.Join(ips, ", "), s.tcpPort)
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("  [*] %s\n", T(
		"Diğer bilgisayar ağda (192.168.0.x / 192.168.1.x) aranıyor...",
		"Searching for Second PC on network (192.168.0.x / 192.168.1.x)...",
	))
	fmt.Println("================================================================")
}

func (s *Server) runActiveClientSeeker() {
	var lastKnownIP string
	if s.targetIP != "" && s.targetIP != "auto" {
		lastKnownIP = s.targetIP
	}

	getMyIPs := func() map[string]bool {
		m := map[string]bool{"127.0.0.1": true}
		for _, ip := range getLocalIPv4s() {
			m[ip] = true
		}
		s.mu.Lock()
		for ip := range s.ignoredIPs {
			m[ip] = true
		}
		s.mu.Unlock()
		return m
	}

	tryDial := func(ip string, myIPs map[string]bool, timeout time.Duration) bool {
		if ip == "" || myIPs[ip] || s.connected.Load() {
			return false
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, s.tcpPort), timeout)
		if err != nil {
			return false
		}
		ConfigureTCPConn(conn)
		if !s.verifyAndStartSession(conn) {
			_ = conn.Close()
			return false
		}
		lastKnownIP = ip
		return true
	}

	for {
		if s.connected.Load() {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		myIPs := getMyIPs()

		if v, ok := s.lastVerifiedIP.Load().(string); ok && v != "" && !myIPs[v] {
			lastKnownIP = v
		}

		// 1. Try last known or user-configured Client IP first
		if lastKnownIP != "" && !myIPs[lastKnownIP] {
			if tryDial(lastKnownIP, myIPs, 1000*time.Millisecond) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if s.targetIP != "" && s.targetIP != "auto" {
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}

		// 2. Fast UDP Probe across upstream Wi-Fi / LAN subnets (192.168.0.x, 192.168.1.x)
		if foundIP := s.udpProbeSubnets(myIPs, 500*time.Millisecond); foundIP != "" {
			if tryDial(foundIP, myIPs, 1000*time.Millisecond) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}

		// 3. Fallback: Parallel TCP scan of 192.168.0.x and 192.168.1.x on port 24800
		subnets := []string{"192.168.0", "192.168.1"}
		for _, sub := range subnets {
			if s.connected.Load() {
				break
			}
			if foundIP := s.tcpScanSubnet(sub, myIPs, 250*time.Millisecond); foundIP != "" {
				lastKnownIP = foundIP
				break
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (s *Server) udpProbeSubnets(myIPs map[string]bool, timeout time.Duration) string {
	if s.connected.Load() {
		return ""
	}
	uconn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return ""
	}
	defer uconn.Close()

	payload := []byte(DiscoverClientMagic)
	subnets := []string{"192.168.0", "192.168.1"}
	for _, sub := range subnets {
		for i := 1; i <= 254; i++ {
			ip := fmt.Sprintf("%s.%d", sub, i)
			if myIPs[ip] {
				continue
			}
			dst := &net.UDPAddr{IP: net.ParseIP(ip), Port: DefaultUDPPort}
			_, _ = uconn.WriteToUDP(payload, dst)
			if i%25 == 0 {
				time.Sleep(2 * time.Millisecond)
			}
		}
	}

	_ = uconn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 256)
	for {
		n, raddr, err := uconn.ReadFromUDP(buf)
		if err != nil {
			return ""
		}
		msg := strings.TrimSpace(string(buf[:n]))
		if strings.HasPrefix(msg, DiscoverClientReply+"|") {
			ip := raddr.IP.String()
			if !myIPs[ip] {
				return ip
			}
		}
	}
}

func (s *Server) tcpScanSubnet(subnetPrefix string, myIPs map[string]bool, timeout time.Duration) string {
	ipCh := make(chan string, 254)
	for i := 1; i <= 254; i++ {
		ip := fmt.Sprintf("%s.%d", subnetPrefix, i)
		if !myIPs[ip] {
			ipCh <- ip
		}
	}
	close(ipCh)

	foundCh := make(chan string, 1)
	var wg sync.WaitGroup
	workers := 48
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ipCh {
				if s.connected.Load() || len(foundCh) > 0 {
					return
				}
				conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, s.tcpPort), timeout)
				if err == nil {
					ConfigureTCPConn(conn)
					if s.verifyAndStartSession(conn) {
						select {
						case foundCh <- ip:
						default:
						}
						return
					}
					_ = conn.Close()
				}
			}
		}()
	}
	wg.Wait()

	select {
	case ip := <-foundCh:
		return ip
	default:
		return ""
	}
}

func (s *Server) runUDPDiscovery() {
	hostname, _ := os.Hostname()
	replyPayload := []byte(fmt.Sprintf("%s|%d|%s", DiscoveryReply, s.tcpPort, hostname))

	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", DefaultUDPPort))
	if err == nil {
		conn, err := net.ListenUDP("udp4", addr)
		if err == nil {
			defer conn.Close()
			go func() {
				buf := make([]byte, 256)
				for {
					n, raddr, err := conn.ReadFromUDP(buf)
					if err != nil {
						return
					}
					msg := strings.TrimSpace(string(buf[:n]))
					if msg == DiscoveryMagic {
						_, _ = conn.WriteToUDP(replyPayload, raddr)
					}
				}
			}()
		}
	}

	bcastConn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return
	}
	defer bcastConn.Close()

	for {
		time.Sleep(1500 * time.Millisecond)
		if s.connected.Load() {
			continue
		}
		for _, bcastIP := range getBroadcastIPs() {
			dst, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", bcastIP, DefaultUDPPort))
			if err == nil {
				_, _ = bcastConn.WriteToUDP(replyPayload, dst)
			}
		}
	}
}

func (s *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		ConfigureTCPConn(conn)
		go func(c net.Conn) {
			if !s.verifyAndStartSession(c) {
				_ = c.Close()
			}
		}(conn)
	}
}

// verifyAndStartSession verifies that the connection is not a phone NAT hairpin (e.g. 192.168.0.6 -> 192.168.0.6)
// and reads the initial Client MsgHandshake BEFORE claiming the session.
func (s *Server) verifyAndStartSession(conn net.Conn) bool {
	if s.connected.Load() {
		return false
	}

	localHost, _, _ := net.SplitHostPort(conn.LocalAddr().String())
	remoteHost, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	// Detect phone USB Tethering NAT hairpin (where dialing the phone's Wi-Fi IP reflects back to ourselves!)
	if localHost != "" && localHost == remoteHost && localHost != "127.0.0.1" {
		s.addIgnoredIP(remoteHost)
		return false
	}

	r := bufio.NewReaderSize(conn, 16*1024)
	_ = conn.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
	msgType, err := r.ReadByte()
	if err != nil || msgType != MsgHandshake {
		return false
	}
	// Put the byte back or read the rest of MsgHandshake
	_ = r.UnreadByte()
	pkt, err := ReadPacket(conn, r)
	if err != nil || pkt.Type != MsgHandshake {
		return false
	}

	s.mu.Lock()
	if s.clientConn != nil || s.connected.Load() {
		s.mu.Unlock()
		return false
	}
	s.clientConn = conn
	s.connected.Store(true)
	if pkt.Width > 0 && pkt.Height > 0 {
		s.clientW = pkt.Width
		s.clientH = pkt.Height
	}
	s.clientName = pkt.Text
	s.mu.Unlock()

	if remoteHost != "" && remoteHost != "127.0.0.1" {
		s.lastVerifiedIP.Store(remoteHost)
	}
	if remoteHost != "" {
		s.fileMgr.SetPeerIP(remoteHost, true)
	}

	go s.handleClient(conn, r, pkt)
	return true
}

func (s *Server) handleClient(conn net.Conn, r *bufio.Reader, initHandshake Packet) {
	remoteAddr := conn.RemoteAddr().String()
	fmt.Printf("\n[+] %s: %s -> %s (%dx%d)\n",
		T("BAĞLANDI", "CONNECTED"),
		remoteAddr, initHandshake.Text, initHandshake.Width, initHandshake.Height)

	for len(s.sendCh) > 0 {
		<-s.sendCh
	}
	s.moveDirty.Store(false)
	s.lastClipSeq = GetClipboardSequence()

	done := make(chan struct{})
	var closeOnce sync.Once
	closeSession := func() {
		closeOnce.Do(func() {
			close(done)
			_ = conn.Close()
		})
	}

	// Send initial handshake with Server hostname
	hostname, _ := os.Hostname()
	s.enqueue(Packet{
		Type:   MsgHandshake,
		Width:  s.localW,
		Height: s.localH,
		Text:   hostname,
	})

	// Writer + 125Hz Mouse Move Flusher + Heartbeat Goroutine
	go func() {
		w := bufio.NewWriterSize(conn, 16*1024)
		hbTicker := time.NewTicker(HeartbeatPeriod)
		moveTicker := time.NewTicker(8 * time.Millisecond) // 125Hz smooth mouse rate
		defer hbTicker.Stop()
		defer moveTicker.Stop()

		flushMouseMove := func() error {
			if s.moveDirty.CompareAndSwap(true, false) {
				mx := s.moveX.Load()
				my := s.moveY.Load()
				mdx := s.moveDX.Swap(0)
				mdy := s.moveDY.Swap(0)
				return WritePacket(conn, w, Packet{
					Type: MsgMouseMove,
					X:    mx,
					Y:    my,
					DX:   mdx,
					DY:   mdy,
				})
			}
			return nil
		}

		for {
			select {
			case <-done:
				return
			case pkt := <-s.sendCh:
				if err := flushMouseMove(); err != nil {
					closeSession()
					return
				}
				if err := WritePacket(conn, w, pkt); err != nil {
					closeSession()
					return
				}
			case <-moveTicker.C:
				if err := flushMouseMove(); err != nil {
					closeSession()
					return
				}
			case <-hbTicker.C:
				if err := WritePacket(conn, w, Packet{Type: MsgPing}); err != nil {
					closeSession()
					return
				}
			}
		}
	}()

	// Reader Loop
	for {
		pkt, err := ReadPacket(conn, r)
		if err != nil {
			fmt.Printf("[!] Okuma hatası (%s): %v\n", remoteAddr, err)
			break
		}
		switch pkt.Type {
		case MsgPing:
			s.enqueue(Packet{Type: MsgPong})
		case MsgPong:
			// Heartbeat alive
		case MsgHandshake:
			s.mu.Lock()
			if pkt.Width > 0 && pkt.Height > 0 {
				s.clientW = pkt.Width
				s.clientH = pkt.Height
			}
			s.clientName = pkt.Text
			s.mu.Unlock()
		case MsgClipboardText:
			if pkt.Text != "" {
				s.mu.Lock()
				s.lastRecvClipTxt = pkt.Text
				s.mu.Unlock()
				if WriteClipboardText(pkt.Text) {
					s.lastClipSeq = GetClipboardSequence()
				}
			}
		}
	}

	closeSession()

	s.mu.Lock()
	if s.clientConn == conn {
		s.clientConn = nil
		s.connected.Store(false)
	}
	s.mu.Unlock()

	if s.isRemote.Load() {
		fmt.Println(T(
			"[!] Bağlantı koptu - Failsafe devreye girdi, fare Ana Bilgisayara geri alındı!",
			"[!] Connection lost - Failsafe triggered, cursor recalled to Main PC!",
		))
		s.forceReturnToLocal()
	} else {
		fmt.Printf("[!] %s %s\n", remoteAddr, T("bağlantısı koptu, otomatik yeniden bağlanılıyor...", "disconnected, auto-reconnecting..."))
	}
}

func (s *Server) enqueue(p Packet) {
	if !s.connected.Load() {
		return
	}
	select {
	case s.sendCh <- p:
	default:
	}
}

func (s *Server) runClipboardWatcher() {
	s.lastClipSeq = GetClipboardSequence()
	for {
		time.Sleep(500 * time.Millisecond)
		if !s.connected.Load() {
			s.lastClipSeq = GetClipboardSequence()
			continue
		}
		seq := GetClipboardSequence()
		if seq != s.lastClipSeq {
			s.lastClipSeq = seq
			if files, ok := ReadClipboardFiles(); ok && len(files) > 0 {
				if !s.fileMgr.IsStagingPath(files) {
					s.fileMgr.SendFilesAsync(files)
				}
				continue
			}
			txt, ok := ReadClipboardText()
			if !ok || txt == "" {
				continue
			}
			s.mu.Lock()
			sameAsRecv := (txt == s.lastRecvClipTxt)
			s.mu.Unlock()
			if !sameAsRecv {
				s.enqueue(Packet{
					Type: MsgClipboardText,
					Text: txt,
				})
			}
		}
	}
}

func (s *Server) switchToRemote(entryX, entryY int32, fromHotkey bool) {
	if !s.connected.Load() || s.isRemote.Load() {
		return
	}

	s.localW, s.localH = GetScreenSize()
	s.anchorX = s.localW / 2
	s.anchorY = s.localH / 2

	s.mu.Lock()
	cw, ch := s.clientW, s.clientH
	if cw <= 0 {
		cw = s.localW
	}
	if ch <= 0 {
		ch = s.localH
	}

	if fromHotkey {
		s.remoteX = cw / 2
		s.remoteY = ch / 2
	} else {
		switch s.edge {
		case "left":
			s.remoteX = cw - 6
			s.remoteY = (entryY * ch) / s.localH
		case "right":
			s.remoteX = 6
			s.remoteY = (entryY * ch) / s.localH
		case "top":
			s.remoteX = (entryX * cw) / s.localW
			s.remoteY = ch - 6
		case "bottom":
			s.remoteX = (entryX * cw) / s.localW
			s.remoteY = 6
		default:
			s.remoteX = cw - 6
			s.remoteY = (entryY * ch) / s.localH
		}
	}
	rx, ry := s.remoteX, s.remoteY
	s.mu.Unlock()

	s.isRemote.Store(true)
	ShowCursorHider(s.hiderHwnd, s.anchorX, s.anchorY, true)
	SetCursorPos(s.anchorX, s.anchorY)
	if s.hookTID != 0 {
		procPostThreadMessageW.Call(uintptr(s.hookTID), WM_USER_SHOW_HIDER, 0, 0)
	}

	s.enqueue(Packet{
		Type:   MsgSwitchActive,
		Active: true,
		X:      rx,
		Y:      ry,
	})
	fmt.Printf("[->] %s (X:%d, Y:%d)\n", T("Kontrol Diğer Bilgisayara Geçti", "Switched Control to Second PC"), rx, ry)
}

func (s *Server) switchToLocal(returnX, returnY int32) {
	if !s.isRemote.CompareAndSwap(true, false) {
		return
	}
	s.moveDirty.Store(false)
	ShowCursorHider(s.hiderHwnd, s.anchorX, s.anchorY, false)
	SetCursorPos(returnX, returnY)
	if s.hookTID != 0 {
		procPostThreadMessageW.Call(uintptr(s.hookTID), WM_USER_HIDE_HIDER, uintptr(returnX), uintptr(returnY))
	}

	s.enqueue(Packet{
		Type:   MsgSwitchActive,
		Active: false,
	})
	fmt.Printf("[<-] %s (X:%d, Y:%d)\n", T("Kontrol Ana Bilgisayara Döndü", "Returned Control to Main PC"), returnX, returnY)
}

func (s *Server) forceReturnToLocal() {
	if !s.isRemote.CompareAndSwap(true, false) {
		return
	}
	s.moveDirty.Store(false)
	cx, cy := s.localW/2, s.localH/2
	if s.hookTID != 0 {
		procPostThreadMessageW.Call(uintptr(s.hookTID), WM_USER_HIDE_HIDER, uintptr(cx), uintptr(cy))
	}
	SetCursorPos(cx, cy)
	s.enqueue(Packet{
		Type:   MsgSwitchActive,
		Active: false,
	})
}

var activeServer *Server

func (s *Server) runHookThread() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	activeServer = s
	tid, _, _ := procGetCurrentThreadId.Call()
	s.hookTID = uint32(tid)

	s.hiderHwnd = CreateCursorHiderWindow(s.anchorX, s.anchorY)

	mouseHook, _, _ := procSetWindowsHookExW.Call(
		WH_MOUSE_LL,
		syscall.NewCallback(lowLevelMouseProc),
		0,
		0,
	)
	if mouseHook == 0 {
		fmt.Println("[HATA] Fare kancası (WH_MOUSE_LL) kurulamadı!")
		return
	}
	defer procUnhookWindowsHookEx.Call(mouseHook)

	kbdHook, _, _ := procSetWindowsHookExW.Call(
		WH_KEYBOARD_LL,
		syscall.NewCallback(lowLevelKeyboardProc),
		0,
		0,
	)
	if kbdHook == 0 {
		fmt.Println("[HATA] Klavye kancası (WH_KEYBOARD_LL) kurulamadı!")
		return
	}
	defer procUnhookWindowsHookEx.Call(kbdHook)

	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		if msg.Message == WM_USER_SHOW_HIDER {
			ShowCursorHider(s.hiderHwnd, s.anchorX, s.anchorY, true)
			SetCursorPos(s.anchorX, s.anchorY)
			continue
		}
		if msg.Message == WM_USER_HIDE_HIDER {
			ShowCursorHider(s.hiderHwnd, s.anchorX, s.anchorY, false)
			if msg.WParam != 0 || msg.LParam != 0 {
				SetCursorPos(int32(msg.WParam), int32(msg.LParam))
			}
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func lowLevelMouseProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	s := activeServer
	if nCode < 0 || s == nil {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	info := (*MSLLHOOKSTRUCT)(unsafe.Pointer(lParam))

	if !s.isRemote.Load() {
		if wParam == WM_MOUSEMOVE && s.connected.Load() {
			x, y := info.Pt.X, info.Pt.Y
			switch s.edge {
			case "left":
				if x <= 0 {
					s.switchToRemote(x, y, false)
					return 1
				}
			case "right":
				if x >= s.localW-1 {
					s.switchToRemote(x, y, false)
					return 1
				}
			case "top":
				if y <= 0 {
					s.switchToRemote(x, y, false)
					return 1
				}
			case "bottom":
				if y >= s.localH-1 {
					s.switchToRemote(x, y, false)
					return 1
				}
			}
		}
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	if !s.connected.Load() {
		s.switchToLocal(s.localW/2, s.localH/2)
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	switch wParam {
	case WM_MOUSEMOVE:
		if info.Pt.X == s.anchorX && info.Pt.Y == s.anchorY {
			ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
			return ret
		}

		curX, curY := GetCursorPos()
		dx := info.Pt.X - curX
		dy := info.Pt.Y - curY
		if curX != s.anchorX || curY != s.anchorY {
			SetCursorPos(s.anchorX, s.anchorY)
		}
		if dx == 0 && dy == 0 {
			return 1
		}

		s.mu.Lock()
		cw, ch := s.clientW, s.clientH
		s.remoteX += dx
		s.remoteY += dy

		returned := false
		var retX, retY int32
		switch s.edge {
		case "left":
			if s.remoteX >= cw {
				returned = true
				retX = 6
				retY = (s.remoteY * s.localH) / ch
			}
		case "right":
			if s.remoteX <= 0 {
				returned = true
				retX = s.localW - 6
				retY = (s.remoteY * s.localH) / ch
			}
		case "top":
			if s.remoteY >= ch {
				returned = true
				retX = (s.remoteX * s.localW) / cw
				retY = 6
			}
		case "bottom":
			if s.remoteY <= 0 {
				returned = true
				retX = (s.remoteX * s.localW) / cw
				retY = s.localH - 6
			}
		}

		if !returned {
			if s.remoteX < 0 {
				s.remoteX = 0
			} else if s.remoteX >= cw {
				s.remoteX = cw - 1
			}
			if s.remoteY < 0 {
				s.remoteY = 0
			} else if s.remoteY >= ch {
				s.remoteY = ch - 1
			}
		}
		rx, ry := s.remoteX, s.remoteY
		s.mu.Unlock()

		if returned {
			if retY < 0 {
				retY = 0
			} else if retY >= s.localH {
				retY = s.localH - 1
			}
			if retX < 0 {
				retX = 0
			} else if retX >= s.localW {
				retX = s.localW - 1
			}
			s.switchToLocal(retX, retY)
			return 1
		}

		s.moveX.Store(rx)
		s.moveY.Store(ry)
		s.moveDX.Add(dx)
		s.moveDY.Add(dy)
		s.moveDirty.Store(true)
		return 1

	case WM_LBUTTONDOWN:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnLeftDown})
		return 1
	case WM_LBUTTONUP:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnLeftUp})
		return 1
	case WM_RBUTTONDOWN:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnRightDown})
		return 1
	case WM_RBUTTONUP:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnRightUp})
		return 1
	case WM_MBUTTONDOWN:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnMiddleDown})
		return 1
	case WM_MBUTTONUP:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnMiddleUp})
		return 1
	case WM_XBUTTONDOWN:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnXDown, XData: info.MouseData >> 16})
		return 1
	case WM_XBUTTONUP:
		s.enqueue(Packet{Type: MsgMouseButton, ButtonAction: BtnXUp, XData: info.MouseData >> 16})
		return 1
	case WM_MOUSEWHEEL:
		delta := int32(int16(info.MouseData >> 16))
		s.enqueue(Packet{Type: MsgMouseWheel, WheelDelta: delta, IsHorizontal: false})
		return 1
	case WM_MOUSEHWHEEL:
		delta := int32(int16(info.MouseData >> 16))
		s.enqueue(Packet{Type: MsgMouseWheel, WheelDelta: delta, IsHorizontal: true})
		return 1
	}

	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func lowLevelKeyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	s := activeServer
	if nCode < 0 || s == nil {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	}

	info := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
	isKeyUp := (wParam == WM_KEYUP || wParam == WM_SYSKEYUP)

	if !isKeyUp {
		ctrlDown := IsKeyDown(VK_CONTROL) || IsKeyDown(VK_LCONTROL) || IsKeyDown(VK_RCONTROL)
		altDown := IsKeyDown(VK_MENU) || IsKeyDown(VK_LMENU) || IsKeyDown(VK_RMENU)

		if ctrlDown && altDown && info.VkCode == VK_S {
			if s.isRemote.Load() {
				s.switchToLocal(s.localW/2, s.localH/2)
			} else if s.connected.Load() {
				s.switchToRemote(s.localW/2, s.localH/2, true)
			}
			return 1
		}

		if (ctrlDown && altDown && info.VkCode == VK_Q) || info.VkCode == VK_PAUSE {
			if s.isRemote.Load() {
				s.switchToLocal(s.localW/2, s.localH/2)
				return 1
			}
		}
	}

	if s.isRemote.Load() {
		if !s.connected.Load() {
			s.switchToLocal(s.localW/2, s.localH/2)
			ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
			return ret
		}
		s.enqueue(Packet{
			Type:     MsgKeyEvent,
			VKCode:   info.VkCode,
			ScanCode: info.ScanCode,
			Flags:    info.Flags,
			IsKeyUp:  isKeyUp,
		})
		return 1
	}

	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}
