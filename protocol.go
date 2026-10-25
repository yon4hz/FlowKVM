package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	DefaultTCPPort      = 24800
	DefaultUDPPort      = 24801
	DefaultFilePort     = 24802
	DiscoveryMagic      = "FLOWKVM_DISCOVER_V1"
	DiscoveryReply      = "FLOWKVM_SERVER_V1"
	DiscoverClientMagic = "FLOWKVM_FIND_CLIENT_V1"
	DiscoverClientReply = "FLOWKVM_CLIENT_V1"
	HeartbeatPeriod     = 1000 * time.Millisecond
	ReadTimeout         = 15000 * time.Millisecond
	WriteTimeout        = 10000 * time.Millisecond
	MaxClipboardSize    = 32 * 1024 * 1024 // 32 MB max text clipboard
)

// Packet message types
const (
	MsgHandshake     byte = 1
	MsgPing          byte = 2
	MsgPong          byte = 3
	MsgMouseMove     byte = 4
	MsgMouseButton   byte = 5
	MsgMouseWheel    byte = 6
	MsgKeyEvent      byte = 7
	MsgClipboardText byte = 8
	MsgSwitchActive  byte = 9
)

// Mouse button actions
const (
	BtnLeftDown   byte = 1
	BtnLeftUp     byte = 2
	BtnRightDown  byte = 3
	BtnRightUp    byte = 4
	BtnMiddleDown byte = 5
	BtnMiddleUp   byte = 6
	BtnXDown      byte = 7
	BtnXUp        byte = 8
)

// Packet represents a unified message between Server and Client.
type Packet struct {
	Type byte
	// Handshake: Width, Height
	Width  int32
	Height int32
	// MouseMove: X, Y (absolute on client screen), DX, DY (relative delta)
	X  int32
	Y  int32
	DX int32
	DY int32
	// MouseButton: ButtonAction, XData (for XButton1/XButton2)
	ButtonAction byte
	XData        uint32
	// MouseWheel: WheelDelta, IsHorizontal
	WheelDelta   int32
	IsHorizontal bool
	// KeyEvent: VKCode, ScanCode, Flags, IsKeyUp
	VKCode   uint32
	ScanCode uint32
	Flags    uint32
	IsKeyUp  bool
	// SwitchActive: Active (true = client is being controlled, false = returned to server)
	Active bool
	// ClipboardText / Hostname
	Text string
}

// WritePacket serializes and writes a packet with a strict write deadline so it never hangs.
func WritePacket(conn net.Conn, w *bufio.Writer, p Packet) error {
	_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	if err := w.WriteByte(p.Type); err != nil {
		return err
	}

	var buf [16]byte
	switch p.Type {
	case MsgPing, MsgPong:
		// No payload needed

	case MsgHandshake:
		binary.LittleEndian.PutUint32(buf[0:4], uint32(p.Width))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(p.Height))
		strBytes := []byte(p.Text)
		if len(strBytes) > 255 {
			strBytes = strBytes[:255]
		}
		buf[8] = byte(len(strBytes))
		if _, err := w.Write(buf[:9]); err != nil {
			return err
		}
		if len(strBytes) > 0 {
			if _, err := w.Write(strBytes); err != nil {
				return err
			}
		}

	case MsgMouseMove:
		binary.LittleEndian.PutUint32(buf[0:4], uint32(p.X))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(p.Y))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(p.DX))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(p.DY))
		if _, err := w.Write(buf[:16]); err != nil {
			return err
		}

	case MsgMouseButton:
		buf[0] = p.ButtonAction
		binary.LittleEndian.PutUint32(buf[1:5], p.XData)
		if _, err := w.Write(buf[:5]); err != nil {
			return err
		}

	case MsgMouseWheel:
		binary.LittleEndian.PutUint32(buf[0:4], uint32(p.WheelDelta))
		if p.IsHorizontal {
			buf[4] = 1
		} else {
			buf[4] = 0
		}
		if _, err := w.Write(buf[:5]); err != nil {
			return err
		}

	case MsgKeyEvent:
		binary.LittleEndian.PutUint32(buf[0:4], p.VKCode)
		binary.LittleEndian.PutUint32(buf[4:8], p.ScanCode)
		binary.LittleEndian.PutUint32(buf[8:12], p.Flags)
		if p.IsKeyUp {
			buf[12] = 1
		} else {
			buf[12] = 0
		}
		if _, err := w.Write(buf[:13]); err != nil {
			return err
		}

	case MsgSwitchActive:
		if p.Active {
			buf[0] = 1
		} else {
			buf[0] = 0
		}
		binary.LittleEndian.PutUint32(buf[1:5], uint32(p.X))
		binary.LittleEndian.PutUint32(buf[5:9], uint32(p.Y))
		if _, err := w.Write(buf[:9]); err != nil {
			return err
		}

	case MsgClipboardText:
		strBytes := []byte(p.Text)
		if len(strBytes) > MaxClipboardSize {
			strBytes = strBytes[:MaxClipboardSize]
		}
		binary.LittleEndian.PutUint32(buf[0:4], uint32(len(strBytes)))
		if _, err := w.Write(buf[:4]); err != nil {
			return err
		}
		if len(strBytes) > 0 {
			if _, err := w.Write(strBytes); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("unknown packet type: %d", p.Type)
	}

	return w.Flush()
}

