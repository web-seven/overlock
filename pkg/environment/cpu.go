package environment

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// parseCPU converts a CPU string to Docker NanoCPUs.
//
//   - ""  or "0"  → 0 (no limit)
//   - "50%"       → percentage of runtime.NumCPU(), expressed as NanoCPUs
//   - "2" / "0.5" → absolute core count expressed as NanoCPUs
func parseCPU(value string) (int64, error) {
	if value == "" || value == "0" {
		return 0, nil
	}

	if strings.HasSuffix(value, "%") {
		pctStr := strings.TrimSuffix(value, "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid CPU percentage %q: %w", value, err)
		}
		if pct <= 0 {
			return 0, nil
		}
		cpus := pct / 100.0 * float64(runtime.NumCPU())
		return int64(cpus * 1e9), nil
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU value %q: %w", value, err)
	}
	if f <= 0 {
		return 0, nil
	}
	return int64(f * 1e9), nil
}
