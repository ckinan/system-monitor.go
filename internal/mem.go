package internal

import (
	"github.com/shirou/gopsutil/v4/mem"
)

type Ram struct {
	MemTotal     int
	MemAvailable int
	MemUsed      int
}

func GetRam() (Ram, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return Ram{}, err
	}
	return Ram{
		MemTotal:     int(v.Total),
		MemAvailable: int(v.Available),
		MemUsed:      int(v.Used),
	}, nil
}
