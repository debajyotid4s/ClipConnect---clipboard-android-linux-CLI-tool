package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Backend reads and writes the system clipboard.
type Backend interface {
	// Get returns the current clipboard text.
	Get(ctx context.Context) (string, error)
	// Set writes text to the clipboard.
	Set(ctx context.Context, text string) error
}

// Detect returns the appropriate backend for the current session.
func Detect() (Backend, error) {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	display := os.Getenv("DISPLAY")

	switch sessionType {
	case "wayland":
		if waylandDisplay != "" {
			return NewWayland()
		}
	case "x11", "tty":
		fallthrough
	default:
		if display != "" || sessionType == "x11" {
			return NewX11()
		}
	}

	// Fallback: try wl-paste first, then xclip
	if _, err := exec.LookPath("wl-paste"); err == nil {
		return NewWayland()
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return NewX11()
	}

	return nil, fmt.Errorf("no clipboard tool found (need wl-clipboard or xclip)")
}

// waylandBackend uses wl-paste/wl-copy for Wayland.
type WaylandBackend struct{}

func NewWayland() (*WaylandBackend, error) {
	if _, err := exec.LookPath("wl-paste"); err != nil {
		return nil, fmt.Errorf("wl-paste not found: install wl-clipboard")
	}
	return &WaylandBackend{}, nil
}

func (b *WaylandBackend) Get(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "wl-paste", "--no-newline")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("wl-paste get: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (b *WaylandBackend) Set(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "wl-copy", "--type", "text/plain")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("wl-copy start: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("wl-copy wait: %w", err)
	}
	return nil
}

// x11Backend uses xclip/xsel for X11.
type X11Backend struct {
	tool string // "xclip" or "xsel"
}

func NewX11() (*X11Backend, error) {
	if _, err := exec.LookPath("xsel"); err == nil {
		return &X11Backend{tool: "xsel"}, nil
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return &X11Backend{tool: "xclip"}, nil
	}
	return nil, fmt.Errorf("neither xsel nor xclip found")
}

func (b *X11Backend) Get(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, b.tool, b.getArgs()...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s get: %w", b.tool, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (b *X11Backend) Set(ctx context.Context, text string) error {
	// xclip can hang if the X clipboard owner changes or a prior instance is
	// stuck holding the selection. Bound the write with a timeout and retry
	// once so an incoming clip is reliably pasted.
	for attempt := 0; attempt < 2; attempt++ {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cmd := exec.CommandContext(cctx, b.tool, b.setArgs()...)
		cmd.Stdin = strings.NewReader(text)
		err := cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
		// Brief pause so a retried xclip isn't blocked by a still-released lock.
		if attempt == 0 {
			time.Sleep(150 * time.Millisecond)
		}
	}
	return fmt.Errorf("%s set: %w", b.tool, context.DeadlineExceeded)
}

// getArgs returns the xclip/xsel flags for reading the clipboard selection.
func (b *X11Backend) getArgs() []string {
	switch b.tool {
	case "xsel":
		return []string{"--clipboard", "--output"}
	case "xclip":
		return []string{"-selection", "clipboard", "-o"}
	}
	return nil
}

// setArgs returns the xclip/xsel flags for writing the clipboard selection.
// The clipboard selection is addressed solely via "-selection clipboard"
// (xclip has no standalone "-clipboard" flag on modern builds; passing it is
// interpreted as a filename and fails silently).
func (b *X11Backend) setArgs() []string {
	switch b.tool {
	case "xsel":
		return []string{"--clipboard", "--input", "--force"}
	case "xclip":
		return []string{"-selection", "clipboard"}
	}
	return nil
}
