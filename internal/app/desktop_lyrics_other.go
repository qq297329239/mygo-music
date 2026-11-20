//go:build !windows

package app

// hideWindowBorder is nothing off Windows, whose window borders are the
// app's own to draw.
func hideWindowBorder(uintptr) {}

// keepOnTop is nothing off Windows, where the shell's windows rise over the
// bar of lyrics to put back down.
func keepOnTop(uintptr, bool) {}
