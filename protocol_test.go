package main

import (
	"bufio"
	"net"
	"testing"
	"unsafe"
)

func TestInputStructSizes(t *testing.T) {
	if sz := unsafe.Sizeof(mouseInput{}); sz != 40 {
		t.Fatalf("expected mouseInput size 40 on 64-bit Windows, got %d", sz)
	}
	if sz := unsafe.Sizeof(keyboardInput{}); sz != 40 {
		t.Fatalf("expected keyboardInput size 40 on 64-bit Windows, got %d", sz)
	}
}

func TestPacketRoundTrip(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	packets := []Packet{
		{Type: MsgHandshake, Width: 1920, Height: 1080, Text: "MAIN-PC"},
		{Type: MsgPing},
		{Type: MsgPong},
		{Type: MsgMouseMove, X: 500, Y: 600, DX: -12, DY: 8},
		{Type: MsgMouseButton, ButtonAction: BtnLeftDown, XData: 0},
		{Type: MsgMouseWheel, WheelDelta: -120, IsHorizontal: false},
		{Type: MsgKeyEvent, VKCode: 0x41, ScanCode: 0x1E, Flags: 0, IsKeyUp: false},
		{Type: MsgSwitchActive, Active: true, X: 1914, Y: 540},
		{Type: MsgClipboardText, Text: "Merhaba Dünya! Türkçe Karakter Testi ğüşıöçĞÜŞİÖÇ"},
	}

	go func() {
		w := bufio.NewWriter(c1)
		for _, p := range packets {
			if err := WritePacket(c1, w, p); err != nil {
				t.Errorf("WritePacket failed: %v", err)
				return
			}
		}
	}()

	r := bufio.NewReader(c2)
	for i, exp := range packets {
		got, err := ReadPacket(c2, r)
		if err != nil {
			t.Fatalf("packet %d ReadPacket failed: %v", i, err)
		}
		if got != exp {
			t.Fatalf("packet %d mismatch: got %+v, want %+v", i, got, exp)
		}
	}
}

func TestClipboardFileDropRoundTrip(t *testing.T) {
	testPaths := []string{
		`C:\Windows\notepad.exe`,
		`C:\Windows\explorer.exe`,
	}
	if !WriteClipboardFiles(testPaths) {
		t.Skip("Clipboard not accessible in current test session")
	}
	got, ok := ReadClipboardFiles()
	if !ok || len(got) != len(testPaths) {
		t.Fatalf("ReadClipboardFiles failed: ok=%v, got=%v", ok, got)
	}
	for i := range testPaths {
		if got[i] != testPaths[i] {
			t.Fatalf("path %d mismatch: got %q, want %q", i, got[i], testPaths[i])
		}
	}
}

