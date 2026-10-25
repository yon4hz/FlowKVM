package main

import (
	"fmt"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procDragQueryFileW                = shell32.NewProc("DragQueryFileW")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procGetCursorPos                  = user32.NewProc("GetCursorPos")
	procSetCursorPos                  = user32.NewProc("SetCursorPos")
	procSendInput                     = user32.NewProc("SendInput")
	procMapVirtualKeyW                = user32.NewProc("MapVirtualKeyW")

	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")

	procGlobalAlloc             = kernel32.NewProc("GlobalAlloc")
	procGlobalFree              = kernel32.NewProc("GlobalFree")
	procGlobalLock              = kernel32.NewProc("GlobalLock")
	procGlobalUnlock            = kernel32.NewProc("GlobalUnlock")
	procGlobalSize              = kernel32.NewProc("GlobalSize")
	procGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadId      = kernel32.NewProc("GetCurrentThreadId")
	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")

	// Pure Go tracked mouse button state (avoids calling GetAsyncKeyState on Client)
	clientRightBtnDown  atomic.Bool
	clientMiddleBtnDown atomic.Bool
	clientLastX         atomic.Int32
	clientLastY         atomic.Int32
)

func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[Kurtarıldı] Arka plan hatası: %v\n", r)
			}
		}()
		fn()
	}()
}

const (
	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	WH_KEYBOARD_LL = 13
	WH_MOUSE_LL    = 14

	WM_SETCURSOR   = 0x0020
	WM_KEYDOWN     = 0x0100
	WM_KEYUP       = 0x0101
	WM_SYSKEYDOWN  = 0x0104
	WM_SYSKEYUP    = 0x0105
	WM_MOUSEMOVE   = 0x0200
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_RBUTTONDOWN = 0x0204
	WM_RBUTTONUP   = 0x0205
	WM_MBUTTONDOWN = 0x0207
	WM_MBUTTONUP   = 0x0208
	WM_MOUSEWHEEL  = 0x020A
	WM_XBUTTONDOWN = 0x020B
	WM_XBUTTONUP   = 0x020C
	WM_MOUSEHWHEEL = 0x020E
	WM_USER        = 0x0400

	WM_USER_SHOW_HIDER = WM_USER + 1
	WM_USER_HIDE_HIDER = WM_USER + 2

	INPUT_MOUSE    = 0
	INPUT_KEYBOARD = 1

	MOUSEEVENTF_MOVE        = 0x0001
	MOUSEEVENTF_LEFTDOWN    = 0x0002
	MOUSEEVENTF_LEFTUP      = 0x0004
	MOUSEEVENTF_RIGHTDOWN   = 0x0008
	MOUSEEVENTF_RIGHTUP     = 0x0010
	MOUSEEVENTF_MIDDLEDOWN  = 0x0020
	MOUSEEVENTF_MIDDLEUP    = 0x0040
	MOUSEEVENTF_XDOWN       = 0x0080
	MOUSEEVENTF_XUP         = 0x0100
	MOUSEEVENTF_WHEEL       = 0x0800
	MOUSEEVENTF_HWHEEL      = 0x1000
	MOUSEEVENTF_VIRTUALDESK = 0x4000
	MOUSEEVENTF_ABSOLUTE    = 0x8000

	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_SCANCODE    = 0x0008

	LLKHF_EXTENDED = 0x01
	LLKHF_INJECTED = 0x10
	LLMHF_INJECTED = 0x01

	VK_SHIFT    = 0x10
	VK_CONTROL  = 0x11
	VK_MENU     = 0x12 // Alt
	VK_PAUSE    = 0x13
	VK_ESCAPE   = 0x1B
	VK_S        = 0x53
	VK_Q        = 0x51
	VK_LWIN     = 0x5B
	VK_RWIN     = 0x5C
	VK_SCROLL   = 0x91
	VK_LSHIFT   = 0xA0
	VK_RSHIFT   = 0xA1
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
	VK_LMENU    = 0xA4
	VK_RMENU    = 0xA5

	WS_POPUP          = 0x80000000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_LAYERED     = 0x00080000
	WS_EX_NOACTIVATE  = 0x08000000
	LWA_ALPHA         = 0x00000002
	SW_HIDE           = 0
	SW_SHOWNOACTIVATE = 4
	SWP_NOSIZE        = 0x0001
	SWP_NOACTIVATE    = 0x0010
	HWND_TOPMOST      = ^uintptr(0) // -1

	CF_UNICODETEXT = 13
	CF_HDROP       = 15
	GMEM_MOVEABLE  = 0x0002
	GMEM_ZEROINIT  = 0x0040

	ES_SYSTEM_REQUIRED  = 0x00000001
	ES_DISPLAY_REQUIRED = 0x00000002
)

type POINT struct {
	X int32
	Y int32
}

