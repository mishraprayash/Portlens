package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/portlens/portlens/internal/actions"
	"github.com/portlens/portlens/internal/model"
	"github.com/portlens/portlens/internal/version"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI color and formatting escape sequences from s.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// visibleLen calculates the displayed column width of s ignoring ANSI escape sequences.
func visibleLen(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

// padRight pads s with spaces so its visible length equals width.
func padRight(s string, width int) string {
	vl := visibleLen(s)
	if vl >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vl)
}

// padLeft pads s on the left with spaces so its visible length equals width.
func padLeft(s string, width int) string {
	vl := visibleLen(s)
	if vl >= width {
		return s
	}
	return strings.Repeat(" ", width-vl) + s
}

// truncateVisible trims s so that its visible length does not exceed maxLen.
// If s is truncated and contains ANSI sequences, a reset sequence is appended.
func truncateVisible(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if visibleLen(s) <= maxLen {
		return s
	}

	hasANSI := strings.Contains(s, "\x1b[")
	var b strings.Builder
	curVisible := 0
	inEsc := false

	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			inEsc = true
			b.WriteByte(s[i])
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if inEsc {
			b.WriteByte(s[i])
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if curVisible+1 > maxLen {
			break
		}
		curVisible++
		b.WriteRune(r)
		i += size
	}

	if hasANSI {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

// ANSI styling helpers
func bold(s string) string    { return "\x1b[1m" + s + "\x1b[0m" }
func dim(s string) string     { return "\x1b[2m" + s + "\x1b[0m" }
func cyan(s string) string    { return "\x1b[36m" + s + "\x1b[0m" }
func green(s string) string   { return "\x1b[32m" + s + "\x1b[0m" }
func yellow(s string) string  { return "\x1b[33m" + s + "\x1b[0m" }
func red(s string) string     { return "\x1b[31m" + s + "\x1b[0m" }
func magenta(s string) string { return "\x1b[35m" + s + "\x1b[0m" }
func inverse(s string) string { return "\x1b[7m" + s + "\x1b[0m" }

// Render produces the complete screen frame for the terminal.
func Render(m *Model) string {
	w := m.Width
	h := m.Height
	if w < 50 || h < 10 {
		return renderTooSmall(w, h)
	}

	lines := make([]string, h)

	// 1. Header (Line 0)
	lines[0] = renderHeader(m, w)

	// 2. Header separator (Line 1)
	lines[1] = dim(strings.Repeat("─", w))

	// 3. Body lines (Lines 2 to h - 3)
	bodyHeight := h - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var leftLines, rightLines []string
	split := w >= 80

	if split {
		leftWidth := w * 42 / 100
		if leftWidth < 38 {
			leftWidth = 38
		}
		if leftWidth > 55 {
			leftWidth = 55
		}
		rightWidth := w - leftWidth - 1 // 1 column for divider '│'

		leftLines = renderLeftPane(m, leftWidth, bodyHeight)
		rightLines = renderRightPane(m, rightWidth, bodyHeight)

		for i := 0; i < bodyHeight; i++ {
			var l, r string
			if i < len(leftLines) {
				l = leftLines[i]
			} else {
				l = strings.Repeat(" ", leftWidth)
			}
			if i < len(rightLines) {
				r = rightLines[i]
			} else {
				r = strings.Repeat(" ", rightWidth)
			}
			lines[2+i] = padRight(l, leftWidth) + dim("│") + padRight(r, rightWidth)
		}
	} else {
		// Single pane layout
		leftLines = renderLeftPane(m, w, bodyHeight)
		for i := 0; i < bodyHeight; i++ {
			if i < len(leftLines) {
				lines[2+i] = padRight(leftLines[i], w)
			} else {
				lines[2+i] = strings.Repeat(" ", w)
			}
		}
	}

	// 4. Footer separator (Line h - 2)
	lines[h-2] = dim(strings.Repeat("─", w))

	// 5. Footer / Status bar (Line h - 1)
	lines[h-1] = renderFooter(m, w)

	// 6. Modal Popups (Confirm dialog or Help overlay)
	if m.ViewMode == ModeHelp {
		overlayHelp(lines, w, h)
	} else if m.ViewMode == ModeConfirm {
		overlayConfirm(lines, m, w, h)
	}

	// Build final output string with cursor home
	var sb strings.Builder
	sb.WriteString("\x1b[H") // Move to home
	for i, line := range lines {
		// Truncate to terminal width to guarantee no line wrap artifacts
		sb.WriteString(truncateVisible(line, w))
		if i < len(lines)-1 {
			sb.WriteString("\r\n")
		}
	}
	sb.WriteString("\x1b[J") // Clear remaining screen space if any
	return sb.String()
}

func renderTooSmall(w, h int) string {
	msg := fmt.Sprintf("Terminal window too small (%dx%d). Please enlarge (min: 50x10).", w, h)
	return fmt.Sprintf("\x1b[H\x1b[J\r\n  %s\r\n", yellow(msg))
}

func renderHeader(m *Model, w int) string {
	title := bold(cyan("PORTLENS TOP")) + dim(" v"+version.Version)
	count := fmt.Sprintf("Sockets: %s", bold(strconv.Itoa(len(m.FilteredIndices))))
	if len(m.FilteredIndices) != len(m.Entries) {
		count = fmt.Sprintf("Sockets: %s/%d", bold(strconv.Itoa(len(m.FilteredIndices))), len(m.Entries))
	}

	filterText := ""
	if m.Filter != "" || m.ViewMode == ModeFilter {
		fVal := m.Filter
		if m.ViewMode == ModeFilter {
			fVal += "_"
		}
		filterText = fmt.Sprintf("Filter: [%s]", cyan(bold(fVal)))
	}

	clock := dim(m.LastUpdated.Format("15:04:05"))
	helpQuit := fmt.Sprintf("%s Help  %s Quit", bold("[?]"), bold("[q]"))

	parts := []string{title, count}
	if filterText != "" {
		parts = append(parts, filterText)
	}
	parts = append(parts, clock, helpQuit)

	leftPart := strings.Join(parts[:len(parts)-1], dim(" │ "))
	rightPart := parts[len(parts)-1]

	sp := w - visibleLen(leftPart) - visibleLen(rightPart)
	if sp < 1 {
		return truncateVisible(leftPart+" "+rightPart, w)
	}
	return leftPart + strings.Repeat(" ", sp) + rightPart
}

func renderLeftPane(m *Model, width, height int) []string {
	var lines []string

	// Table header
	portW := 6
	protoW := 5
	pidW := 7
	procW := 12
	if width > 45 {
		procW = 15
	}
	srvW := width - 2 - portW - protoW - pidW - procW - 4
	if srvW < 6 {
		srvW = 6
	}

	hdr := fmt.Sprintf("  %s %s %s %s %s",
		padRight("PORT", portW),
		padRight("PROTO", protoW),
		padRight("PROCESS", procW),
		padLeft("PID", pidW),
		padRight("SERVICE", srvW),
	)
	lines = append(lines, bold(hdr))

	dataRows := height - 1
	if dataRows <= 0 {
		return lines
	}

	m.EnsureVisible(dataRows)

	if len(m.FilteredIndices) == 0 {
		msg := dim("  No listening ports match.")
		if len(m.Entries) == 0 {
			msg = dim("  Scanning for active listeners...")
		}
		lines = append(lines, msg)
		return lines
	}

	endIdx := m.ScrollOffset + dataRows
	if endIdx > len(m.FilteredIndices) {
		endIdx = len(m.FilteredIndices)
	}

	for i := m.ScrollOffset; i < endIdx; i++ {
		entryIdx := m.FilteredIndices[i]
		e := m.Entries[entryIdx]
		isSelected := (i == m.SelectedIdx)

		marker := "  "
		if isSelected {
			marker = cyan(bold("❯ "))
		}

		portStr := strconv.Itoa(int(e.Port))
		protoStr := string(e.Protocol)
		procStr := truncateVisible(e.Process, procW)
		pidStr := "-"
		if e.PID > 0 {
			pidStr = strconv.Itoa(int(e.PID))
		}
		srvStr := e.Service
		if srvStr == "" {
			srvStr = "-"
		}
		srvStr = truncateVisible(srvStr, srvW)

		row := fmt.Sprintf("%s%s %s %s %s %s",
			marker,
			padRight(portStr, portW),
			padRight(protoStr, protoW),
			padRight(procStr, procW),
			padLeft(pidStr, pidW),
			padRight(srvStr, srvW),
		)

		if isSelected {
			row = inverse(padRight(marker+" "+padRight(portStr, portW)+" "+padRight(protoStr, protoW)+" "+padRight(procStr, procW)+" "+padLeft(pidStr, pidW)+" "+padRight(srvStr, srvW), width))
		}
		lines = append(lines, row)
	}

	return lines
}

func renderRightPane(m *Model, width, height int) []string {
	var lines []string

	// Tabs Header
	tab1 := "[1:Overview]"
	tab2 := "[2:Tree]"
	tab3 := "[3:Connections]"

	switch m.DetailTab {
	case TabOverview:
		tab1 = bold(cyan(tab1))
		tab2 = dim(tab2)
		tab3 = dim(tab3)
	case TabTree:
		tab1 = dim(tab1)
		tab2 = bold(cyan(tab2))
		tab3 = dim(tab3)
	case TabConnections:
		tab1 = dim(tab1)
		tab2 = dim(tab2)
		tab3 = bold(cyan(tab3))
	}

	tabsLine := fmt.Sprintf(" %s  %s  %s", tab1, tab2, tab3)
	lines = append(lines, tabsLine)
	lines = append(lines, dim(strings.Repeat("─", width)))

	sel := m.SelectedEntry()
	if sel == nil {
		lines = append(lines, dim(" No listener selected."))
		return lines
	}

	rep := m.SelectedReport
	if rep == nil || rep.Port != sel.Port {
		// Use basic PortEntry info while report is loading
		lines = append(lines, fmt.Sprintf(" %s Port %d (%s)", bold("Inspecting:"), sel.Port, sel.Protocol))
		lines = append(lines, fmt.Sprintf(" %s  %s (PID %d)", dim("Process:"), bold(sel.Process), sel.PID))
		lines = append(lines, fmt.Sprintf(" %s  %s", dim("Address:"), sel.Address))
		if sel.Service != "" {
			lines = append(lines, fmt.Sprintf(" %s  %s", dim("Service:"), sel.Service))
		}
		lines = append(lines, "")
		lines = append(lines, dim(" Loading full inspection details..."))
		return lines
	}

	switch m.DetailTab {
	case TabOverview:
		lines = append(lines, renderOverviewTab(rep, width)...)
	case TabTree:
		lines = append(lines, renderTreeTab(rep, width)...)
	case TabConnections:
		lines = append(lines, renderConnectionsTab(rep, width)...)
	}

	return lines
}

func renderOverviewTab(rep *model.Report, width int) []string {
	var lines []string

	statusText := green(bold("LISTENING"))
	if rep.Status != "listening" {
		statusText = yellow(rep.Status)
	}
	lines = append(lines, fmt.Sprintf(" %s %-12s %s %s:%d (%s)",
		dim("Status:"), statusText,
		dim("Address:"), rep.Address, rep.Port, rep.Protocol))

	// Exposure Assessment
	if rep.Exposure != nil {
		expText := green("LOW RISK (localhost only)")
		if rep.Exposure.Worst == model.RiskDangerous {
			expText = red(bold("POTENTIALLY DANGEROUS - bound to public interface"))
		} else if rep.Exposure.Worst == model.RiskWarning {
			expText = yellow(bold("WARNING - wildcard bind 0.0.0.0"))
		}
		lines = append(lines, fmt.Sprintf(" %s %s", dim("Exposure:"), expText))
	}

	// Process details
	if rep.Process != nil {
		p := rep.Process
		lines = append(lines, fmt.Sprintf(" %s %s (PID %s, User: %s)",
			dim("Process:"), bold(p.Name), cyan(strconv.Itoa(int(p.PID))), p.User))

		if p.MemoryBytes > 0 || !p.StartTime.IsZero() {
			memStr := model.FormatBytes(p.MemoryBytes)
			upStr := p.Runtime(time.Now())
			lines = append(lines, fmt.Sprintf(" %s %s │ %s %s",
				dim("Memory:"), memStr, dim("Uptime:"), upStr))
		}

		if p.Command != "" {
			lines = append(lines, fmt.Sprintf(" %s %s", dim("Command:"), truncateVisible(p.Command, width-11)))
		}
		if p.CWD != "" {
			lines = append(lines, fmt.Sprintf(" %s %s", dim("CWD:"), truncateVisible(p.CWD, width-7)))
		}
	}

	// Project & Git
	if rep.Project != nil && rep.Project.Detected {
		pj := rep.Project
		projDesc := pj.Name
		if pj.Framework != "" {
			projDesc += " (" + pj.Framework + ")"
		}
		if pj.GitBranch != "" {
			projDesc += " [git: " + cyan(pj.GitBranch) + "]"
		}
		lines = append(lines, fmt.Sprintf(" %s %s", dim("Project:"), truncateVisible(projDesc, width-11)))
	}

	// Container
	if rep.Container != nil {
		c := rep.Container
		lines = append(lines, fmt.Sprintf(" %s %s (image: %s)",
			dim("Container:"), bold(c.Name), c.Image))
	}

	// HTTP Probe
	if rep.HTTPProbe != nil {
		lines = append(lines, fmt.Sprintf(" %s %s (%s, latency: %s)",
			dim("HTTP:"), rep.HTTPProbe.Status, rep.HTTPProbe.Title, rep.HTTPProbe.Latency.Round(time.Millisecond)))
	}

	// Quick actions tip
	localURL := actions.LocalURL(rep)
	lines = append(lines, "")
	lines = append(lines, dim(fmt.Sprintf(" Local URL: %s", cyan(localURL))))

	return lines
}

func renderTreeTab(rep *model.Report, width int) []string {
	var lines []string
	lines = append(lines, bold(" Process Hierarchy:"))

	if rep.Process == nil {
		lines = append(lines, dim("  No process information available."))
		return lines
	}

	// Ancestors
	for _, a := range rep.Ancestors {
		lines = append(lines, fmt.Sprintf("  %s %s (PID %d)", dim("├──"), a.Name, a.PID))
	}

	// Current target process
	targetPrefix := "  └── "
	if len(rep.Children) > 0 {
		targetPrefix = "  ├── "
	}
	lines = append(lines, fmt.Sprintf("%s%s %s",
		targetPrefix,
		cyan(bold(rep.Process.Name)),
		dim(fmt.Sprintf("(PID %d) ← owns port %d", rep.Process.PID, rep.Port))))

	// Descendants / Children
	for i, c := range rep.Children {
		childPrefix := "  │   ├── "
		if i == len(rep.Children)-1 {
			childPrefix = "  │   └── "
		}
		lines = append(lines, fmt.Sprintf("%s%s (PID %d)", childPrefix, c.Name, c.PID))
	}

	return lines
}

func renderConnectionsTab(rep *model.Report, width int) []string {
	var lines []string
	if rep.Network == nil || len(rep.Network.Connections) == 0 {
		lines = append(lines, dim(" No active network connections for this process."))
		return lines
	}

	addrW := 17
	if width > 65 {
		addrW = (width - 20) / 2
	}

	lines = append(lines, fmt.Sprintf(" %s %s %s %s",
		padRight("PROTO", 6),
		padRight("LOCAL", addrW),
		padRight("REMOTE", addrW),
		"STATE",
	))

	for _, c := range rep.Network.Connections {
		local := fmt.Sprintf("%s:%d", c.LocalAddr, c.LocalPort)
		remote := "-"
		if c.RemotePort > 0 {
			remote = fmt.Sprintf("%s:%d", c.RemoteAddr, c.RemotePort)
		}
		lines = append(lines, fmt.Sprintf(" %s %s %s %s",
			padRight(string(c.Protocol), 6),
			padRight(truncateVisible(local, addrW), addrW),
			padRight(truncateVisible(remote, addrW), addrW),
			c.State,
		))
	}

	return lines
}

func renderFooter(m *Model, w int) string {
	if m.ViewMode == ModeFilter {
		prompt := fmt.Sprintf(" %s %s_", bold("Filter:"), m.Filter)
		return cyan(padRight(prompt, w))
	}

	if msg, isErr := m.ActiveStatus(); msg != "" {
		if isErr {
			return red(bold(fmt.Sprintf(" ✖ %s", msg)))
		}
		return green(bold(fmt.Sprintf(" ✔ %s", msg)))
	}

	shortcuts := fmt.Sprintf(" %s Nav  %s Filter  %s Kill  %s Force  %s Restart  %s Open  %s Copy  %s Tree  %s Help",
		bold("[↑/↓]"),
		bold("[/]"),
		bold("[k]"),
		bold("[f]"),
		bold("[r]"),
		bold("[o]"),
		bold("[c]"),
		bold("[t]"),
		bold("[?]"),
	)
	return truncateVisible(shortcuts, w)
}

func overlayHelp(lines []string, w, h int) {
	boxW := 62
	boxH := 16
	if w < boxW+2 || h < boxH+2 {
		return
	}

	content := []string{
		bold("                  PORTLENS TOP SHORTCUTS                   "),
		dim("──────────────────────────────────────────────────────────"),
		fmt.Sprintf("  %s / %s          Move cursor up / down", bold("↑"), bold("k")),
		fmt.Sprintf("  %s / %s          Move cursor down", bold("↓"), bold("j")),
		fmt.Sprintf("  %s / %s      Scroll one page up / down", bold("PgUp"), bold("PgDn")),
		fmt.Sprintf("  %s / %s          Jump to top / bottom", bold("Home/g"), bold("End/G")),
		fmt.Sprintf("  %s                Enter live search filter", bold("/")),
		fmt.Sprintf("  %s              Graceful terminate process (SIGTERM)", bold("k")),
		fmt.Sprintf("  %s              Force terminate process (SIGKILL)", bold("f")),
		fmt.Sprintf("  %s              Restart process (re-runs launch command)", bold("r")),
		fmt.Sprintf("  %s              Open in default browser (http://localhost)", bold("o")),
		fmt.Sprintf("  %s / %s          Copy PID / local URL to clipboard", bold("c"), bold("u")),
		fmt.Sprintf("  %s / %s / %s      Switch tab: Overview / Tree / Connections", bold("1"), bold("2/t"), bold("3/n")),
		fmt.Sprintf("  %s / %s      Clear filter / Close popup / Quit", bold("Esc"), bold("q")),
		dim("──────────────────────────────────────────────────────────"),
		cyan("                Press any key to close help                "),
	}

	drawBox(lines, content, boxW, boxH, w, h)
}

func overlayConfirm(lines []string, m *Model, w, h int) {
	boxW := 56
	boxH := 8
	if w < boxW+2 || h < boxH+2 {
		return
	}

	title := bold(red("                CONFIRM ACTION                "))
	prompt := truncateVisible(m.ConfirmPrompt, boxW-4)

	content := []string{
		title,
		dim("──────────────────────────────────────────────────────"),
		"",
		"  " + prompt,
		"",
		"            " + bold(green("[y] Confirm")) + "    " + bold(yellow("[n/Esc] Cancel")),
	}

	drawBox(lines, content, boxW, boxH, w, h)
}

func drawBox(lines []string, content []string, boxW, boxH, w, h int) {
	top := (h - boxH) / 2
	left := (w - boxW) / 2

	for i, row := range content {
		lineIdx := top + i
		if lineIdx >= 0 && lineIdx < len(lines) {
			orig := lines[lineIdx]
			contentRow := padRight(row, boxW)
			framed := inverse(contentRow)

			leftPad := strings.Repeat(" ", left)
			if visibleLen(orig) >= left {
				leftPad = truncateVisible(orig, left)
				leftPad = padRight(leftPad, left)
			}
			rightPad := ""
			rem := w - left - boxW
			if rem > 0 {
				rightPad = strings.Repeat(" ", rem)
			}
			lines[lineIdx] = leftPad + framed + rightPad
		}
	}
}
