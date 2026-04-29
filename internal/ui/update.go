package ui

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ckinan/sysmon/internal"
	"github.com/ckinan/sysmon/internal/collector"
)

type snapshotMsg collector.Snapshot

func waitForSnapshot(ch <-chan collector.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-ch
		if !ok {
			return nil
		}
		return snapshotMsg(snap)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotMsg:
		snap := collector.Snapshot(msg)
		m.ram = msg.Ram
		m.procs = msg.Processes

		// Sort before converting from int to string
		sorted := slices.Clone(snap.Processes)
		slices.SortFunc(sorted, func(a, b internal.Process) int {
			return cmp.Compare(b.Rss, a.Rss)
		})

		// Convert []Process -> []table.Row([]string per row)
		rows := make([]table.Row, len(sorted))

		for i, p := range sorted {
			rows[i] = table.Row{
				fmt.Sprintf("%d", p.Pid),
				fmt.Sprintf("%d", p.Ppid),
				p.Username,
				p.Name,
				internal.HumanBytes(p.Rss),
				p.Cmdline,
			}
		}
		m.table.SetRows(rows)

		return m, waitForSnapshot(m.snapCh)
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		// Reserve lines for RAM header (1) + blank (1) + [table content] + blank (1) + footer (1) = 4
		// bubbles/table renders its own column header row internally
		m.table.SetHeight(m.height - 4)
		cmdW := m.width - colPIDWidth - colPPIDWidth - colUserWidth - colNameWidth - colRSSWidth

		if cmdW < 20 {
			cmdW = 20
		}

		m.table.SetColumns([]table.Column{
			{Title: "PID", Width: colPIDWidth},
			{Title: "PPID", Width: colPPIDWidth},
			{Title: "User", Width: colUserWidth},
			{Title: "Name", Width: colNameWidth},
			{Title: "RSS", Width: colRSSWidth},
			{Title: "Command", Width: cmdW},
		})

		return m, nil
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}
