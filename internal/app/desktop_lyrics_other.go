//go:build !windows

package app

// hideWindowBorder is nothing off Windows, whose window borders are the
// app's own to draw.
func hideWindowBorder(uintptr) {}
