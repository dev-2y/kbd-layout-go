//go:build windows

package main

import "unsafe"

const (
	wmTrayIcon = wmApp + 1

	idExit        = 1004
	trayUID       = 1
	timerLayout   = 1
	layoutPollMS  = 500
	appName       = "LayoutTray"
	mutexName     = `Local\LayoutTray_singleton`
	menuExitLabel = "Sair"
)

// Everything here belongs to the UI thread.
var g struct {
	hwnd           uintptr
	icon           uintptr
	taskbarCreated uint32
	shown          Layout // layout whose flag is on the tray right now
	br, us         *Flag
}

// ---------------------------------------------------------------- layout

// currentLayout asks for the keyboard layout of the thread that owns the
// foreground window, i.e. the one the user is typing into.
func currentLayout() Layout {
	var thread uintptr
	if fg, _, _ := procGetForegroundWindow.Call(); fg != 0 {
		thread, _, _ = procGetWindowThreadProcID.Call(fg, 0)
		thread &= 0xFFFFFFFF // DWORD result
	} else {
		thread, _, _ = procGetCurrentThreadID.Call()
	}
	hkl, _, _ := procGetKeyboardLayout.Call(thread)
	return classifyHKL(hkl)
}

// ---------------------------------------------------------------- tray icon

func flagFor(l Layout) *Flag {
	if l == LayoutPtBR {
		return g.br
	}
	return g.us
}

// trayRefresh draws the flag of g.shown and adds or updates the tray icon.
func trayRefresh(add bool) {
	previous := g.icon
	sz := smallIconSize()
	g.icon = iconFromBGRA(sz, flagFor(g.shown).square(sz))

	var nid notifyIconData
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = g.hwnd
	nid.UID = trayUID
	nid.UFlags = nifIcon | nifMessage | nifTip
	nid.UCallbackMessage = wmTrayIcon
	nid.HIcon = g.icon
	copyUTF16(nid.SzTip[:], g.shown.Tooltip())

	op := uint32(nimModify)
	if add {
		op = nimAdd
	}
	if !shellNotifyIcon(op, &nid) && !add {
		shellNotifyIcon(nimAdd, &nid)
	}
	if previous != 0 {
		procDestroyIcon.Call(previous)
	}
}

// checkLayout switches the flag when the layout changed; "other" layouts keep
// the previous flag.
func checkLayout() {
	if next, changed := nextShown(g.shown, currentLayout()); changed {
		g.shown = next
		trayRefresh(false)
	}
}

// ---------------------------------------------------------------- tray menu

func showMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	procAppendMenuW.Call(menu, mfString, idExit, uintptr(unsafe.Pointer(utf16Ptr(menuExitLabel))))

	var cursor point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	procSetForegroundWindow.Call(g.hwnd)
	procTrackPopupMenu.Call(menu, tpmRightButton, uintptr(cursor.X), uintptr(cursor.Y), 0, g.hwnd, 0)
	postMessage(g.hwnd, wmNull, 0, 0)
	procDestroyMenu.Call(menu)
}

// -------------------------------------------------------------- window proc

func wndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	if g.taskbarCreated != 0 && uint32(message) == g.taskbarCreated {
		trayRefresh(true)
		return 0
	}

	switch message {
	case wmTimer:
		if wparam == timerLayout {
			checkLayout()
		}
		return 0

	case wmTrayIcon:
		switch loword(lparam) {
		case wmRButtonUp, wmContextMenu:
			showMenu()
		}
		return 0

	case wmCommand:
		if loword(wparam) == idExit {
			procDestroyWindow.Call(hwnd)
		}
		return 0

	case wmDPIChanged, wmDisplayChange:
		trayRefresh(false)
		return 0

	case wmDestroy:
		procKillTimer.Call(hwnd, timerLayout)
		var nid notifyIconData
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		nid.HWnd = g.hwnd
		nid.UID = trayUID
		shellNotifyIcon(nimDelete, &nid)
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return r
}
