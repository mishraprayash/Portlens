package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

// ViewMode defines the active UI mode.
type ViewMode int

const (
	ModeNormal ViewMode = iota
	ModeFilter
	ModeConfirm
	ModeHelp
)

// DetailTab specifies which tab is displayed in the right-hand detail pane.
type DetailTab int

const (
	TabOverview DetailTab = iota
	TabTree
	TabConnections
)

// ConfirmAction indicates the pending destructive action.
type ConfirmAction int

const (
	ConfirmNone ConfirmAction = iota
	ConfirmKill
	ConfirmForceKill
	ConfirmRestart
)

// Model holds the complete state of the TUI application.
type Model struct {
	Entries         []model.PortEntry
	FilteredIndices []int
	SelectedIdx     int
	ScrollOffset    int
	Filter          string

	ViewMode      ViewMode
	DetailTab     DetailTab
	ConfirmAction ConfirmAction
	ConfirmPrompt string
	ConfirmTarget *model.PortEntry

	SelectedReport *model.Report
	ReportCache    map[int32]*model.Report
	LoadingReport  bool

	StatusMessage string
	StatusIsError bool
	StatusExpires time.Time

	Width  int
	Height int

	TCPOnly     bool
	LastUpdated time.Time
}

// NewModel initializes a fresh Model.
func NewModel(tcpOnly bool) *Model {
	return &Model{
		TCPOnly:     tcpOnly,
		ReportCache: make(map[int32]*model.Report),
		Width:       80,
		Height:      24,
		LastUpdated: time.Now(),
	}
}

// SetEntries updates the list of port entries, preserving the current selection if possible.
func (m *Model) SetEntries(entries []model.PortEntry) {
	var prevPort int32 = -1
	var prevProto model.Protocol
	if sel := m.SelectedEntry(); sel != nil {
		prevPort = sel.Port
		prevProto = sel.Protocol
	}

	m.Entries = entries
	m.LastUpdated = time.Now()
	m.RecomputeFilter()

	// Try to restore previous selection
	restored := false
	if prevPort != -1 {
		for i, idx := range m.FilteredIndices {
			e := m.Entries[idx]
			if e.Port == prevPort && e.Protocol == prevProto {
				m.SelectedIdx = i
				restored = true
				break
			}
		}
	}

	if !restored {
		if len(m.FilteredIndices) == 0 {
			m.SelectedIdx = 0
			m.ScrollOffset = 0
			m.SelectedReport = nil
		} else if m.SelectedIdx >= len(m.FilteredIndices) {
			m.SelectedIdx = len(m.FilteredIndices) - 1
		}
	}
}

// RecomputeFilter rebuilds FilteredIndices matching the current Filter text.
func (m *Model) RecomputeFilter() {
	m.FilteredIndices = m.FilteredIndices[:0]
	q := strings.TrimSpace(strings.ToLower(m.Filter))

	for i, e := range m.Entries {
		if m.TCPOnly && e.Protocol.Normalize() != model.ProtocolTCP {
			continue
		}
		if q == "" {
			m.FilteredIndices = append(m.FilteredIndices, i)
			continue
		}
		// Match against port, process, service, project, runtime, container, address
		if strings.Contains(strconv.Itoa(int(e.Port)), q) ||
			strings.Contains(strings.ToLower(e.Process), q) ||
			strings.Contains(strings.ToLower(e.Service), q) ||
			strings.Contains(strings.ToLower(e.Project), q) ||
			strings.Contains(strings.ToLower(e.Runtime), q) ||
			strings.Contains(strings.ToLower(e.Address), q) ||
			strings.Contains(strings.ToLower(string(e.Protocol)), q) ||
			(e.Container != nil && (strings.Contains(strings.ToLower(e.Container.Name), q) ||
				strings.Contains(strings.ToLower(e.Container.Image), q))) {
			m.FilteredIndices = append(m.FilteredIndices, i)
		}
	}

	if len(m.FilteredIndices) == 0 {
		m.SelectedIdx = 0
		m.ScrollOffset = 0
	} else if m.SelectedIdx >= len(m.FilteredIndices) {
		m.SelectedIdx = len(m.FilteredIndices) - 1
	}
}

// SetFilter updates the filter query and re-filters entries.
func (m *Model) SetFilter(q string) {
	m.Filter = q
	m.RecomputeFilter()
}

// SelectedEntry returns the currently selected PortEntry, or nil if none.
func (m *Model) SelectedEntry() *model.PortEntry {
	if len(m.FilteredIndices) == 0 || m.SelectedIdx < 0 || m.SelectedIdx >= len(m.FilteredIndices) {
		return nil
	}
	return &m.Entries[m.FilteredIndices[m.SelectedIdx]]
}

// MoveUp shifts the selection cursor up by one.
func (m *Model) MoveUp() {
	if m.SelectedIdx > 0 {
		m.SelectedIdx--
	}
}

// MoveDown shifts the selection cursor down by one.
func (m *Model) MoveDown() {
	if m.SelectedIdx < len(m.FilteredIndices)-1 {
		m.SelectedIdx++
	}
}

// PageUp moves the selection cursor up by pageSize.
func (m *Model) PageUp(pageSize int) {
	if pageSize <= 0 {
		pageSize = 10
	}
	m.SelectedIdx -= pageSize
	if m.SelectedIdx < 0 {
		m.SelectedIdx = 0
	}
}

// PageDown moves the selection cursor down by pageSize.
func (m *Model) PageDown(pageSize int) {
	if pageSize <= 0 {
		pageSize = 10
	}
	m.SelectedIdx += pageSize
	if m.SelectedIdx >= len(m.FilteredIndices) {
		m.SelectedIdx = len(m.FilteredIndices) - 1
	}
	if m.SelectedIdx < 0 {
		m.SelectedIdx = 0
	}
}

// Home moves selection to the first item.
func (m *Model) Home() {
	m.SelectedIdx = 0
}

// End moves selection to the last item.
func (m *Model) End() {
	if len(m.FilteredIndices) > 0 {
		m.SelectedIdx = len(m.FilteredIndices) - 1
	}
}

// EnsureVisible adjusts ScrollOffset so SelectedIdx is within the visible window.
func (m *Model) EnsureVisible(visibleRows int) {
	if visibleRows <= 0 {
		return
	}
	if m.SelectedIdx < m.ScrollOffset {
		m.ScrollOffset = m.SelectedIdx
	} else if m.SelectedIdx >= m.ScrollOffset+visibleRows {
		m.ScrollOffset = m.SelectedIdx - visibleRows + 1
	}
	if m.ScrollOffset < 0 {
		m.ScrollOffset = 0
	}
}

// SetStatus displays a transient status message for the specified duration.
func (m *Model) SetStatus(msg string, isError bool, duration time.Duration) {
	m.StatusMessage = msg
	m.StatusIsError = isError
	m.StatusExpires = time.Now().Add(duration)
}

// ActiveStatus returns the active status message if not expired.
func (m *Model) ActiveStatus() (string, bool) {
	if m.StatusMessage == "" {
		return "", false
	}
	if time.Now().After(m.StatusExpires) {
		m.StatusMessage = ""
		return "", false
	}
	return m.StatusMessage, m.StatusIsError
}
