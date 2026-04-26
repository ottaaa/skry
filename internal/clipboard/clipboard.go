// Package clipboard wraps the two paths a TUI app can take to copy text:
//
//  1. OSC 52 — an escape sequence (ESC ]52;c;<base64>BEL) that asks the
//     terminal emulator itself to put data on the local clipboard. Works
//     over SSH and inside tmux/screen with the right config; works in
//     iTerm2, kitty, Alacritty, WezTerm, modern Apple Terminal, etc.
//
//  2. atotto/clipboard — shells out to pbcopy / xclip / wl-copy. Works on
//     local desktop sessions; fails on headless / SSH targets without
//     DISPLAY.
//
// We fire both unconditionally so the user gets the paste in whichever
// channel is reachable. Errors from atotto are swallowed: when it fails,
// OSC 52 has almost always already done the job, and surfacing a "copy
// failed" toast would be misleading.
package clipboard

import (
	"github.com/atotto/clipboard"
	"github.com/muesli/termenv"
)

// Copy writes s to the system clipboard via OSC 52 + atotto best-effort.
// Always returns nil; kept on the signature so future callers can extend
// without a breaking API change.
func Copy(s string) error {
	termenv.Copy(s)
	_ = clipboard.WriteAll(s)
	return nil
}
