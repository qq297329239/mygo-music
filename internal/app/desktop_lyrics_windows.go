package app

import (
	"syscall"
	"unsafe"
)

// procDwmSetWindowAttribute is DWM's DwmSetWindowAttribute, which restyles
// what it draws around a window.
var procDwmSetWindowAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")

// hideWindowBorder has DWM draw nothing of the window outside its content:
// it would paint a hairline around a window that resizes, and the pixel of
// caption Windows 11 keeps on one without a title bar.
func hideWindowBorder(hwnd uintptr) {
	disabled := int32(1) // DWMNCRP_DISABLED
	procDwmSetWindowAttribute.Call(hwnd, 2 /* DWMWA_NCRENDERING_POLICY */, uintptr(unsafe.Pointer(&disabled)), 4)
	none := uint32(0xFFFFFFFE) // DWMWA_COLOR_NONE: no border, not the accent
	procDwmSetWindowAttribute.Call(hwnd, 34 /* DWMWA_BORDER_COLOR */, uintptr(unsafe.Pointer(&none)), 4)
}
