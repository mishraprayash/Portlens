package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

func TestVisibleLenAndPad(t *testing.T) {
	plain := "hello world"
	if visibleLen(plain) != 11 {
		t.Errorf("expected 11, got %d", visibleLen(plain))
	}

	colored := "\x1b[32mhello\x1b[0m world"
	if visibleLen(colored) != 11 {
		t.Errorf("expected 11 visible runes, got %d", visibleLen(colored))
	}

	padded := padRight(colored, 15)
	if visibleLen(padded) != 15 {
		t.Errorf("expected padded visible len 15, got %d", visibleLen(padded))
	}

	paddedL := padLeft(colored, 15)
	if visibleLen(paddedL) != 15 {
		t.Errorf("expected padded left visible len 15, got %d", visibleLen(paddedL))
	}
}

func TestTruncateVisible(t *testing.T) {
	plain := "1234567890"
	tr := truncateVisible(plain, 5)
	if tr != "12345" {
		t.Errorf("expected '12345', got %q", tr)
	}

	colored := "\x1b[31m1234567890\x1b[0m"
	trColored := truncateVisible(colored, 5)
	if visibleLen(trColored) != 5 {
		t.Errorf("expected visible len 5, got %d (%q)", visibleLen(trColored), trColored)
	}
}

func TestRenderLayouts(t *testing.T) {
	m := NewModel(false)
	m.Width = 100
	m.Height = 25
	m.SetEntries(sampleEntries())

	// Split pane render
	out := stripANSI(Render(m))
	if !strings.Contains(out, "PORTLENS TOP") {
		t.Errorf("expected PORTLENS TOP in output")
	}
	if !strings.Contains(out, "3000") {
		t.Errorf("expected port 3000 in output")
	}

	// Narrow terminal (<80 columns)
	m.Width = 70
	outNarrow := stripANSI(Render(m))
	if !strings.Contains(outNarrow, "PORTLENS TOP") {
		t.Errorf("expected header in narrow output")
	}

	// Tiny terminal (<50 columns)
	m.Width = 40
	m.Height = 8
	outTiny := stripANSI(Render(m))
	if !strings.Contains(outTiny, "too small") {
		t.Errorf("expected 'too small' warning for tiny terminal")
	}
}

func TestRenderHelpAndConfirmOverlays(t *testing.T) {
	m := NewModel(false)
	m.Width = 100
	m.Height = 25
	m.SetEntries(sampleEntries())

	// Test Help Mode
	m.ViewMode = ModeHelp
	outHelp := stripANSI(Render(m))
	if !strings.Contains(outHelp, "SHORTCUTS") {
		t.Errorf("expected SHORTCUTS in help overlay")
	}

	// Test Confirm Mode
	m.ViewMode = ModeConfirm
	m.ConfirmPrompt = "Kill process 1024 (node)?"
	outConfirm := stripANSI(Render(m))
	if !strings.Contains(outConfirm, "CONFIRM ACTION") {
		t.Errorf("expected CONFIRM ACTION in confirm overlay")
	}
	if !strings.Contains(outConfirm, "Kill process 1024 (node)?") {
		t.Errorf("expected confirm prompt in overlay")
	}
}

func TestRenderTabs(t *testing.T) {
	m := NewModel(false)
	m.Width = 100
	m.Height = 25
	m.SetEntries(sampleEntries())
	// sampleEntries[2] is port 3000
	m.SelectedIdx = 2
	m.SelectedReport = &model.Report{
		Port:     3000,
		Protocol: model.ProtocolTCP,
		Status:   "listening",
		Address:  "127.0.0.1",
		Process: &model.ProcessInfo{
			PID:     1024,
			Name:    "node",
			Command: "node server.js",
		},
		Exposure: &model.Exposure{
			Worst: model.RiskLow,
		},
		Network: &model.NetworkInfo{
			Connections: []model.Connection{
				{Protocol: model.ProtocolTCP, LocalAddr: "127.0.0.1", LocalPort: 3000, RemoteAddr: "127.0.0.1", RemotePort: 54321, State: "ESTABLISHED"},
			},
		},
	}

	// Tab 1: Overview
	m.DetailTab = TabOverview
	outOverview := stripANSI(Render(m))
	if !strings.Contains(outOverview, "node (PID") {
		t.Errorf("expected process info in overview: %s", outOverview)
	}

	// Tab 2: Tree
	m.DetailTab = TabTree
	outTree := stripANSI(Render(m))
	if !strings.Contains(outTree, "Process Hierarchy") {
		t.Errorf("expected Process Hierarchy in tree tab: %s", outTree)
	}

	// Tab 3: Connections
	m.DetailTab = TabConnections
	outConn := stripANSI(Render(m))
	if !strings.Contains(outConn, "ESTABLISHED") {
		t.Errorf("expected ESTABLISHED in connections tab: %s", outConn)
	}
}

func TestRenderFilterFooter(t *testing.T) {
	m := NewModel(false)
	m.Width = 100
	m.Height = 25
	m.ViewMode = ModeFilter
	m.Filter = "node"

	out := stripANSI(Render(m))
	if !strings.Contains(out, "Filter: node_") {
		t.Errorf("expected 'Filter: node_' in footer when in filter mode: %s", out)
	}

	// Status message
	m.ViewMode = ModeNormal
	m.SetStatus("Action successful", false, 5*time.Second)
	outStatus := stripANSI(Render(m))
	if !strings.Contains(outStatus, "Action successful") {
		t.Errorf("expected status message in footer: %s", outStatus)
	}
}
