package app

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
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

// The shell's windows — the taskbar and the menus and panels it pops — are
// topmost like the bar of lyrics, and working any of them lifts those over
// the bar again. Only a new song would otherwise restate the bar's place at
// the top of the stack. The bar restates it itself: on the foreground
// changing, on windows coming and going, and on a slow look in between.

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procSetWindowPos    = user32.NewProc("SetWindowPos")
	procSetWinEventHook = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent  = user32.NewProc("UnhookWinEvent")
)

// swpNo* tell SetWindowPos to move nothing but the window's place in the
// stack.
const (
	swpNoSize             = 0x0001
	swpNoMove             = 0x0002
	swpNoActivate         = 0x0010
	hwndTopmost           = ^uintptr(0) // -1
	eventSystemForeground = 0x0003
	eventObjectHide       = 0x8001
	eventObjectShow       = 0x8002
	objidWindow           = 0
)

// aboveProc, aboveHook and aboveTicker are the watch that keeps the bar of
// lyrics at the top of the stack of windows on top: a hook on the foreground
// changing and on windows showing and hiding, as when the listener works
// the taskbar's icons, its start menu or its tray's panels, and a slow look
// for any of them quietly keeping the bar down.
var (
	aboveHWND   uintptr
	aboveHook   uintptr
	aboveTicker *time.Ticker
	aboveProc   = syscall.NewCallback(func(hook, event, ehwnd uintptr, idObject, idChild int32, thread, ms uintptr) uintptr {
		if idObject == objidWindow {
			mygo.RunOnMain(func() { restateOnTop(aboveHWND) })
		}
		return 0
	})
)

// keepOnTop watches while on, on the main thread: each time the shell lifts
// one of its windows over the bar, it restates the bar's place at the top.
func keepOnTop(hwnd uintptr, on bool) {
	if on {
		aboveHWND = hwnd
		// The hook rides the main thread's message loop, where it is set.
		if aboveHook == 0 {
			aboveHook, _, _ = procSetWinEventHook.Call(eventSystemForeground, eventObjectShow, 0, aboveProc, 0, 0, 0)
		}
		if aboveTicker == nil {
			aboveTicker = time.NewTicker(time.Second)
			go func() {
				for range aboveTicker.C {
					mygo.RunOnMain(func() { restateOnTop(aboveHWND) })
				}
			}()
		}
		restateOnTop(hwnd)
		return
	}
	aboveHWND = 0
	if aboveTicker != nil {
		aboveTicker.Stop()
		aboveTicker = nil
	}
	if aboveHook != 0 {
		procUnhookWinEvent.Call(aboveHook)
		aboveHook = 0
	}
}

// restateOnTop puts the window back at the top of the stack of windows on
// top, the move a new song's bar showing makes and the only one the shell
// leaves the bar: its own windows rise over the bar whenever they are
// worked.
func restateOnTop(hwnd uintptr) {
	if hwnd != 0 {
		procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate)
	}
}
