package internal

import (
	"fmt"
	"sync"

	"github.com/shirou/gopsutil/v4/process"
)

type Process struct {
	Pid      int // process id
	Ppid     int // parent process id
	Name     string
	Rss      int // bytes
	Cmdline  string
	Username string
}

func readProcess(p *process.Process) (Process, error) {
	name, err := p.Name()
	if err != nil {
		return Process{}, err
	}
	ppid, err := p.Ppid()
	if err != nil {
		return Process{}, err
	}
	mem, err := p.MemoryInfo()
	if err != nil {
		return Process{}, err
	}
	// cmdline will error on kernel threads (they do not have cmdline)
	// so let's not evaluate the errors for them
	cmdline, _ := p.Cmdline()
	username, err := p.Username()
	if err != nil {
		return Process{}, err
	}
	return Process{
		Pid:      int(p.Pid),
		Ppid:     int(ppid),
		Name:     name,
		Rss:      int(mem.RSS),
		Cmdline:  cmdline,
		Username: username,
	}, nil
}

func ListProcess() ([]Process, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("listing /proc: %w", err)
	}

	results := make(chan Process, len(procs))
	var wg sync.WaitGroup

	for _, p := range procs {
		wg.Add(1)
		go func(p *process.Process) {
			defer wg.Done()
			if proc, err := readProcess(p); err == nil {
				results <- proc
			}
		}(p)

	}

	// wait for all goroutines to finish, then close so range belo terminates
	wg.Wait()
	close(results)

	var processes []Process
	for p := range results {
		processes = append(processes, p)
	}
	return processes, nil
}
