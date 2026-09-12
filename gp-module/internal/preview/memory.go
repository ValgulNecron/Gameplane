// Package preview evaluates runtime manifests and dynamic memory calculations.
package preview

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"
)

// CalculateAutoMemory parses a Kubernetes memory quantity string and computes
// the percentage-based memory in mebibytes with an "M" suffix, matching the
// Gameplane operator's exact arithmetic.
func CalculateAutoMemory(memoryLimitStr string, percent int) (string, error) {
	if percent < 1 || percent > 100 {
		return "", fmt.Errorf("percent must be between 1 and 100, got %d", percent)
	}

	q, err := resource.ParseQuantity(memoryLimitStr)
	if err != nil {
		return "", fmt.Errorf("invalid memory quantity %q: %w", memoryLimitStr, err)
	}

	mib := q.Value() * int64(percent) / 100 / (1 << 20)
	if mib <= 0 {
		return "", fmt.Errorf("calculated memory limit is 0 or negative")
	}

	return strconv.FormatInt(mib, 10) + "M", nil
}
