package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	serverIP   string
	tcpPort    int
	connected  atomic.Bool
	sendCh     chan Packet
	mu         sync.Mutex
	activeConn net.Conn
	lastClip   string
	lastSeq    uint32
	lastWakeUp time.Time
	fileMgr    *FileTransferManager
}

func NewClient(serverIP string, port int) *Client {
	c := &Client{
		serverIP: strings.TrimSpace(serverIP),
		tcpPort:  port,
		sendCh:   make(chan Packet, 256),
	}
	c.fileMgr = NewFileTransferManager(DefaultFilePort, func() {
		c.lastSeq = GetClipboardSequence()
	})
	return c
}

func (c *Client) Start() error {
	hostname, _ := os.Hostname()
	w, h := GetScreenSize()
	localIPs := getLocalIPv4s()

	fmt.Println("================================================================")
	fmt.Printf("  FlowKVM - DİĞER BİLGİSAYAR (CLIENT) AKTİF [%s]\n", hostname)
	fmt.Println("================================================================")
	fmt.Printf("  Ekran Çözünürlüğü       : %dx%d\n", w, h)
	fmt.Printf("  Bu PC'nin IP Adresleri  : %s (Port: %d)\n", strings.Join(localIPs, ", "), c.tcpPort)
	fmt.Println("  USB Tethering / NAT Modu: Aktif (Çift Yönlü Bağlantı Desteği)")
	fmt.Println("  Timeout Koruması        : Aktif (15sn Tolerans + Oto-Reconnect)")
	fmt.Println("================================================================")
	fmt.Println("  [*] Ana Bilgisayar bekleniyor veya ağda aranıyor...")

	// Start Dedicated File Transfer Channel (Port 24802)
	c.fileMgr.StartListener()

	go c.runClipboardWatcher()

	// 1. Run UDP Responder so a USB-tethered Server behind phone NAT can discover this Client on Wi-Fi!
	go c.runClientUDPResponder()

	// 2. Run TCP Listener on Client so a USB-tethered Server can connect OUTBOUND to this Client!
	go c.runReverseTCPListener()

	// 3. Also run standard outbound discovery/connect in case both are on the same LAN
	for {
		if c.connected.Load() {
			time.Sleep(400 * time.Millisecond)
			continue
		}

		targetIP := c.serverIP
		targetPort := c.tcpPort

		if targetIP == "" || targetIP == "auto" {
			ip, port, srvName, err := c.discoverServer(2500 * time.Millisecond)
			if err != nil {
				continue
			}
			if c.connected.Load() {
				continue
			}
			targetIP = ip
			targetPort = port
			fmt.Printf("[✓] Ana Bilgisayar Bulundu: %s (%s:%d)\n", srvName, targetIP, targetPort)
		}

		if c.connected.Load() {
			continue
		}

		addr := fmt.Sprintf("%s:%d", targetIP, targetPort)
		conn, err := net.DialTimeout("tcp", addr, 1500*time.Millisecond)
		if err != nil {
			time.Sleep(400 * time.Millisecond)
			continue
		}

		ConfigureTCPConn(conn)
		if !c.claimAndStartSession(conn) {
			_ = conn.Close()
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func (c *Client) claimAndStartSession(conn net.Conn) bool {
	c.mu.Lock()
	if c.activeConn != nil || c.connected.Load() {
		c.mu.Unlock()
		return false
	}
	c.activeConn = conn
	c.connected.Store(true)
	c.mu.Unlock()

	go func() {
		c.handleSession(conn)
		ReleaseAllModifiers()
	}()
	return true
}

func (c *Client) runClientUDPResponder() {
	hostname, _ := os.Hostname()
	reply := []byte(fmt.Sprintf("%s|%d|%s", DiscoverClientReply, c.tcpPort, hostname))

	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("0.0.0.0:%d", DefaultUDPPort))
	if err != nil {
		return
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return
	}
	defer conn.Close()

	buf := make([]byte, 256)
	for {
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		msg := strings.TrimSpace(string(buf[:n]))
		if msg == DiscoverClientMagic {
			_, _ = conn.WriteToUDP(reply, raddr)
		}
	}
}

func (c *Client) runReverseTCPListener() {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", c.tcpPort))
	if err != nil {
		return
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		ConfigureTCPConn(conn)
		if !c.claimAndStartSession(conn) {
			_ = conn.Close()
		}
	}
}

func (c *Client) discoverServer(timeout time.Duration) (string, int, string, error) {
	if c.connected.Load() {
		return "", 0, "", fmt.Errorf("already connected")
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		return "", 0, "", err
	}
	defer conn.Close()

	req := []byte(DiscoveryMagic)
	for _, bcastIP := range getBroadcastIPs() {
		dst, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", bcastIP, DefaultUDPPort))
		if err == nil {
			_, _ = conn.WriteToUDP(req, dst)
		}
	}

	type result struct {
		ip   string
		port int
		name string
	}
	resCh := make(chan result, 2)

	go func() {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		buf := make([]byte, 256)
		for {
			n, raddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			msg := strings.TrimSpace(string(buf[:n]))
			if strings.HasPrefix(msg, DiscoveryReply+"|") {
				parts := strings.SplitN(msg, "|", 3)
				if len(parts) == 3 {
					p, err := strconv.Atoi(parts[1])
					if err == nil && p > 0 {
						select {
						case resCh <- result{ip: raddr.IP.String(), port: p, name: parts[2]}:
						default:
						}
						return
					}
				}
			}
		}
	}()

	select {
	case r := <-resCh:
		return r.ip, r.port, r.name, nil
	case <-time.After(timeout):
		return "", 0, "", fmt.Errorf("timeout")
	}
}

func (c *Client) handleSession(conn net.Conn) {
	for len(c.sendCh) > 0 {
		<-c.sendCh
	}

	done := make(chan struct{})
	var closeOnce sync.Once
	closeSession := func() {
		closeOnce.Do(func() {
			close(done)
			_ = conn.Close()
		})
	}
	defer func() {
		closeSession()
		c.mu.Lock()
		if c.activeConn == conn {
			c.activeConn = nil
			c.connected.Store(false)
		}
		c.mu.Unlock()
	}()

	w, h := GetScreenSize()
	hostname, _ := os.Hostname()

	// Send Client Handshake immediately
	c.enqueue(Packet{
		Type:   MsgHandshake,
		Width:  w,
		Height: h,
		Text:   hostname,
	})

	fmt.Printf("\n[+] Ana Bilgisayara Bağlandı (%s)\n", conn.RemoteAddr().String())
	c.lastSeq = GetClipboardSequence()
	if remoteHost, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil && remoteHost != "" {
		c.fileMgr.SetPeerIP(remoteHost, false)
	}

	// Writer + Heartbeat Goroutine
	go func() {
		bw := bufio.NewWriterSize(conn, 16*1024)
		ticker := time.NewTicker(HeartbeatPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case pkt := <-c.sendCh:
				if err := WritePacket(conn, bw, pkt); err != nil {
					closeSession()
					return
				}
			case <-ticker.C:
				nw, nh := GetScreenSize()
				if nw != w || nh != h {
					w, h = nw, nh
					_ = WritePacket(conn, bw, Packet{
						Type:   MsgHandshake,
						Width:  w,
						Height: h,
						Text:   hostname,
					})
				}
				if err := WritePacket(conn, bw, Packet{Type: MsgPing}); err != nil {
					closeSession()
					return
				}
			}
		}
	}()

	// Reader Loop
	br := bufio.NewReaderSize(conn, 16*1024)
	for {
		pkt, err := ReadPacket(conn, br)
		if err != nil {
			fmt.Printf("[!] Bağlantı kesildi (%v), yeniden bağlantı bekleniyor...\n", err)
			return
		}

		switch pkt.Type {
		case MsgPing:
			c.enqueue(Packet{Type: MsgPong})
		case MsgPong:
			// Heartbeat OK
		case MsgHandshake:
			fmt.Printf("[✓] Ana Bilgisayar Hazır: %s (%dx%d)\n", pkt.Text, pkt.Width, pkt.Height)
		case MsgSwitchActive:
			if pkt.Active {
				sw, sh := GetScreenSize()
				InjectMouseMove(pkt.X, pkt.Y, 0, 0, sw, sh)
				KeepDisplayAwake()
			} else {
				ReleaseAllModifiers()
			}
		case MsgMouseMove:
			sw, sh := GetScreenSize()
			InjectMouseMove(pkt.X, pkt.Y, pkt.DX, pkt.DY, sw, sh)
			if time.Since(c.lastWakeUp) > 10*time.Second {
				c.lastWakeUp = time.Now()
				KeepDisplayAwake()
			}
		case MsgMouseButton:
			InjectMouseButton(pkt.ButtonAction, pkt.XData)
		case MsgMouseWheel:
			InjectMouseWheel(pkt.WheelDelta, pkt.IsHorizontal)
		case MsgKeyEvent:
			InjectKeyEvent(pkt.VKCode, pkt.ScanCode, pkt.Flags, pkt.IsKeyUp)
		case MsgClipboardText:
			if pkt.Text != "" {
				c.mu.Lock()
				c.lastClip = pkt.Text
				c.mu.Unlock()
				if WriteClipboardText(pkt.Text) {
					c.lastSeq = GetClipboardSequence()
				}
			}
		}
	}
}

func (c *Client) enqueue(p Packet) {
	if !c.connected.Load() {
		return
	}
	select {
	case c.sendCh <- p:
	default:
	}
}

func (c *Client) runClipboardWatcher() {
	c.lastSeq = GetClipboardSequence()
	for {
		time.Sleep(500 * time.Millisecond)
		if !c.connected.Load() {
			c.lastSeq = GetClipboardSequence()
			continue
		}
		seq := GetClipboardSequence()
		if seq != c.lastSeq {
			c.lastSeq = seq
			if files, ok := ReadClipboardFiles(); ok && len(files) > 0 {
				if !c.fileMgr.IsStagingPath(files) {
					c.fileMgr.SendFilesAsync(files)
				}
				continue
			}
			txt, ok := ReadClipboardText()
			if !ok || txt == "" {
				continue
			}
			c.mu.Lock()
			same := (txt == c.lastClip)
			c.mu.Unlock()
			if !same {
				c.enqueue(Packet{
					Type: MsgClipboardText,
					Text: txt,
				})
			}
		}
	}
}
