//go:build windows

package main

// Thin Win32 layer using golang.org/x/sys/windows package: no
// third-party modules. Pointers handed to the OS are always converted inline in
// the argument list of LazyProc.Call (which is //go:uintptrescapes), so they
// stay valid for the duration of the call.

import (
	"unicode/utf16"
	"unsafe"
	"golang.org/x/sys/windows"
)

// sysDLL loads a DLL by absolute System32 path, so a look-alike DLL sitting
// next to the executable can never be picked up.
func sysDLL(name string) *windows.LazyDLL {
	var buf [windows.MAX_PATH]uint16
	r, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemDirectoryW").
		Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 || r >= uintptr(len(buf)) {
		return windows.NewLazyDLL(name)
	}
	return windows.NewLazyDLL(windows.UTF16ToString(buf[:r]) + `\` + name)
}

var (
	dllKernel32 = sysDLL("kernel32.dll")
	dllUser32   = sysDLL("user32.dll")
	dllGdi32    = sysDLL("gdi32.dll")
	dllShell32  = sysDLL("shell32.dll")
	dllAdvapi32 = sysDLL("advapi32.dll")

	procCreateMutexW       = dllKernel32.NewProc("CreateMutexW")
	procGetModuleHandleW   = dllKernel32.NewProc("GetModuleHandleW")
	procGetModuleFileNameW = dllKernel32.NewProc("GetModuleFileNameW")
	procGetCurrentThreadID = dllKernel32.NewProc("GetCurrentThreadId")

	procRegisterClassExW       = dllUser32.NewProc("RegisterClassExW")
	procCreateWindowExW        = dllUser32.NewProc("CreateWindowExW")
	procDefWindowProcW         = dllUser32.NewProc("DefWindowProcW")
	procDestroyWindow          = dllUser32.NewProc("DestroyWindow")
	procPostQuitMessage        = dllUser32.NewProc("PostQuitMessage")
	procGetMessageW            = dllUser32.NewProc("GetMessageW")
	procTranslateMessage       = dllUser32.NewProc("TranslateMessage")
	procDispatchMessageW       = dllUser32.NewProc("DispatchMessageW")
	procPostMessageW           = dllUser32.NewProc("PostMessageW")
	procRegisterWindowMessageW = dllUser32.NewProc("RegisterWindowMessageW")
	procLoadIconW              = dllUser32.NewProc("LoadIconW")
	procCreatePopupMenu        = dllUser32.NewProc("CreatePopupMenu")
	procAppendMenuW            = dllUser32.NewProc("AppendMenuW")
	procTrackPopupMenu         = dllUser32.NewProc("TrackPopupMenu")
	procDestroyMenu            = dllUser32.NewProc("DestroyMenu")
	procGetCursorPos           = dllUser32.NewProc("GetCursorPos")
	procSetForegroundWindow    = dllUser32.NewProc("SetForegroundWindow")
	procSetProcessDPIAware     = dllUser32.NewProc("SetProcessDPIAware")
	procGetSystemMetrics       = dllUser32.NewProc("GetSystemMetrics")
	procCreateIconIndirect     = dllUser32.NewProc("CreateIconIndirect")
	procDestroyIcon            = dllUser32.NewProc("DestroyIcon")
	procGetForegroundWindow    = dllUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcID  = dllUser32.NewProc("GetWindowThreadProcessId")
	procGetKeyboardLayout      = dllUser32.NewProc("GetKeyboardLayout")
	procSetTimer               = dllUser32.NewProc("SetTimer")
	procKillTimer              = dllUser32.NewProc("KillTimer")

	procCreateCompatibleDC = dllGdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = dllGdi32.NewProc("CreateDIBSection")
	procDeleteObject       = dllGdi32.NewProc("DeleteObject")
	procDeleteDC           = dllGdi32.NewProc("DeleteDC")
	procCreateBitmap       = dllGdi32.NewProc("CreateBitmap")

	procShellNotifyIconW = dllShell32.NewProc("Shell_NotifyIconW")

	// advapi32
	procRegOpenKeyExW    = dllAdvapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = dllAdvapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW   = dllAdvapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW  = dllAdvapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = dllAdvapi32.NewProc("RegCloseKey")
)

const (
	wmDestroy       = 0x0002
	wmCommand       = 0x0111
	wmTimer         = 0x0113
	wmNull          = 0x0000
	wmLButtonUp     = 0x0202
	wmRButtonUp     = 0x0205
	wmContextMenu   = 0x007B
	wmDisplayChange = 0x007E
	wmDPIChanged    = 0x02E0
	wmApp           = 0x8000

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2

	nifMessage = 0x01
	nifIcon    = 0x02
	nifTip     = 0x04

	mfString = 0x0000
	mfSeparator = 0x0800
	mfChecked = 0x0008
	mfUnchecked = 0x0000

	tpmRightButton = 0x0002

	smCXSmIcon         = 49
	wsOverlapped       = 0x00000000

	biRGB        = 0
	dibRGBColors = 0

	// Registry constants
	hkeyCurrentUser = uintptr(0x80000001)
	keyQueryValue   = 0x0001
	keySetValue     = 0x0002
)

type point struct{ X, Y int32 }

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type wndClassEx struct {
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

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// notifyIconData is NOTIFYICONDATAW with the Vista+ tail (hBalloonIcon).
type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeout         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         guid
	HBalloonIcon     uintptr
}

type iconInfo struct {
	FIcon    uint32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [4]byte // RGBQUAD bmiColors[1]
}

// utf16Ptr converts a Go string to a NUL-terminated UTF-16 string. The result
// must be converted to uintptr inline in the Call argument list.
func utf16Ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil { // embedded NUL: cut the string there, like a C string
		for i := 0; i < len(s); i++ {
			if s[i] == 0 {
				p, _ = windows.UTF16PtrFromString(s[:i])
				break
			}
		}
	}
	return p
}

// copyUTF16 behaves like lstrcpynW(dst, src, len(dst)): at most len(dst)-1
// units are copied and the result is always NUL-terminated.
func copyUTF16(dst []uint16, s string) {
	if len(dst) == 0 {
		return
	}
	u := utf16.Encode([]rune(s))
	n := len(dst) - 1
	if len(u) > n {
		u = u[:n]
		if n > 0 && u[n-1] >= 0xD800 && u[n-1] < 0xDC00 {
			u = u[:n-1]
		}
	}
	copy(dst, u)
	dst[len(u)] = 0
	for i := len(u) + 1; i < len(dst); i++ {
		dst[i] = 0
	}
}

func loword(v uintptr) uint16 { return uint16(v) }

func postMessage(hwnd uintptr, message uint32, wparam, lparam uintptr) bool {
	r, _, _ := procPostMessageW.Call(hwnd, uintptr(message), wparam, lparam)
	return r != 0
}

func shellNotifyIcon(message uint32, nid *notifyIconData) bool {
	r, _, _ := procShellNotifyIconW.Call(uintptr(message), uintptr(unsafe.Pointer(nid)))
	return r != 0
}

// smallIconSize is the edge, in pixels, of a tray icon at the current DPI.
func smallIconSize() int {
	r, _, _ := procGetSystemMetrics.Call(smCXSmIcon)
	if sz := int(int32(r)); sz >= 16 {
		return sz
	}
	return 16
}

// iconFromBGRA turns sz x sz premultiplied BGRA pixels into an HICON (32-bit
// colour bitmap with alpha, plus an all-zero AND mask so alpha decides every
// pixel). Returns 0 on failure; the caller owns the icon.
func iconFromBGRA(sz int, pixels []byte) uintptr {
	if len(pixels) != sz*sz*4 {
		return 0
	}
	var bi bitmapInfo
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	bi.Header.Width = int32(sz)
	bi.Header.Height = -int32(sz) // top-down
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = biRGB

	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return 0
	}
	defer procDeleteDC.Call(dc)

	var bits unsafe.Pointer
	colorBmp, _, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if colorBmp == 0 || bits == nil {
		return 0
	}
	defer procDeleteObject.Call(colorBmp)
	copy(unsafe.Slice((*byte)(bits), len(pixels)), pixels)

	maskBits := make([]byte, (((sz+31)/32)*4)*sz)
	maskBmp, _, _ := procCreateBitmap.Call(uintptr(sz), uintptr(sz), 1, 1,
		uintptr(unsafe.Pointer(&maskBits[0])))
	if maskBmp == 0 {
		return 0
	}
	defer procDeleteObject.Call(maskBmp)

	info := iconInfo{FIcon: 1, HbmMask: maskBmp, HbmColor: colorBmp}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	return icon
}