type MSG struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type MSLLHOOKSTRUCT struct {
	Pt          POINT
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type KBDLLHOOKSTRUCT struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type mouseInput struct {
	inputType   uint32
	_pad0       uint32
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	_pad1       uint32
	dwExtraInfo uintptr
}

type keyboardInput struct {
	inputType   uint32
	_pad0       uint32
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	_pad1       uint32
	dwExtraInfo uintptr
	_unionPad   [8]byte
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

func EnableDPIAwareness() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		r, _, _ := procSetProcessDpiAwarenessContext.Call(^uintptr(3))
		if r != 0 {
			return
		}
	}
	if procSetProcessDPIAware.Find() == nil {
		_, _, _ = procSetProcessDPIAware.Call()
	}
}

func GetScreenSize() (int32, int32) {
	w, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	h, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	if w <= 0 {
		w = 1920
	}
	if h <= 0 {
		h = 1080
	}
	return int32(w), int32(h)
}

func GetCursorPos() (int32, int32) {
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return pt.X, pt.Y
}

func SetCursorPos(x, y int32) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
}

func KeepDisplayAwake() {
	procSetThreadExecutionState.Call(ES_SYSTEM_REQUIRED | ES_DISPLAY_REQUIRED)
}

// InjectMouseMove moves the cursor via SetCursorPos, and when right/middle button is held (Metin2 camera)
// sends relative SendInput movement without triggering antivirus SendInput-flood heuristics.
func InjectMouseMove(x, y, dx, dy, screenW, screenH int32) {
	clientLastX.Store(x)
	clientLastY.Store(y)
	SetCursorPos(x, y)

	if (dx != 0 || dy != 0) && (clientRightBtnDown.Load() || clientMiddleBtnDown.Load()) {
		inpRel := mouseInput{
			inputType: INPUT_MOUSE,
			dx:        dx,
			dy:        dy,
			dwFlags:   MOUSEEVENTF_MOVE,
		}
		procSendInput.Call(1, uintptr(unsafe.Pointer(&inpRel)), unsafe.Sizeof(inpRel))
	}
}

