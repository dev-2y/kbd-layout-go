//go:build windows

package main

import "unsafe"

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
