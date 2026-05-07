package gopsutil

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/ckinan/cktop/internal/domain"
	"github.com/shirou/gopsutil/v4/process"
)

type GopsutilProcessReader struct {
	cache map[int32]*process.Process
}

func NewGopsutilProcessReader() *GopsutilProcessReader {
	return &GopsutilProcessReader{
		cache: make(map[int32]*process.Process),
	}
}

func (g *GopsutilProcessReader) ReadProcesses() ([]domain.Process, error) {
	fresh, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}

	livePIDs := make(map[int32]bool, len(fresh))
	for _, p := range fresh {
		livePIDs[p.Pid] = true
		if _, ok := g.cache[p.Pid]; !ok {
			g.cache[p.Pid] = p
		}
	}

	for pid := range g.cache {
		if !livePIDs[pid] {
			delete(g.cache, pid)
		}
	}

	results := make([]domain.Process, 0, len(g.cache))
	for _, p := range g.cache {
		proc, err := readOne(p)
		if err != nil {
			log.Printf("dropped pid=%d err=%v", p.Pid, err)
			continue
		}
		results = append(results, proc)
	}
	return results, nil
}

func readOne(p *process.Process) (domain.Process, error) {
	ppid, err := p.Ppid()
	if err != nil {
		return domain.Process{}, err
	}
	mem, err := p.MemoryInfo()
	if err != nil {
		return domain.Process{}, err
	}
	// cmdline will error on kernel threads (they do not have cmdline)
	// so let's not evaluate the errors for them
	cmdline, _ := p.Cmdline()
	// do not show the full path, only the executable and the args
	if cmdline != "" {
		parts := strings.SplitN(cmdline, " ", 2)
		parts[0] = filepath.Base(parts[0])
		cmdline = strings.Join(parts, " ")
	}
	username, err := p.Username()
	if err != nil {
		uid, _ := p.Uids()
		if len(uid) > 0 {
			username = fmt.Sprintf("%d", uid[0])
		} else {
			username = "?"
		}
		// return domain.Process{}, err
		// let's fallback to user id, otherwise it would error with: something like:
		// "2026/05/07 14:04:56 dropped pid=4172 err=user: unknown userid 70"
	}
	cpu, err := p.Percent(0)
	if err != nil {
		cpu = 0 // not fatal
	}
	return domain.Process{
		Pid:      int(p.Pid),
		Ppid:     int(ppid),
		Rss:      int(mem.RSS),
		CPU:      cpu,
		Cmdline:  cmdline,
		Username: username,
	}, nil
}
