package internal

import (
	"fmt"
	"strings"
)

func HumanBytes(b int) string {
	switch {
	case b >= 1<<30: // >= 1GiB
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1<<30))
	case b >= 20: // >= 1 MiB
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(1<<20))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func extractFieldFromLine(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", fmt.Errorf("invalid line, expected at least 2 fields, got %v, line: %s", len(fields), line)
	}
	return fields[1], nil
}
