package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Backend reads and writes the system clipboard.
type Backend interface {
	// Get returns the current clipboard text.
	Get(ctx context.Context) (string, error)
	// Set writes text to the clipboard.
	Set(ctx context.Context, text string) error
}

// Detect returns the appropriate backend for the current session.
//
// Strategy (most specific first, then capability-based fallback):
//  1. A Wayland session with wl-copy/wl-paste present -> Wayland backend.
//  2. An X11/TTY session with xclip/xsel present -> X11 backend.
//  3. Capability probe: wl-paste available (works on Wayland AND on Xwayland
//     X11 sessions) -> Wayland backend.
//  4. Capability probe: xclip/xsel available -> X11 backend.
//
// The key fix: a Wayland session WITHOUT wl-clipboard must NOT hard-fail. Some
// Wayland setups (or minimal installs) only have xclip/xsel running under Xwayland,
// so we fall back to the X11 backend instead of returning "no clipboard tool".
func Detect() (Backend, error) {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	display := os.Getenv("DISPLAY")

	// 1. Wayland session that can actually use wl-copy/wl-paste.
	if sessionType == "wayland" && waylandDisplay != "" {
		if _, err := exec.LookPath("wl-paste"); err == nil {
			return NewWayland()
		}
		// Wayland session but no wl-clipboard: fall through to capability probe
		// (xclip/xsel under Xwayland) rather than failing outright.
	}

	// 2. X11 / TTY / unknown session with an X server and xclip/xsel.
	if display != "" || sessionType == "x11" || sessionType == "tty" || sessionType == "" {
		if _, err := exec.LookPath("xsel"); err == nil {
			return &X11Backend{tool: "xsel"}, nil
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			return &X11Backend{tool: "xclip"}, nil
		}
	}

	// 3. Capability probe: wl-paste works on both Wayland and Xwayland.
	if _, err := exec.LookPath("wl-paste"); err == nil {
		return NewWayland()
	}

	return nil, fmt.Errorf("no clipboard tool found (need wl-clipboard or xclip/xsel)")
}
