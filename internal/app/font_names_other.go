//go:build !windows

package app

// localizedFontNames is nil off Windows, where the app doesn't read what
// fonts call themselves: families keep the names the system lists.
func localizedFontNames() map[string]string { return nil }
