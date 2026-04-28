package internal

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Process struct {
	Pid     int // process id
	Ppid    int // parent process id
	Name    string
	State   string // process state (R=running, S=sleeping, Z=zombie, etc)
	Threads int
	RssKB   int // actual RAM used (in kB)
}

func readProcess(pid int) (Process, error) {
	file, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return Process{}, err
	}
	defer file.Close()

	process := Process{Pid: pid}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		var fieldErr error

		if strings.HasPrefix(line, "PPid:") {
			var s string
			s, fieldErr = extractFieldFromLine(line)
			if fieldErr == nil {
				process.Ppid, fieldErr = strconv.Atoi(s)
			}
		} else if strings.HasPrefix(line, "Name:") {
			process.Name, fieldErr = extractFieldFromLine(line)
		} else if strings.HasSuffix(line, "State:") {
			process.State, fieldErr = extractFieldFromLine(line)
		} else if strings.HasPrefix(line, "Threads:") {
			var s string
			s, fieldErr = extractFieldFromLine(line)
			if fieldErr == nil {
				process.Threads, fieldErr = strconv.Atoi(s)
			}
		} else if strings.HasPrefix(line, "VmRSS:") {
			var s string
			s, fieldErr = extractFieldFromLine(line)
			if fieldErr == nil {
				process.RssKB, fieldErr = strconv.Atoi(s)
			}
		}
		if fieldErr != nil {
			return Process{}, fieldErr
		}
	}
	return process, nil
}

func ListProcess() ([]Process, error) {
	procDirs, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("listing /proc: %w", err)
	}

	results := make(chan Process, len(procDirs))
	var wg sync.WaitGroup

	for _, entry := range procDirs {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue // skip non-PID entries like "tty", "net", etc.
		}
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			if p, err := readProcess(pid); err == nil {
				results <- p
				// errors are silently skipped: a PID may dissapear
				// ReadDir and Open (process exited): that's normal, not fatal
			}
		}(pid)

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
