package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOverviewRendersKeyFields(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetStatus(Status{
		Paired:         true,
		Backend:        "x11 (xclip)",
		Port:           10369,
		PhoneConnected: true,
		ClientCount:    1,
		SentCount:      3,
		ReceivedCount:  2,
		Uptime:         90 * time.Second,
	})

	out := m.Render(DefaultConfig())

	for _, want := range []string{
		"Paired", "yes",
		"Backend", "x11 (xclip)",
		"Port", "10369",
		"Phone", "connected",
		"clients: 1",
		"Sent", "3",
		"Received", "2",
		"Overview", "QR", "Activity",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("overview missing %q\n---\n%s", want, out)
		}
	}
	if !strings.Contains(out, "1m30s") {
		t.Errorf("overview missing formatted uptime, got:\n%s", out)
	}
}

func TestOverviewUnpaired(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetStatus(Status{Paired: false, Port: 0})
	out := m.Render(DefaultConfig())
	if !strings.Contains(out, "no") {
		t.Errorf("expected 'no' for unpaired, got:\n%s", out)
	}
	if !strings.Contains(out, "—") {
		t.Errorf("expected dash for zero port, got:\n%s", out)
	}
}

func TestQRTabPlaceholderWhenNoProvider(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetStatus(Status{Paired: true})
	m.SetTab(TabQR)
	out := m.Render(DefaultConfig())
	if !strings.Contains(out, "QR unavailable") {
		t.Errorf("expected QR placeholder, got:\n%s", out)
	}
}

func TestQRTabNotPaired(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetTab(TabQR)
	out := m.Render(DefaultConfig())
	if !strings.Contains(out, "Not paired yet") {
		t.Errorf("expected not-paired message, got:\n%s", out)
	}
}

func TestQRTabWithProvider(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetStatus(Status{Paired: true})
	m.SetTab(TabQR)
	cfg := DefaultConfig()
	cfg.QR = func() string { return "QRBODY" }
	out := m.Render(cfg)
	if !strings.Contains(out, "QRBODY") {
		t.Errorf("expected QR body, got:\n%s", out)
	}
}

func TestActivityTabEmpty(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetTab(TabActivity)
	out := m.Render(DefaultConfig())
	if !strings.Contains(out, "No activity yet") {
		t.Errorf("expected empty activity, got:\n%s", out)
	}
}

func TestActivityTabRecordsEvents(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetTab(TabActivity)
	m.PushEvent("first")
	m.PushEvent("second")
	out := m.Render(DefaultConfig())
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Errorf("expected both events, got:\n%s", out)
	}
	if !strings.Contains(out, "2 entries") {
		t.Errorf("expected entry count, got:\n%s", out)
	}
}

func TestActivityLogBounded(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetTab(TabActivity)
	for i := 0; i < maxLog+50; i++ {
		m.PushEvent("e")
	}
	if len(m.log) != maxLog {
		t.Errorf("log size = %d, want %d", len(m.log), maxLog)
	}
}

func TestStatusDeltasProduceEvents(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetStatus(Status{SentCount: 0, ReceivedCount: 0, PhoneConnected: false})
	m.SetStatus(Status{SentCount: 5, ReceivedCount: 2, PhoneConnected: true})
	m.SetTab(TabActivity)

	out := m.Render(DefaultConfig())
	for _, want := range []string{"sent clip (+5)", "received clip (+2)", "phone: connected"} {
		if !strings.Contains(out, want) {
			t.Errorf("activity missing %q\n---\n%s", want, out)
		}
	}
}

func TestTabSwitching(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	if m.Tab() != TabOverview {
		t.Fatalf("default tab = %v", m.Tab())
	}
	m.SetTab(TabQR)
	if m.Tab() != TabQR {
		t.Errorf("tab not set to QR")
	}
	m.SetTab(TabActivity)
	if m.Tab() != TabActivity {
		t.Errorf("tab not set to Activity")
	}
}

func TestScrollClamps(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	m.SetTab(TabActivity)
	for i := 0; i < 20; i++ {
		m.PushEvent("x")
	}
	m.Scroll(-100)
	if m.cursor != 0 {
		t.Errorf("cursor underflow = %d", m.cursor)
	}
	m.Scroll(1000)
	if m.cursor != len(m.log)-1 {
		t.Errorf("cursor overflow = %d, want %d", m.cursor, len(m.log)-1)
	}
}

func TestPollFromChannel(t *testing.T) {
	ch := make(chan Status, 4)
	m := NewModel(ch)
	ch <- Status{Paired: true, Port: 10369}
	if msg := m.poll(); msg == nil {
		t.Fatal("expected tickMsg after push")
	}
	if !m.status.Paired {
		t.Errorf("status not applied from channel")
	}
	if m.poll() != nil {
		t.Errorf("expected nil poll when channel empty")
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "5s"},
		{90 * time.Second, "1m30s"},
		{3661 * time.Second, "1h01m01s"},
		{-time.Second, "0s"},
	}
	for _, c := range cases {
		if got := formatUptime(c.d, time.Now()); got != c.want {
			t.Errorf("formatUptime(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestHandleKeyTabSwitching(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if m.Tab() != TabQR {
		t.Errorf("key '2' did not switch to QR tab")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if m.Tab() != TabActivity {
		t.Errorf("key '3' did not switch to Activity tab")
	}
}

func TestHandleKeyArrows(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Tab() != TabQR {
		t.Errorf("right arrow did not advance tab")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.Tab() != TabActivity {
		t.Errorf("left arrow wrap-around failed")
	}
}

func TestHandleKeyQuit(t *testing.T) {
	m := NewModel(make(chan Status, 4))
	cmd := func() {
		_, c := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		if c == nil {
			t.Errorf("expected quit command")
		}
	}
	cmd()
}
