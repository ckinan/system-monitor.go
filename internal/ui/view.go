package ui

import (
	"fmt"

	"github.com/ckinan/sysmon/internal"
)

func (m Model) View() string {
	header := fmt.Sprintf("Mem: %s / %s\n", internal.HumanBytes(m.ram.MemUsed), internal.HumanBytes(m.ram.MemTotal))
	footer := "[q] quit"
	return header + "\n" + m.table.View() + "\n\n" + footer
}
