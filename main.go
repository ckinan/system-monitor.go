package main

import (
	"fmt"
	"log/slog"

	"github.com/ckinan/sm.go/internal"
)

func main() {
	ram, err := internal.GetRam()
	if err != nil {
		slog.Error("error getting ram data", "err", err)
	}
	fmt.Printf("used memory: %d, available memory: %d, total memory: %d\n", ram.MemUsed, ram.MemAvailable, ram.MemTotal)
}
