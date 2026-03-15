package environment

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// parseCPU parses a CPU limit string and returns the equivalent NanoCPUs value
// for Docker's HostConfig.Resources.NanoCPUs.
//
// Accepted formats:
//   - ""  or "0" → 0 (no limit)
//   - "2"        → 2 CPUs (absolute)
//   - "0.5"      → half a CPU (absolute float)
//   - "50%"      → 50% of runtime.NumCPU()
func parseCPU(value string) (int64, error) {
	if value == "" || value == "0" {
		return 0, nil
	}

	if strings.HasSuffix(value, "%") {
		pct, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid CPU percentage %q: %w", value, err)
		}
		if pct < 0 || pct > 100 {
			return 0, fmt.Errorf("CPU percentage must be between 0 and 100, got %q", value)
		}
		cpus := float64(runtime.NumCPU()) * pct / 100.0
		return int64(cpus * 1e9), nil
	}

	cpus, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value %q: %w", value, err)
	}
	if cpus < 0 {
		return 0, fmt.Errorf("CPU value must be non-negative, got %q", value)
	}
	return int64(cpus * 1e9), nil
}
