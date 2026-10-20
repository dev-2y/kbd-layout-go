//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	regSZ  = 1
)

// ---------------------------------------------------------------- autostart

// autostartEnabled checks if the app is registered to run at startup.
func autostartEnabled() bool {
	var key uintptr
	if r, _, _ := procRegOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(utf16Ptr(runKey))),
		0, keyQueryValue, uintptr(unsafe.Pointer(&key))); r != 0 {
		return false
	}
	defer procRegCloseKey.Call(key)

	status, _, _ := procRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(appName))),
		0, 0, 0, 0)
	return status == 0
}

// autostartToggle adds or removes the app from the Windows startup registry.
func autostartToggle() {
	var key uintptr
	if r, _, _ := procRegOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(utf16Ptr(runKey))),
		0, keySetValue, uintptr(unsafe.Pointer(&key))); r != 0 {
		return
	}
	defer procRegCloseKey.Call(key)

	if autostartEnabled() {
		procRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(appName))))
		return
	}

	if exe := moduleFileName(); exe != "" {
		quoted, _ := syscall.UTF16FromString(`"` + exe + `"`)
		procRegSetValueExW.Call(key, uintptr(unsafe.Pointer(utf16Ptr(appName))), 0, regSZ,
			uintptr(unsafe.Pointer(&quoted[0])), uintptr(len(quoted)*2))
	}
}

// moduleFileName returns the full path to the current executable.
func moduleFileName() string {
	var buf [syscall.MAX_PATH]uint16
	r, _, _ := procGetModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 || r >= uintptr(len(buf)) {
		return ""
	}
	return syscall.UTF16ToString(buf[:r])
}