func InjectMouseButton(action byte, xData uint32) {
	// Update pure-Go tracked button state and ensure cursor position is synced for games like Metin2
	x, y := clientLastX.Load(), clientLastY.Load()
	w, h := GetScreenSize()
	if w > 1 && h > 1 && (x > 0 || y > 0) {
		normX := (int64(x) * 65535) / int64(w-1)
		normY := (int64(y) * 65535) / int64(h-1)
		inpMove := mouseInput{
			inputType: INPUT_MOUSE,
			dx:        int32(normX),
			dy:        int32(normY),
			dwFlags:   MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE,
		}
		procSendInput.Call(1, uintptr(unsafe.Pointer(&inpMove)), unsafe.Sizeof(inpMove))
	}

	var flags uint32
	switch action {
	case BtnLeftDown:
		flags = MOUSEEVENTF_LEFTDOWN
	case BtnLeftUp:
		flags = MOUSEEVENTF_LEFTUP
	case BtnRightDown:
		clientRightBtnDown.Store(true)
		flags = MOUSEEVENTF_RIGHTDOWN
	case BtnRightUp:
		clientRightBtnDown.Store(false)
		flags = MOUSEEVENTF_RIGHTUP
	case BtnMiddleDown:
		clientMiddleBtnDown.Store(true)
		flags = MOUSEEVENTF_MIDDLEDOWN
	case BtnMiddleUp:
		clientMiddleBtnDown.Store(false)
		flags = MOUSEEVENTF_MIDDLEUP
	case BtnXDown:
		flags = MOUSEEVENTF_XDOWN
	case BtnXUp:
		flags = MOUSEEVENTF_XUP
	default:
		return
	}
	inp := mouseInput{
		inputType: INPUT_MOUSE,
		mouseData: xData,
		dwFlags:   flags,
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func InjectMouseWheel(delta int32, isHorizontal bool) {
	var flags uint32 = MOUSEEVENTF_WHEEL
	if isHorizontal {
		flags = MOUSEEVENTF_HWHEEL
	}
	inp := mouseInput{
		inputType: INPUT_MOUSE,
		mouseData: uint32(delta),
		dwFlags:   flags,
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func InjectKeyEvent(vkCode, scanCode, llFlags uint32, isKeyUp bool) {
	if scanCode == 0 && vkCode != 0 {
		sc, _, _ := procMapVirtualKeyW.Call(uintptr(vkCode), 0)
		scanCode = uint32(sc)
	}
	var flags uint32
	if (llFlags & LLKHF_EXTENDED) != 0 {
		flags |= KEYEVENTF_EXTENDEDKEY
	}
	if isKeyUp {
		flags |= KEYEVENTF_KEYUP
	}
	inp := keyboardInput{
		inputType: INPUT_KEYBOARD,
		wVk:       uint16(vkCode),
		wScan:     uint16(scanCode),
		dwFlags:   flags,
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func ReleaseAllModifiers() {
	clientRightBtnDown.Store(false)
	clientMiddleBtnDown.Store(false)
	keys := []uint32{
		VK_LCONTROL, VK_RCONTROL, VK_CONTROL,
		VK_LSHIFT, VK_RSHIFT, VK_SHIFT,
		VK_LMENU, VK_RMENU, VK_MENU,
		VK_LWIN, VK_RWIN,
	}
	for _, vk := range keys {
		InjectKeyEvent(vk, 0, 0, true)
	}
}

func GetClipboardSequence() uint32 {
	r, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(r)
}

func ReadClipboardText() (string, bool) {
	avail, _, _ := procIsClipboardFormatAvailable.Call(CF_UNICODETEXT)
	if avail == 0 {
		return "", false
	}
	r, _, _ := procOpenClipboard.Call(0)
	if r == 0 {
		return "", false
	}
	defer procCloseClipboard.Call()

	hMem, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if hMem == 0 {
		return "", false
	}
	memSize, _, _ := procGlobalSize.Call(hMem)
	if memSize < 2 || memSize > MaxClipboardSize {
		return "", false
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return "", false
	}
	defer procGlobalUnlock.Call(hMem)

	maxChars := int(memSize) / 2
	u16Slice := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), maxChars)
	length := 0
	for length < maxChars && u16Slice[length] != 0 {
		length++
	}
	return string(utf16.Decode(u16Slice[:length])), true
}

func WriteClipboardText(text string) bool {
	if len(text) == 0 || len(text) > MaxClipboardSize {
		return false
	}
	u16 := utf16.Encode([]rune(text))
	u16 = append(u16, 0)
	byteSize := uintptr(len(u16) * 2)

	r, _, _ := procOpenClipboard.Call(0)
	if r == 0 {
		return false
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, byteSize)
	if hMem == 0 {
		return false
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return false
	}

	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(u16))
	copy(dst, u16)
	procGlobalUnlock.Call(hMem)

	res, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, hMem)
	if res == 0 {
		procGlobalFree.Call(hMem)
		return false
	}
	return true
}

func ReadClipboardFiles() ([]string, bool) {
	avail, _, _ := procIsClipboardFormatAvailable.Call(CF_HDROP)
	if avail == 0 {
		return nil, false
	}
	r, _, _ := procOpenClipboard.Call(0)
	if r == 0 {
		return nil, false
	}
	defer procCloseClipboard.Call()

	hDrop, _, _ := procGetClipboardData.Call(CF_HDROP)
	if hDrop == 0 {
		return nil, false
	}
	ptr, _, _ := procGlobalLock.Call(hDrop)
	if ptr == 0 {
		return nil, false
	}
	defer procGlobalUnlock.Call(hDrop)

	count, _, _ := procDragQueryFileW.Call(ptr, 0xFFFFFFFF, 0, 0)
	if count == 0 || count > 10000 {
		return nil, false
	}

	var files []string
	for i := uintptr(0); i < count; i++ {
		charLen, _, _ := procDragQueryFileW.Call(ptr, i, 0, 0)
		if charLen == 0 || charLen > 32768 {
			continue
		}
		buf := make([]uint16, charLen+1)
		procDragQueryFileW.Call(ptr, i, uintptr(unsafe.Pointer(&buf[0])), charLen+1)
		files = append(files, syscall.UTF16ToString(buf))
	}
	return files, len(files) > 0
}

func WriteClipboardFiles(paths []string) bool {
	if len(paths) == 0 {
		return false
	}

	var u16List []uint16
	for _, p := range paths {
		if p == "" {
			continue
		}
		encoded := utf16.Encode([]rune(p))
		u16List = append(u16List, encoded...)
		u16List = append(u16List, 0)
	}
	if len(u16List) == 0 {
		return false
	}
	u16List = append(u16List, 0)

	const dropFilesHeaderSize = 20
	totalBytes := uintptr(dropFilesHeaderSize + len(u16List)*2)

	r, _, _ := procOpenClipboard.Call(0)
	if r == 0 {
		return false
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE|GMEM_ZEROINIT, totalBytes)
	if hMem == 0 {
		return false
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return false
	}

	header := unsafe.Slice((*uint32)(unsafe.Pointer(ptr)), 5)
	header[0] = dropFilesHeaderSize
	header[1] = 0
	header[2] = 0
	header[3] = 0
	header[4] = 1

	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr+dropFilesHeaderSize)), len(u16List))
	copy(dst, u16List)
	procGlobalUnlock.Call(hMem)

	res, _, _ := procSetClipboardData.Call(CF_HDROP, hMem)
	if res == 0 {
		procGlobalFree.Call(hMem)
		return false
	}
	return true
}
