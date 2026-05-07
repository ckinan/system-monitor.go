package ui

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

func filterProcs(procs []domain.Process, query string) []domain.Process {
	if query == "" {
		return procs
	}
	q := strings.ToLower(query)
	var out []domain.Process
	for _, p := range procs {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%d %d %s %s %s", p.Pid, p.Ppid, p.Username, p.Cmdline, util.HumanBytes(int64(p.Rss)))), q) {
			out = append(out, p)
		}
	}
	return out
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
	procs := filterProcs(m.procs, m.filter.Value())
	if !m.showKThreads {
		filtered := procs[:0]
		for _, p := range procs {
			if !p.IsKthread {
				filtered = append(filtered, p)
			}
		}
		procs = filtered
	}
	switch m.sortBy {
	case SortByRSS:
		sorted = util.SortBy(procs, func(p domain.Process) int { return p.Rss }, m.sortDesc)
	case SortByCPU:
		sorted = util.SortBy(procs, func(p domain.Process) float64 { return p.CPU }, m.sortDesc)
	case SortByPID:
		sorted = util.SortBy(procs, func(p domain.Process) int { return p.Pid }, m.sortDesc)
	case SortByPPID:
		sorted = util.SortBy(procs, func(p domain.Process) int { return p.Ppid }, m.sortDesc)
	case SortByCmdLine:
		sorted = util.SortBy(procs, func(p domain.Process) string { return p.Cmdline }, m.sortDesc)
	}

	rows := make([]table.Row, len(sorted))
	for i, p := range sorted {
		rows[i] = table.Row{
			fmt.Sprintf("%d", p.Pid),
			fmt.Sprintf("%d", p.Ppid),
			p.Username,
			fmt.Sprintf("%.2f%%", p.CPU),
			util.HumanBytes(int64(p.Rss)),
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
	seen := make(map[int]bool, len(procs))
	for _, p := range procs {
		if seen[p.Pid] {
			continue // gopsutil can return duplicate PIDs on /proc race; skip
		}
		seen[p.Pid] = true
		childrenByPid[p.Ppid] = append(childrenByPid[p.Ppid], p.Pid)
	}
	return childrenByPid
}

// appendTreeRows recursively builds table rows for the descendants of pid.
// prefix carries the vertical-bar context from parent levels so connectors line up correctly.
func appendTreeRows(pid int, pByPid map[int]domain.Process, childrenByPid map[int][]int, prefix string, rows []table.Row, pids []int) ([]table.Row, []int) {
	children := childrenByPid[pid]
	for i, childPid := range children {
		isLast := i == len(children)-1
		connector, nextPrefix := "├─ ", prefix+"│  "
		if isLast {
			connector, nextPrefix = "└─ ", prefix+"   "
		}
		p := pByPid[childPid]
		rows = append(rows, table.Row{fmt.Sprintf("%s%s[pid:%d | cpu:%.2f%% | rss:%s] %s", prefix, connector, p.Pid, p.CPU, util.HumanBytes(int64(p.Rss)), p.Cmdline)})
		pids = append(pids, p.Pid)
		rows, pids = appendTreeRows(childPid, pByPid, childrenByPid, nextPrefix, rows, pids)
	}
	return rows, pids
}

func buildTreeRows(procs []domain.Process, selected domain.Process) ([]table.Row, []int) {
	pByPid := make(map[int]domain.Process, len(procs))
	for _, p := range procs {
		pByPid[p.Pid] = p
	}
	childrenByPid := buildChildren(procs)
	parents := buildParents(procs, selected)

	var rows []table.Row
	var pids []int

	// ancestors: root → immediate parent — always a single chain so └─ is always correct
	for depth, i := 0, len(parents)-1; i >= 0; i, depth = i-1, depth+1 {
		p := parents[i]
		if depth == 0 {
			rows = append(rows, table.Row{fmt.Sprintf("[pid:%d | cpu:%.2f%% | rss:%s] %s", p.Pid, p.CPU, util.HumanBytes(int64(p.Rss)), p.Cmdline)})
		} else {
			rows = append(rows, table.Row{fmt.Sprintf("%s└─ [pid:%d | cpu:%.2f%% | rss:%s] %s", strings.Repeat("   ", depth-1), p.Pid, p.CPU, util.HumanBytes(int64(p.Rss)), p.Cmdline)})
		}
		pids = append(pids, p.Pid)
	}

	// selected process
	depth := len(parents)
	if depth == 0 {
		rows = append(rows, table.Row{fmt.Sprintf("[pid:%d | cpu:%.2f%% | rss:%s] %s", selected.Pid, selected.CPU, util.HumanBytes(int64(selected.Rss)), selected.Cmdline)})
	} else {
		rows = append(rows, table.Row{fmt.Sprintf("%s└─ [pid:%d | cpu:%.2f%% | rss:%s] %s", strings.Repeat("   ", depth-1), selected.Pid, selected.CPU, util.HumanBytes(int64(selected.Rss)), selected.Cmdline)})
	}
	pids = append(pids, selected.Pid)

	// children subtree — prefix is the vertical context below the selected node
	childPrefix := strings.Repeat("   ", depth)
	rows, pids = appendTreeRows(selected.Pid, pByPid, childrenByPid, childPrefix, rows, pids)

	return rows, pids
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
		m.killMsg = ""
		if m.killPending {
			switch msg.String() {
			case "t":
				if err := syscall.Kill(m.killPID, syscall.SIGTERM); err != nil {
					m.killMsg = fmt.Sprintf("SIGTERM failed: %s", err)
				} else {
					m.killMsg = fmt.Sprintf("sent SIGTERM to PID %d", m.killPID)
				}
				m.killPending = false
			case "k":
				if err := syscall.Kill(m.killPID, syscall.SIGKILL); err != nil {
					m.killMsg = fmt.Sprintf("SIGKILL failed: %s", err)
				} else {
					m.killMsg = fmt.Sprintf("sent SIGKILL to PID %d", m.killPID)
				}
				m.killPending = false
			case "esc":
				m.killPending = false
			}
			return m, nil
		}
		if m.filterActive {
			switch msg.String() {
			case "enter":
				m.filterActive = false
				m.filter.Blur()
				if m.showDetail {
					m.openDetailView()
				} else {
					m.applySort()
				}
				return m, nil
			case "esc":
				m.filterActive = false
				m.filter.Blur()
				m.filter.SetValue("")
				if m.showDetail {
					m.openDetailView()
				} else {
					m.applySort()
				}
				return m, nil
			}
			var tiCmd tea.Cmd
			m.filter, tiCmd = m.filter.Update(msg)
			if m.showDetail {
				m.openDetailView()
			} else {
				m.applySort()
			}
			return m, tiCmd
		}
		if m.showDetail {
			switch msg.String() {
			case "/":
				m.filterActive = true
				m.filter.SetValue("")
				m.filter.Focus()
				return m, textinput.Blink
			case "esc":
				if m.filter.Value() != "" {
					m.filter.SetValue("")
					m.openDetailView()
				}
				return m, nil
			case "f9":
				cursor := m.tableDetail.Cursor()
				if cursor >= 0 && cursor < len(m.treeRowPIDs) {
					m.killPID = m.treeRowPIDs[cursor]
					m.killPending = true
				}
				return m, nil
			case "H":
				m.showKThreads = !m.showKThreads
				return m, nil
			case "q":
				m.showDetail = false
				m.filter.SetValue("")
				return m, nil
			case "enter":
				cursor := m.tableDetail.Cursor()
				if cursor >= 0 && cursor < len(m.treeRowPIDs) {
					pid := m.treeRowPIDs[cursor]
					for _, p := range m.frozenProcs {
						if p.Pid == pid {
							m.frozenProc = p
							break
						}
					}
					m.filter.SetValue("")
					m.openDetailView()
				}
				return m, nil
			}
			m.tableDetail, cmd = m.tableDetail.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "/":
			m.filterActive = true
			m.filter.SetValue("")
			m.filter.Focus()
			return m, textinput.Blink
		case "esc":
			if m.filter.Value() != "" {
				m.filter.SetValue("")
				m.applySort()
			}
			return m, nil
		case "f9":
			row := m.table.SelectedRow()
			if len(row) > 0 {
				pid, _ := strconv.Atoi(row[0])
				m.killPID = pid
				m.killPending = true
			}
			return m, nil
		case "H":
			m.showKThreads = !m.showKThreads
			m.applySort()
			return m, nil
		}
		prev := m.sortBy
		isSortKey := true
		switch msg.String() {
		case "enter":
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
			m.filter.SetValue("")
			m.openDetailView()
			m.showDetail = true
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
		m.tableDetail.SetHeight(m.height - 4)
		m.applySort()

		return m, nil
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *Model) openDetailView() {
	rows, pids := buildTreeRows(m.frozenProcs, m.frozenProc)

	if q := m.filter.Value(); q != "" {
		q = strings.ToLower(q)
		var filteredRows []table.Row
		var filteredPIDs []int
		for i, r := range rows {
			if strings.Contains(strings.ToLower(r[0]), q) {
				filteredRows = append(filteredRows, r)
				filteredPIDs = append(filteredPIDs, pids[i])
			}
		}
		rows, pids = filteredRows, filteredPIDs
	}

	m.tableDetail.SetRows(rows)
	m.treeRowPIDs = pids
	for i, pid := range pids {
		if pid == m.frozenProc.Pid {
			m.tableDetail.SetCursor(i)
			return
		}
	}
	m.tableDetail.SetCursor(0)
}
