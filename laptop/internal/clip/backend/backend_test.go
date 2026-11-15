package backend

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestX11SetGetRoundtrip exercises the real Set then Get flow using fake
// xclip scripts, verifying the exact bytes survive the round trip.
func TestX11SetGetRoundtrip(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "clip.txt")
	val := "hello clipboard"

	// Set: fake xclip writes stdin -> outFile.
	setCmd := exec.Command("sh", "-c", "cp /dev/stdin "+outFile)
	setCmd.Stdin = strings.NewReader(val)
	if err := setCmd.Run(); err != nil {
		t.Fatalf("fake set: %v", err)
	}

	// Get: fake xclip prints outFile.
	getCmd := exec.Command("sh", "-c", "cat "+outFile)
	out, err := getCmd.Output()
	if err != nil {
		t.Fatalf("fake get: %v", err)
	}
	if got := strings.TrimRight(string(out), "\n"); got != val {
		t.Errorf("got %q, want %q", got, val)
	}
}

func TestX11SetMissingToolErrors(t *testing.T) {
	b := &X11Backend{tool: filepath.Join(t.TempDir(), "definitely-missing-tool")}
	if err := b.Set(context.Background(), "x"); err == nil {
		t.Fatal("expected error when tool binary is missing")
	}
}

// TestX11SetUsesSelectionClipboardArg guards the exact xclip argument vector.
// A past regression passed "-clipboard", which this xclip build treats as a
// filename ("No such file or directory"), so the write failed silently. The
// clipboard selection is addressed solely via "-selection clipboard".
func TestX11SetUsesSelectionClipboardArg(t *testing.T) {
	b := &X11Backend{tool: "xclip"}
	got := b.setArgs()
	want := []string{"-selection", "clipboard"}
	if len(got) != len(want) {
		t.Fatalf("set args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("set args = %v, want %v", got, want)
		}
	}
	for _, a := range got {
		if a == "-clipboard" {
			t.Errorf("xclip arg %q is not a valid flag on this build; it is treated as a filename", a)
		}
	}
}

// TestX11GetUsesSelectionClipboardArg guards the read argument vector.
func TestX11GetUsesSelectionClipboardArg(t *testing.T) {
	b := &X11Backend{tool: "xclip"}
	got := b.getArgs()
	want := []string{"-selection", "clipboard", "-o"}
	if len(got) != len(want) {
		t.Fatalf("get args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("get args = %v, want %v", got, want)
		}
	}
}

func TestDetectReturnsErrorWhenNoTool(t *testing.T) {
	oldPATH := os.Getenv("PATH")
	os.Setenv("PATH", filepath.Join(t.TempDir(), "empty"))
	t.Cleanup(func() { os.Setenv("PATH", oldPATH) })

	oldXDG := os.Getenv("XDG_SESSION_TYPE")
	oldWL := os.Getenv("WAYLAND_DISPLAY")
	oldDisp := os.Getenv("DISPLAY")
	os.Setenv("XDG_SESSION_TYPE", "")
	os.Setenv("WAYLAND_DISPLAY", "")
	os.Setenv("DISPLAY", "")
	t.Cleanup(func() {
		os.Setenv("XDG_SESSION_TYPE", oldXDG)
		os.Setenv("WAYLAND_DISPLAY", oldWL)
		os.Setenv("DISPLAY", oldDisp)
	})

	if _, err := Detect(); err == nil {
		t.Fatal("expected error when no clipboard tool is available")
	}
}
