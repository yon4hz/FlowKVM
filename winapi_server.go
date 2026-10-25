//go:build !clientonly

package main

import (
	"syscall"
	"unsafe"
)

var (
	procSetWindowsHookExW          = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx             = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx        = user32.NewProc("UnhookWindowsHookEx")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW         = user32.NewProc("PostThreadMessageW")
	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procSetCursor                  = user32.NewProc("SetCursor")
	procGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
)

func IsKeyDown(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (uint16(r) & 0x8000) != 0
}

// CreateCursorHiderWindow creates an invisible topmost 120x120 window at the anchor point
// whose cursor is NULL so the local cursor completely disappears when controlling the remote PC.
func CreateCursorHiderWindow(anchorX, anchorY int32) uintptr {
	className, _ := syscall.UTF16PtrFromString("FlowKVMCursorHider")
	hInst, _, _ := procGetModuleHandleW.Call(0)

	wndProc := syscall.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
		if msg == WM_SETCURSOR {
			procSetCursor.Call(0)
			return 1
		}
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	})

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   wndProc,
		HInstance:     hInst,
		HCursor:       0,
		LpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	exStyle := uintptr(WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_LAYERED | WS_EX_NOACTIVATE)
	style := uintptr(WS_POPUP)

	hwnd, _, _ := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		style,
		uintptr(anchorX-60),
		uintptr(anchorY-60),
		120,
		120,
		0, 0, hInst, 0,
	)
	if hwnd != 0 {
		procSetLayeredWindowAttributes.Call(hwnd, 0, 1, LWA_ALPHA)
	}
	return hwnd
}

func ShowCursorHider(hwnd uintptr, anchorX, anchorY int32, show bool) {
	if hwnd == 0 {
		return
	}
	if show {
		procSetWindowPos.Call(hwnd, HWND_TOPMOST, uintptr(anchorX-60), uintptr(anchorY-60), 120, 120, SWP_NOACTIVATE)
		procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE)
		procSetCursor.Call(0)
	} else {
		procShowWindow.Call(hwnd, SW_HIDE)
	}
}
