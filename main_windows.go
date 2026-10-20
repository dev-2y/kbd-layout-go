//go:build windows

// layout-tray - tray icon showing the active keyboard layout as a flag: Brazil
// for Portuguese (ABNT2), the United States for US-International.
//
// Go port of the Rust program; the window, tray and menu code comes from
// mouse-battery. Standard library only (syscall); see build.cmd.
package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
	"golang.org/x/sys/windows"
)

func init() {
	// The window, its message loop and the tray all belong to one OS thread.
	runtime.LockOSThread()
}

const (
	appName       = "LayoutTray"
	mutexName     = `Local\LayoutTray_singleton`
	timerLayout   = 1
	layoutPollMS  = 500
)

func main() { os.Exit(run()) }

func run() int {
	var err error
	if g.br, err = decodeFlag(brFlagPNG); err != nil {
		return 1
	}
	if g.us, err = decodeFlag(usFlagPNG); err != nil {
		return 1
	}

	// Only one instance at a time. The handle is kept open until the process
	// ends, which is what holds the mutex.
	mutex, _, e := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16Ptr(mutexName))))
	if mutex != 0 {
		if err, ok := e.(windows.Errno); ok && err == windows.ERROR_ALREADY_EXISTS {
			return 0
		}
	}

	procSetProcessDPIAware.Call()
	instance, _, _ := procGetModuleHandleW.Call(0)

	var wc wndClassEx
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(wndProc)
	wc.HInstance = instance
	wc.LpszClassName = utf16Ptr(appName)
	// Application icon: resource id 1, the exe's file icon (see rsrc_*.syso).
	wc.HIcon, _, _ = procLoadIconW.Call(instance, 1)
	wc.HIconSm = wc.HIcon
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return 1
	}

	// A real (never shown) top-level window: message-only windows miss the
	// TaskbarCreated broadcast that follows an Explorer restart.
	name := uintptr(unsafe.Pointer(utf16Ptr(appName)))
	g.hwnd, _, _ = procCreateWindowExW.Call(0, name, name, wsOverlapped,
		0, 0, 0, 0, 0, 0, instance, 0)
	if g.hwnd == 0 {
		return 1
	}

	tb, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(utf16Ptr("TaskbarCreated"))))
	g.taskbarCreated = uint32(tb)

	// The first icon already reflects the layout in use (the Rust version always
	// started with the US flag until the layout changed). "Other" starts as US.
	g.shown = LayoutEnUS
	if l := currentLayout(); l != LayoutOther {
		g.shown = l
	}
	trayRefresh(true)
	procSetTimer.Call(g.hwnd, timerLayout, layoutPollMS, 0)

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	if g.icon != 0 {
		procDestroyIcon.Call(g.icon)
	}
	runtime.KeepAlive(mutex)
	return 0
}
