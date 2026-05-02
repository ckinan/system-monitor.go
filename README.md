# cktop

System Monitor written in Go.

This CLI tool is supposed to work on any OS supported by `gopsutil` e.g. Linux, MacOS, Windows. For supported platforms, read: https://github.com/shirou/gopsutil#current-status

## ROADMAP

1. Show live CPU and RAM used by the system.
2. Sort by process name, memory, cpu
3. Process details view: expand a process to see:
- Full process tree (parent -> children hierarchy)
- Environment variables
- Start time and running duration
- Note: need to explore what can be extracted and presented
4. App-grouped view: group related processes under their root app. Something like treeview, but instead of showing all the processes, it will just show the most parent application e.g. app:Firefox without showing all its subprocesses

## Installation

```sh
go install github.com/ckinan/cktop@latest
```

## Upgrade

```sh
GOPROXY=direct go install github.com/ckinan/cktop@latest
```