// ReadPacket reads and deserializes a packet with a strict read deadline to detect timeouts quickly.
func ReadPacket(conn net.Conn, r *bufio.Reader) (Packet, error) {
	_ = conn.SetReadDeadline(time.Now().Add(ReadTimeout))
	msgType, err := r.ReadByte()
	if err != nil {
		return Packet{}, err
	}

	p := Packet{Type: msgType}
	var buf [16]byte

	switch msgType {
	case MsgPing, MsgPong:
		return p, nil

	case MsgHandshake:
		if _, err := io.ReadFull(r, buf[:9]); err != nil {
			return p, err
		}
		p.Width = int32(binary.LittleEndian.Uint32(buf[0:4]))
		p.Height = int32(binary.LittleEndian.Uint32(buf[4:8]))
		nameLen := int(buf[8])
		if nameLen > 0 {
			nameBuf := make([]byte, nameLen)
			if _, err := io.ReadFull(r, nameBuf); err != nil {
				return p, err
			}
			p.Text = string(nameBuf)
		}
		return p, nil

	case MsgMouseMove:
		if _, err := io.ReadFull(r, buf[:16]); err != nil {
			return p, err
		}
		p.X = int32(binary.LittleEndian.Uint32(buf[0:4]))
		p.Y = int32(binary.LittleEndian.Uint32(buf[4:8]))
		p.DX = int32(binary.LittleEndian.Uint32(buf[8:12]))
		p.DY = int32(binary.LittleEndian.Uint32(buf[12:16]))
		return p, nil

	case MsgMouseButton:
		if _, err := io.ReadFull(r, buf[:5]); err != nil {
			return p, err
		}
		p.ButtonAction = buf[0]
		p.XData = binary.LittleEndian.Uint32(buf[1:5])
		return p, nil

	case MsgMouseWheel:
		if _, err := io.ReadFull(r, buf[:5]); err != nil {
			return p, err
		}
		p.WheelDelta = int32(binary.LittleEndian.Uint32(buf[0:4]))
		p.IsHorizontal = buf[4] != 0
		return p, nil

	case MsgKeyEvent:
		if _, err := io.ReadFull(r, buf[:13]); err != nil {
			return p, err
		}
		p.VKCode = binary.LittleEndian.Uint32(buf[0:4])
		p.ScanCode = binary.LittleEndian.Uint32(buf[4:8])
		p.Flags = binary.LittleEndian.Uint32(buf[8:12])
		p.IsKeyUp = buf[12] != 0
		return p, nil

	case MsgSwitchActive:
		if _, err := io.ReadFull(r, buf[:9]); err != nil {
			return p, err
		}
		p.Active = buf[0] != 0
		p.X = int32(binary.LittleEndian.Uint32(buf[1:5]))
		p.Y = int32(binary.LittleEndian.Uint32(buf[5:9]))
		return p, nil

	case MsgClipboardText:
		if _, err := io.ReadFull(r, buf[:4]); err != nil {
			return p, err
		}
		length := binary.LittleEndian.Uint32(buf[0:4])
		if length > MaxClipboardSize {
			return p, fmt.Errorf("clipboard payload too large: %d", length)
		}
		if length > 0 {
			strBuf := make([]byte, length)
			if _, err := io.ReadFull(r, strBuf); err != nil {
				return p, err
			}
			p.Text = string(strBuf)
		}
		return p, nil

	default:
		return p, fmt.Errorf("unknown packet type: %d", msgType)
	}
}

// ConfigureTCPConn sets aggressive keepalive and disables Nagle's algorithm (TCP_NODELAY)
// so mouse and keyboard events are transmitted with 0ms buffering delay and never silently drop.
func ConfigureTCPConn(conn net.Conn) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(2 * time.Second)
		_ = tcpConn.SetReadBuffer(64 * 1024)
		_ = tcpConn.SetWriteBuffer(64 * 1024)
	}
}
