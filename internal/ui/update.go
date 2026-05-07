package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ckinan/cktop/internal/domain"
	"github.com/ckinan/cktop/internal/util"
)

type snapshotMsg domain.Snapshot

func waitForSnapshot(ch <-chan domain.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-ch
		if !ok {
			return nil
		}
		return snapshotMsg(snap)
	}
}

func calcDir(showDir bool, sortDesc bool) string {
	if !showDir {
		return ""
	}
	if sortDesc == true {
		return " ▼"
	}
	return " ▲"
}

func (m *Model) applySort() {
	// Reserve lines for CPU header (1) + RAM header (1) + blank (1) + [table content] + blank (1) + footer (1) = 5
	// bubbles/table renders its own column header row internally
	cmdW := max(20, m.width-colPIDWidth-colPPIDWidth-colUserWidth-colCPUWidth-colRSSWidth)

	m.table.SetColumns([]table.Column{
		{Title: "PID" + calcDir(m.sortBy == SortByPID, m.sortDesc), Width: colPIDWidth},
		{Title: "PPID", Width: colPPIDWidth},
		{Title: "User", Width: colUserWidth},
		{Title: "CPU%" + calcDir(m.sortBy == SortByCPU, m.sortDesc), Width: colCPUWidth},
		{Title: "RSS" + calcDir(m.sortBy == SortByRSS, m.sortDesc), Width: colRSSWidth},
		{Title: "CmdLine" + calcDir(m.sortBy == SortByCmdLine, m.sortDesc), Width: cmdW},
	})

	var sorted []domain.Process
	switch m.sortBy {
	case SortByRSS:
		sorted = util.SortBy(m.procs, func(p domain.Process) int { return p.Rss }, m.sortDesc)
	case SortByCPU:
		sorted = util.SortBy(m.procs, func(p domain.Process) float64 { return p.CPU }, m.sortDesc)
	case SortByPID:
		sorted = util.SortBy(m.procs, func(p domain.Process) int { return p.Pid }, m.sortDesc)
	case SortByPPID:
		sorted = util.SortBy(m.procs, func(p domain.Process) int { return p.Ppid }, m.sortDesc)
	case SortByCmdLine:
		sorted = util.SortBy(m.procs, func(p domain.Process) string { return p.Cmdline }, m.sortDesc)
	}

	rows := make([]table.Row, len(sorted))
	for i, p := range sorted {
		rows[i] = table.Row{
			fmt.Sprintf("%d", p.Pid),
			fmt.Sprintf("%d", p.Ppid),
			p.Username,
			fmt.Sprintf("%.2f%%", p.CPU),
			util.HumanBytes(p.Rss),
			p.Cmdline,
		}
	}
	m.table.SetRows(rows)
}

func buildParents(procs []domain.Process, selected domain.Process) []domain.Process {
	pByPid := make(map[int]domain.Process, len(procs))
	for _, p := range procs {
		pByPid[p.Pid] = p
	}

	var chain []domain.Process
	currentPPID := selected.Ppid
	for currentPPID != 0 {
		p, ok := pByPid[currentPPID]
		if !ok {
			break
		}
		chain = append(chain, p)
		currentPPID = p.Ppid
	}
	return chain
}

func buildChildren(procs []domain.Process) map[int][]int {
	childrenByPid := make(map[int][]int)
	for _, p := range procs {
		childrenByPid[p.Ppid] = append(childrenByPid[p.Ppid], p.Pid)
	}
	return childrenByPid
}

func renderSubtree(selectedPid int, pByPid map[int]domain.Process, childrenByPid map[int][]int, depth int, treeview string) string {
	depth++
	for _, childrenPid := range childrenByPid[selectedPid] {
		indent := strings.Repeat("  ", depth)
		treeview = fmt.Sprintf(
			"%s%s|- [pid:%d | cpu:%.2f%% | rss: %s] %s\n",
			treeview,
			indent,
			pByPid[childrenPid].Pid,
			pByPid[childrenPid].CPU,
			util.HumanBytes(pByPid[childrenPid].Rss),
			pByPid[childrenPid].Cmdline,
		)
		treeview = renderSubtree(pByPid[childrenPid].Pid, pByPid, childrenByPid, depth, treeview)
	}
	return treeview
}

func (m *Model) treeview() string {
	var treeview string
	var parents []domain.Process

	parents = buildParents(m.frozenProcs, m.frozenProc)

	depth := 0
	for i := len(parents) - 1; i >= 0; i-- {
		indent := strings.Repeat("  ", depth)
		treeview = fmt.Sprintf(
			"%s%s|- [%d] %s\n",
			treeview,
			indent,
			parents[i].Pid,
			parents[i].Cmdline,
		)
		depth++
	}

	indent := strings.Repeat("  ", depth)
	treeview = fmt.Sprintf(
		"%s%s%s\n",
		treeview,
		indent,
		lipgloss.NewStyle().Reverse(true).Render(fmt.Sprintf("|-[%d] %s", m.frozenProc.Pid, m.frozenProc.Cmdline)),
	)

	pByPid := make(map[int]domain.Process, len(m.frozenProcs))
	for _, p := range m.frozenProcs {
		pByPid[p.Pid] = p
	}

	childrenByPid := buildChildren(m.frozenProcs)
	treeview = renderSubtree(m.frozenProc.Pid, pByPid, childrenByPid, depth, treeview)
	return treeview
}

func (m *Model) details() string {
	return fmt.Sprintf(
		"PID: %d\nPPID: %d\nUser: %s\nCPU%%: %.2f%%\nRSS: %s\nCmdLine: %s\n\n%s",
		m.frozenProc.Pid,
		m.frozenProc.Ppid,
		m.frozenProc.Username,
		m.frozenProc.CPU,
		util.HumanBytes(m.frozenProc.Rss),
		m.frozenProc.Cmdline,
		m.treeview(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case snapshotMsg:
		snap := domain.Snapshot(msg)
		m.CPU = msg.CPU
		m.memory = msg.Memory
		wasEmpty := len(m.procs) == 0 // first data arrival?
		m.procs = snap.Processes
		m.applySort()
		if wasEmpty {
			m.table.GotoTop()
		}
		return m, waitForSnapshot(m.snapCh)
	case tea.KeyMsg:
		if m.showDetail {
			if msg.String() == "q" {
				m.showDetail = false
				return m, nil
			}
			// pass keys to viewport for scrolling
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		prev := m.sortBy
		isSortKey := true
		switch msg.String() {
		case "enter":
			m.showDetail = true
			isSortKey = false
			frozenProcs := make([]domain.Process, len(m.procs))
			copy(frozenProcs, m.procs)
			m.frozenProcs = frozenProcs

			selectedPID := m.table.SelectedRow()[0]
			selectedPIDint, _ := strconv.Atoi(selectedPID)
			for _, p := range m.frozenProcs {
				if p.Pid == selectedPIDint {
					m.frozenProc = p
					break
				}
			}
			m.viewport.SetContent(m.details())
		case "M":
			m.sortBy = SortByRSS
		case "C":
			m.sortBy = SortByCPU
		case "P":
			m.sortBy = SortByPID
		case "L":
			m.sortBy = SortByCmdLine
		case "q":
			return m, tea.Quit
		default:
			isSortKey = false
		}
		if isSortKey {
			if m.sortBy == prev {
				// same key: toggle direction
				m.sortDesc = !m.sortDesc
			} else {
				// new field: reset to descending
				m.sortDesc = true
			}
			m.applySort()
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.table.SetHeight(m.height - 5)
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 3
		m.applySort()

		return m, nil
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}
