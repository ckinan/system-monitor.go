package ui

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ckinan/sysmon/internal"
	"github.com/ckinan/sysmon/internal/collector"
)

const (
	colPIDWidth     = 8
	colPPIDWidth    = 8
	colUserWidth    = 10
	colNameWidth    = 15
	colRSSWidth     = 10
	colCommandWidth = 40
)

// Model is the bubbletea model. It holds all UI state
type Model struct {
	snapCh <-chan collector.Snapshot // read-only channel from the collect
	ram    internal.Ram
	procs  []internal.Process
	height int // terminal height
	width  int
	table  table.Model
}

func New(ch <-chan collector.Snapshot) Model {
	// height: 24 is a safe fallback
	// frame is painted right after startup, so this default is almost never actually visible
	cols := []table.Column{
		{Title: "PID", Width: colPIDWidth},
		{Title: "PPID", Width: colNameWidth},
		{Title: "User", Width: colUserWidth},
		{Title: "Name", Width: colNameWidth},
		{Title: "RSS", Width: colRSSWidth},
		{Title: "Command", Width: colCommandWidth},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true), // focused = keyboard nav (↑/↓) is active
	)
	return Model{snapCh: ch, height: 24, table: t}
}

func (m Model) Init() tea.Cmd {
	return waitForSnapshot(m.snapCh)
}
