package ui

import (
	"fmt"

	"github.com/ckinan/cktop/internal/util"
)

func (m Model) View() string {
	if m.showDetail {
		header := fmt.Sprintf(
			"Snapshot for PID: %d",
			m.frozenProc.Pid,
		)
		footer := "[q]quit"
		return header + "\n\n" + m.viewport.View() + "\n" + footer
	}
	header := fmt.Sprintf(
		"CPU: %.2f%%\nMem: %s / %s (%.2f%%)\n",
		m.CPU,
		util.HumanBytes(m.memory.Used),
		util.HumanBytes(m.memory.Total),
		float64(m.memory.Used)*100.0/float64(m.memory.Total),
	)
	footer := "sort: [C]cpu [M]rss [P]pid [L]cmdline | [enter]details [q]quit"
	return header + "\n" + m.table.View() + "\n\n" + footer
}
