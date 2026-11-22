package backend

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// WaylandBackend uses wl-paste/wl-copy for Wayland.
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
